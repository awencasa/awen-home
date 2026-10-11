package generator

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"html/template"
	"io"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/renderer/html"
	"gopkg.in/yaml.v3"
)

// ArticleFrontmatter represents the YAML frontmatter in markdown files.
type ArticleFrontmatter struct {
	Title       string   `yaml:"title"`
	Slug        string   `yaml:"slug"`
	Date        string   `yaml:"date"`
	Author      string   `yaml:"author"`
	Summary     string   `yaml:"summary"`
	Tags        []string `yaml:"tags"`
	ReadingTime string   `yaml:"reading_time"`
	Featured    bool     `yaml:"featured"`
}

// CommentData represents a reader comment stored in dynamic data YAML.
type CommentData struct {
	ID        string    `yaml:"id" json:"id"`
	Author    string    `yaml:"author" json:"author"`
	Content   string    `yaml:"content" json:"content"`
	CreatedAt time.Time `yaml:"created_at" json:"created_at"`
}

// ArticleDynamicData represents dynamic claps and comments stored in data/<slug>.yaml.
type ArticleDynamicData struct {
	Slug     string        `yaml:"slug"`
	Claps    int           `yaml:"claps"`
	Comments []CommentData `yaml:"comments"`
}

// CommentView is passed to HTML templates.
type CommentView struct {
	ID             string
	Author         string
	AuthorInitials string
	Content        string
	CreatedAt      string
	DateFormatted  string
}

// Article represents a compiled article ready for templating and RSS.
type Article struct {
	Title          string
	Slug           string
	Date           string
	DateFormatted  string
	DateTime       time.Time
	Author         string
	AuthorInitials string
	Summary        string
	Tags           []string
	ReadingTime    string
	Featured       bool
	Claps          int
	Comments       []CommentView
	RawMarkdown    string
	HTMLContent    template.HTML
	PlainText      string
}

// SearchIndexEntry is serialized to search.json.
type SearchIndexEntry struct {
	Title         string   `json:"title"`
	Slug          string   `json:"slug"`
	Date          string   `json:"date"`
	DateFormatted string   `json:"date_formatted"`
	Summary       string   `json:"summary"`
	Tags          []string `json:"tags"`
	ReadingTime   string   `json:"reading_time"`
	Claps         int      `json:"claps"`
	Content       string   `json:"content"`
}

// PaginationLink represents a page link in the pagination nav.
type PaginationLink struct {
	PageNum   int
	URL       string
	IsCurrent bool
}

// Template data views
type PageData struct {
	Title           string
	Description     string
	ActiveNav       string
	Content         template.HTML
	ShowHero        bool
	Articles        []Article
	TotalArticles   int
	AllTags         []string
	TagArchiveName  string
	TotalPages      int
	CurrentPage     int
	PrevPageURL     string
	NextPageURL     string
	PaginationLinks []PaginationLink
	Article         *Article
	CanonicalURL    string
}

// Config holds options for the build generator.
type Config struct {
	BlogDir   string
	PublicDir string
	DataDir   string
	DistDir   string
	BaseURL   string
	PageSize  int
}

// DefaultConfig returns reasonable defaults for the Awen site.
func DefaultConfig() Config {
	return Config{
		BlogDir:   "blog",
		PublicDir: "public",
		DataDir:   "data/blog",
		DistDir:   "dist",
		BaseURL:   "http://localhost:8080",
		PageSize:  3, // 3 per page produces clean pagination with sample articles
	}
}

// Build generates the complete static site.
func Build(cfg Config) error {
	fmt.Println("==> Building Awen Static Site...")

	if err := os.MkdirAll(cfg.DistDir, 0755); err != nil {
		return fmt.Errorf("failed to create dist directory: %w", err)
	}

	// 1. Load articles
	articles, err := loadArticles(cfg)
	if err != nil {
		return fmt.Errorf("failed to load articles: %w", err)
	}
	fmt.Printf("==> Loaded %d articles from %s\n", len(articles), cfg.BlogDir)

	// 2. Load templates
	templates, err := parseTemplates(cfg.PublicDir)
	if err != nil {
		return fmt.Errorf("failed to parse templates: %w", err)
	}

	// 3. Extract unique tags
	tagSet := make(map[string]bool)
	for _, a := range articles {
		for _, t := range a.Tags {
			tagSet[t] = true
		}
	}
	var allTags []string
	for t := range tagSet {
		allTags = append(allTags, t)
	}
	sort.Strings(allTags)

	// 4. Generate Paginated Index Pages
	if err := generatePaginatedIndex(cfg, templates, articles, allTags); err != nil {
		return fmt.Errorf("failed to generate index pages: %w", err)
	}

	// 5. Generate Tag Archive Pages
	if err := generateTagArchives(cfg, templates, articles, allTags); err != nil {
		return fmt.Errorf("failed to generate tag archives: %w", err)
	}

	// 6. Generate Single Article Pages
	if err := generateArticlePages(cfg, templates, articles); err != nil {
		return fmt.Errorf("failed to generate article pages: %w", err)
	}

	// 7. Generate About Page
	if err := generateAboutPage(cfg, templates); err != nil {
		return fmt.Errorf("failed to generate about page: %w", err)
	}

	// 8. Generate search.json
	if err := generateSearchIndex(cfg, articles); err != nil {
		return fmt.Errorf("failed to generate search index: %w", err)
	}

	// 9. Generate RSS Feeds (feed.xml and rss.xml)
	if err := generateRSSFeed(cfg, articles); err != nil {
		return fmt.Errorf("failed to generate RSS feed: %w", err)
	}

	// 10. Copy static assets from public/ (CSS, JS, favicon, etc.)
	if err := copyStaticAssets(cfg.PublicDir, cfg.DistDir); err != nil {
		return fmt.Errorf("failed to copy static assets: %w", err)
	}

	fmt.Println("==> Awen site successfully built into", cfg.DistDir)
	return nil
}

func loadArticles(cfg Config) ([]Article, error) {
	searchDirs := []string{cfg.BlogDir, "articles"}
	var files []string

	for _, dir := range searchDirs {
		if stat, err := os.Stat(dir); err == nil && stat.IsDir() {
			entries, _ := os.ReadDir(dir)
			for _, entry := range entries {
				if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".md") {
					files = append(files, filepath.Join(dir, entry.Name()))
				}
			}
			if len(files) > 0 {
				break
			}
		}
	}

	if len(files) == 0 {
		return nil, fmt.Errorf("no markdown articles found in %s or articles/", cfg.BlogDir)
	}

	mdParser := goldmark.New(
		goldmark.WithExtensions(
			extension.GFM,
			extension.Typographer,
		),
		goldmark.WithRendererOptions(
			html.WithUnsafe(),
		),
	)

	var articles []Article

	for _, filePath := range files {
		content, err := os.ReadFile(filePath)
		if err != nil {
			continue
		}

		frontmatter, body, err := parseFrontmatterAndBody(content)
		if err != nil {
			fmt.Printf("Warning: failed to parse frontmatter for %s: %v\n", filePath, err)
			continue
		}

		// Slug determination
		slug := frontmatter.Slug
		if slug == "" {
			slug = strings.TrimSuffix(filepath.Base(filePath), ".md")
		}

		// Date parsing
		var dt time.Time
		if frontmatter.Date != "" {
			for _, layout := range []string{"2006-01-02", "2006-01-02T15:04:05Z", time.RFC3339} {
				if t, parseErr := time.Parse(layout, frontmatter.Date); parseErr == nil {
					dt = t
					break
				}
			}
		}
		if dt.IsZero() {
			dt = time.Now()
		}
		dateFormatted := dt.Format("Jan 2, 2006")

		// Reading time
		words := len(strings.Fields(string(body)))
		readingTime := frontmatter.ReadingTime
		if readingTime == "" {
			minutes := int(math.Ceil(float64(words) / 200.0))
			if minutes < 1 {
				minutes = 1
			}
			readingTime = fmt.Sprintf("%d min read", minutes)
		}

		// Author
		author := frontmatter.Author
		if author == "" {
			author = "Awen Studio"
		}
		initials := getInitials(author)

		// Convert Markdown to HTML
		var htmlBuf bytes.Buffer
		if err := mdParser.Convert(body, &htmlBuf); err != nil {
			fmt.Printf("Warning: goldmark conversion failed for %s: %v\n", filePath, err)
			continue
		}

		// Plain text for search indexing
		plainText := stripMarkdownAndHTML(string(body))

		// Read dynamic claps & comments from data/<slug>.yaml
		claps, comments := loadDynamicData(cfg.DataDir, slug)

		articles = append(articles, Article{
			Title:          frontmatter.Title,
			Slug:           slug,
			Date:           dt.Format("2006-01-02"),
			DateFormatted:  dateFormatted,
			DateTime:       dt,
			Author:         author,
			AuthorInitials: initials,
			Summary:        frontmatter.Summary,
			Tags:           frontmatter.Tags,
			ReadingTime:    readingTime,
			Featured:       frontmatter.Featured,
			Claps:          claps,
			Comments:       comments,
			RawMarkdown:    string(body),
			HTMLContent:    template.HTML(htmlBuf.String()),
			PlainText:      plainText,
		})
	}

	// Sort articles by date descending
	sort.Slice(articles, func(i, j int) bool {
		return articles[i].DateTime.After(articles[j].DateTime)
	})

	return articles, nil
}

func parseFrontmatterAndBody(data []byte) (ArticleFrontmatter, []byte, error) {
	var fm ArticleFrontmatter
	text := string(data)

	if !strings.HasPrefix(strings.TrimSpace(text), "---") {
		return fm, data, nil
	}

	trimmed := strings.TrimLeft(text, "\r\n\t ")
	parts := strings.SplitN(trimmed, "---", 3)
	if len(parts) < 3 {
		return fm, data, nil
	}

	yamlContent := parts[1]
	body := strings.TrimSpace(parts[2])

	if err := yaml.Unmarshal([]byte(yamlContent), &fm); err != nil {
		return fm, nil, err
	}

	return fm, []byte(body), nil
}

func loadDynamicData(dataDir, slug string) (int, []CommentView) {
	possiblePaths := []string{
		filepath.Join(dataDir, slug+".yaml"),
		filepath.Join(dataDir, "blog", slug+".yaml"),
		filepath.Join("data", "blog", slug+".yaml"),
		filepath.Join("data", slug+".yaml"),
	}

	var content []byte
	var err error
	for _, p := range possiblePaths {
		content, err = os.ReadFile(p)
		if err == nil {
			break
		}
	}
	if err != nil || len(content) == 0 {
		return 0, nil
	}

	var dyn ArticleDynamicData
	if err := yaml.Unmarshal(content, &dyn); err != nil {
		return 0, nil
	}

	var commentViews []CommentView
	for _, c := range dyn.Comments {
		createdAtStr := c.CreatedAt.Format(time.RFC3339)
		if c.CreatedAt.IsZero() {
			createdAtStr = time.Now().Format(time.RFC3339)
		}
		commentViews = append(commentViews, CommentView{
			ID:             c.ID,
			Author:         c.Author,
			AuthorInitials: getInitials(c.Author),
			Content:        c.Content,
			CreatedAt:      createdAtStr,
			DateFormatted:  c.CreatedAt.Format("Jan 2, 2006"),
		})
	}

	return dyn.Claps, commentViews
}

type Templates struct {
	Layout  *template.Template
	Index   *template.Template
	Article *template.Template
	About   *template.Template
}

func parseTemplates(publicDir string) (*Templates, error) {
	templatesDir := filepath.Join(publicDir, "templates")

	layoutContent, err := os.ReadFile(filepath.Join(templatesDir, "layout.html"))
	if err != nil {
		return nil, fmt.Errorf("missing layout.html: %w", err)
	}

	indexContent, err := os.ReadFile(filepath.Join(templatesDir, "index.html"))
	if err != nil {
		return nil, fmt.Errorf("missing index.html: %w", err)
	}

	articleContent, err := os.ReadFile(filepath.Join(templatesDir, "article.html"))
	if err != nil {
		return nil, fmt.Errorf("missing article.html: %w", err)
	}

	aboutContent, err := os.ReadFile(filepath.Join(templatesDir, "about.html"))
	if err != nil {
		return nil, fmt.Errorf("missing about.html: %w", err)
	}

	layoutTmpl, err := template.New("layout").Parse(string(layoutContent))
	if err != nil {
		return nil, err
	}

	indexTmpl, err := template.New("index").Parse(string(indexContent))
	if err != nil {
		return nil, err
	}

	articleTmpl, err := template.New("article").Parse(string(articleContent))
	if err != nil {
		return nil, err
	}

	aboutTmpl, err := template.New("about").Parse(string(aboutContent))
	if err != nil {
		return nil, err
	}

	return &Templates{
		Layout:  layoutTmpl,
		Index:   indexTmpl,
		Article: articleTmpl,
		About:   aboutTmpl,
	}, nil
}

func renderPage(tmpl *Templates, contentTmpl *template.Template, data PageData, outPath string) error {
	var bodyBuf bytes.Buffer
	if err := contentTmpl.Execute(&bodyBuf, data); err != nil {
		return fmt.Errorf("error executing body template: %w", err)
	}

	data.Content = template.HTML(bodyBuf.String())

	var fullBuf bytes.Buffer
	if err := tmpl.Layout.Execute(&fullBuf, data); err != nil {
		return fmt.Errorf("error executing layout template: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(outPath), 0755); err != nil {
		return err
	}

	return os.WriteFile(outPath, fullBuf.Bytes(), 0644)
}

func generatePaginatedIndex(cfg Config, tmpl *Templates, articles []Article, allTags []string) error {
	totalArticles := len(articles)
	pageSize := cfg.PageSize
	if pageSize < 1 {
		pageSize = 3
	}

	totalPages := int(math.Ceil(float64(totalArticles) / float64(pageSize)))
	if totalPages < 1 {
		totalPages = 1
	}

	for page := 1; page <= totalPages; page++ {
		start := (page - 1) * pageSize
		end := start + pageSize
		if end > totalArticles {
			end = totalArticles
		}
		pageArticles := articles[start:end]

		var prevURL, nextURL string
		if page > 1 {
			if page == 2 {
				prevURL = "/"
			} else {
				prevURL = fmt.Sprintf("/page/%d/", page-1)
			}
		}
		if page < totalPages {
			nextURL = fmt.Sprintf("/page/%d/", page+1)
		}

		var links []PaginationLink
		for p := 1; p <= totalPages; p++ {
			url := "/"
			if p > 1 {
				url = fmt.Sprintf("/page/%d/", p)
			}
			links = append(links, PaginationLink{
				PageNum:   p,
				URL:       url,
				IsCurrent: p == page,
			})
		}

		pageData := PageData{
			Title:           "Awen — Inquiries into Politics, Technology & History",
			Description:     "A monochromatic publication exploring politics, technology, history, and the ideas shaping our collective future.",
			ActiveNav:       "essays",
			ShowHero:        page == 1,
			Articles:        pageArticles,
			TotalArticles:   totalArticles,
			AllTags:         allTags,
			TotalPages:      totalPages,
			CurrentPage:     page,
			PrevPageURL:     prevURL,
			NextPageURL:     nextURL,
			PaginationLinks: links,
		}

		var outPath string
		if page == 1 {
			outPath = filepath.Join(cfg.DistDir, "index.html")
		} else {
			outPath = filepath.Join(cfg.DistDir, "page", fmt.Sprintf("%d", page), "index.html")
		}

		if err := renderPage(tmpl, tmpl.Index, pageData, outPath); err != nil {
			return err
		}
	}

	return nil
}

func generateTagArchives(cfg Config, tmpl *Templates, articles []Article, allTags []string) error {
	for _, tag := range allTags {
		var matched []Article
		for _, a := range articles {
			for _, t := range a.Tags {
				if strings.EqualFold(t, tag) {
					matched = append(matched, a)
					break
				}
			}
		}

		pageData := PageData{
			Title:          fmt.Sprintf("Topic: %s — Awen", tag),
			Description:    fmt.Sprintf("Essays and reflections archived under #%s.", tag),
			ActiveNav:      "essays",
			ShowHero:       false,
			Articles:       matched,
			TotalArticles:  len(matched),
			AllTags:        allTags,
			TagArchiveName: tag,
			TotalPages:     1,
			CurrentPage:    1,
		}

		outPath := filepath.Join(cfg.DistDir, "tags", strings.ToLower(tag), "index.html")
		if err := renderPage(tmpl, tmpl.Index, pageData, outPath); err != nil {
			return err
		}
	}
	return nil
}

func generateArticlePages(cfg Config, tmpl *Templates, articles []Article) error {
	for _, a := range articles {
		artCopy := a
		pageData := PageData{
			Title:        artCopy.Title,
			Description:  artCopy.Summary,
			ActiveNav:    "essays",
			Article:      &artCopy,
			CanonicalURL: fmt.Sprintf("%s/blog/%s/", strings.TrimRight(cfg.BaseURL, "/"), artCopy.Slug),
		}

		outPath := filepath.Join(cfg.DistDir, "blog", artCopy.Slug, "index.html")
		if err := renderPage(tmpl, tmpl.Article, pageData, outPath); err != nil {
			return err
		}
	}
	return nil
}

func generateAboutPage(cfg Config, tmpl *Templates) error {
	pageData := PageData{
		Title:       "About Awen — Politics, Technology & History",
		Description: "The philosophy, historical perspectives, and technology behind Awen.",
		ActiveNav:   "about",
	}

	outPath := filepath.Join(cfg.DistDir, "about", "index.html")
	return renderPage(tmpl, tmpl.About, pageData, outPath)
}

func generateSearchIndex(cfg Config, articles []Article) error {
	var index []SearchIndexEntry

	for _, a := range articles {
		index = append(index, SearchIndexEntry{
			Title:         a.Title,
			Slug:          a.Slug,
			Date:          a.Date,
			DateFormatted: a.DateFormatted,
			Summary:       a.Summary,
			Tags:          a.Tags,
			ReadingTime:   a.ReadingTime,
			Claps:         a.Claps,
			Content:       a.PlainText,
		})
	}

	jsonData, err := json.MarshalIndent(index, "", "  ")
	if err != nil {
		return err
	}

	outPath := filepath.Join(cfg.DistDir, "search.json")
	return os.WriteFile(outPath, jsonData, 0644)
}

// RSS Structures
type RSSFeed struct {
	XMLName xml.Name   `xml:"rss"`
	Version string     `xml:"version,attr"`
	Atom    string     `xml:"xmlns:atom,attr"`
	Content string     `xml:"xmlns:content,attr"`
	Channel RSSChannel `xml:"channel"`
}

type RSSChannel struct {
	Title         string    `xml:"title"`
	Link          string    `xml:"link"`
	Description   string    `xml:"description"`
	Language      string    `xml:"language"`
	LastBuildDate string    `xml:"lastBuildDate"`
	AtomLink      AtomLink  `xml:"atom:link"`
	Items         []RSSItem `xml:"item"`
}

type AtomLink struct {
	Href string `xml:"href,attr"`
	Rel  string `xml:"rel,attr"`
	Type string `xml:"type,attr"`
}

type RSSItem struct {
	Title          string         `xml:"title"`
	Link           string         `xml:"link"`
	Guid           RSSGuid        `xml:"guid"`
	PubDate        string         `xml:"pubDate"`
	Description    string         `xml:"description"`
	ContentEncoded CDATAContainer `xml:"content:encoded"`
	Author         string         `xml:"author,omitempty"`
	Categories     []string       `xml:"category"`
}

type RSSGuid struct {
	Value       string `xml:",chardata"`
	IsPermaLink bool   `xml:"isPermaLink,attr"`
}

type CDATAContainer struct {
	Value string `xml:",cdata"`
}

func generateRSSFeed(cfg Config, articles []Article) error {
	baseURL := strings.TrimRight(cfg.BaseURL, "/")
	var items []RSSItem

	for _, a := range articles {
		articleURL := fmt.Sprintf("%s/blog/%s/", baseURL, a.Slug)
		pubDate := a.DateTime.Format(time.RFC1123Z)

		items = append(items, RSSItem{
			Title:   a.Title,
			Link:    articleURL,
			Guid:    RSSGuid{Value: articleURL, IsPermaLink: true},
			PubDate: pubDate,
			Description: a.Summary,
			ContentEncoded: CDATAContainer{Value: string(a.HTMLContent)},
			Author:      a.Author,
			Categories:  a.Tags,
		})
	}

	feed := RSSFeed{
		Version: "2.0",
		Atom:    "http://www.w3.org/2005/Atom",
		Content: "http://purl.org/rss/1.0/modules/content/",
		Channel: RSSChannel{
			Title:         "Awen — Inquiries into Politics, Technology & History",
			Link:          baseURL + "/",
			Description:   "A monochromatic publication exploring politics, technology, history, and the ideas shaping our collective future.",
			Language:      "en-us",
			LastBuildDate: time.Now().Format(time.RFC1123Z),
			AtomLink: AtomLink{
				Href: baseURL + "/feed.xml",
				Rel:  "self",
				Type: "application/rss+xml",
			},
			Items: items,
		},
	}

	xmlBytes, err := xml.MarshalIndent(feed, "", "  ")
	if err != nil {
		return err
	}

	fullXML := []byte(xml.Header + string(xmlBytes))

	// Write both feed.xml and rss.xml for maximum client compatibility
	if err := os.WriteFile(filepath.Join(cfg.DistDir, "feed.xml"), fullXML, 0644); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(cfg.DistDir, "rss.xml"), fullXML, 0644)
}

func copyStaticAssets(srcDir, destDir string) error {
	return filepath.Walk(srcDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(srcDir, path)
		if err != nil {
			return err
		}

		// Skip templates directory since templates are compiled into HTML
		if relPath == "templates" || strings.HasPrefix(relPath, "templates"+string(filepath.Separator)) {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		targetPath := filepath.Join(destDir, relPath)

		if info.IsDir() {
			return os.MkdirAll(targetPath, 0755)
		}

		srcFile, err := os.Open(path)
		if err != nil {
			return err
		}
		defer srcFile.Close()

		destFile, err := os.Create(targetPath)
		if err != nil {
			return err
		}
		defer destFile.Close()

		_, err = io.Copy(destFile, srcFile)
		return err
	})
}

var htmlTagRegex = regexp.MustCompile(`<[^>]*>`)

func stripMarkdownAndHTML(s string) string {
	cleaned := htmlTagRegex.ReplaceAllString(s, " ")
	cleaned = strings.ReplaceAll(cleaned, "#", "")
	cleaned = strings.ReplaceAll(cleaned, "*", "")
	cleaned = strings.ReplaceAll(cleaned, "`", "")
	cleaned = strings.ReplaceAll(cleaned, ">", "")
	cleaned = strings.ReplaceAll(cleaned, "[", "")
	cleaned = strings.ReplaceAll(cleaned, "]", "")
	cleaned = strings.ReplaceAll(cleaned, "(", "")
	cleaned = strings.ReplaceAll(cleaned, ")", "")
	return strings.Join(strings.Fields(cleaned), " ")
}

func getInitials(name string) string {
	parts := strings.Fields(name)
	if len(parts) == 0 {
		return "A"
	}
	if len(parts) == 1 {
		runes := []rune(parts[0])
		if len(runes) >= 2 {
			return strings.ToUpper(string(runes[:2]))
		}
		return strings.ToUpper(string(runes))
	}
	first := []rune(parts[0])
	last := []rune(parts[len(parts)-1])
	return strings.ToUpper(string(first[0]) + string(last[0]))
}
