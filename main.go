package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"awen/pkg/generator"
	"gopkg.in/yaml.v3"
)

// DynamicComment represents a reader reflection.
type DynamicComment struct {
	ID        string    `yaml:"id" json:"id"`
	Author    string    `yaml:"author" json:"author"`
	Content   string    `yaml:"content" json:"content"`
	CreatedAt time.Time `yaml:"created_at" json:"created_at"`
}

// DynamicArticleData stores claps and comments in data/<slug>.yaml.
type DynamicArticleData struct {
	Slug     string           `yaml:"slug" json:"slug"`
	Claps    int              `yaml:"claps" json:"claps"`
	Comments []DynamicComment `yaml:"comments" json:"comments"`
}

// DataStore manages thread-safe read/write operations to data/<slug>.yaml.
type DataStore struct {
	dataDir string
	mu      sync.RWMutex
}

func NewDataStore(dataDir string) (*DataStore, error) {
	if dataDir == "data" {
		if stat, err := os.Stat("data/blog"); err == nil && stat.IsDir() {
			dataDir = "data/blog"
		}
	}
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create data dir: %w", err)
	}
	return &DataStore{dataDir: dataDir}, nil
}

func (ds *DataStore) getFilePath(slug string) string {
	// Sanitize slug to prevent path traversal
	cleanSlug := filepath.Base(filepath.Clean(slug))
	cleanSlug = strings.TrimSuffix(cleanSlug, ".yaml")
	return filepath.Join(ds.dataDir, cleanSlug+".yaml")
}

func (ds *DataStore) GetArticleData(slug string) (*DynamicArticleData, error) {
	ds.mu.RLock()
	defer ds.mu.RUnlock()

	filePath := ds.getFilePath(slug)
	content, err := os.ReadFile(filePath)
	if err != nil && os.IsNotExist(err) {
		// Fallback check in parent data/ directory if migrated
		cleanSlug := filepath.Base(filepath.Clean(slug))
		cleanSlug = strings.TrimSuffix(cleanSlug, ".yaml")
		parentPath := filepath.Join("data", cleanSlug+".yaml")
		content, err = os.ReadFile(parentPath)
	}
	if err != nil {
		if os.IsNotExist(err) {
			return &DynamicArticleData{
				Slug:     slug,
				Claps:    0,
				Comments: []DynamicComment{},
			}, nil
		}
		return nil, err
	}

	var data DynamicArticleData
	if err := yaml.Unmarshal(content, &data); err != nil {
		return nil, err
	}
	if data.Comments == nil {
		data.Comments = []DynamicComment{}
	}
	return &data, nil
}

func (ds *DataStore) AddClaps(slug string, count int) (int, error) {
	if count <= 0 {
		count = 1
	}

	ds.mu.Lock()
	defer ds.mu.Unlock()

	filePath := ds.getFilePath(slug)
	var data DynamicArticleData

	content, err := os.ReadFile(filePath)
	if err == nil {
		_ = yaml.Unmarshal(content, &data)
	}

	data.Slug = slug
	data.Claps += count
	if data.Comments == nil {
		data.Comments = []DynamicComment{}
	}

	marshaled, err := yaml.Marshal(&data)
	if err != nil {
		return 0, err
	}

	if err := os.WriteFile(filePath, marshaled, 0644); err != nil {
		return 0, err
	}

	return data.Claps, nil
}

func (ds *DataStore) AddComment(slug, author, commentText string) (*DynamicComment, error) {
	ds.mu.Lock()
	defer ds.mu.Unlock()

	filePath := ds.getFilePath(slug)
	var data DynamicArticleData

	content, err := os.ReadFile(filePath)
	if err == nil {
		_ = yaml.Unmarshal(content, &data)
	}

	data.Slug = slug
	if data.Comments == nil {
		data.Comments = []DynamicComment{}
	}

	// Generate a unique comment ID
	randBytes := make([]byte, 4)
	_, _ = rand.Read(randBytes)
	commentID := fmt.Sprintf("comm-%d-%s", time.Now().Unix(), hex.EncodeToString(randBytes))

	newComment := DynamicComment{
		ID:        commentID,
		Author:    author,
		Content:   commentText,
		CreatedAt: time.Now().UTC(),
	}

	data.Comments = append(data.Comments, newComment)

	marshaled, err := yaml.Marshal(&data)
	if err != nil {
		return nil, err
	}

	if err := os.WriteFile(filePath, marshaled, 0644); err != nil {
		return nil, err
	}

	return &newComment, nil
}

type Server struct {
	distDir string
	store   *DataStore
	mux     *http.ServeMux
}

func NewServer(distDir string, store *DataStore) *Server {
	s := &Server{
		distDir: distDir,
		store:   store,
		mux:     http.NewServeMux(),
	}
	s.routes()
	return s
}

func (s *Server) routes() {
	// API Endpoints
	s.mux.HandleFunc("/api/articles/", s.handleArticleAPI)
	s.mux.HandleFunc("/api/stats", s.handleStatsAPI)

	// Static File Server
	fileServer := http.FileServer(http.Dir(s.distDir))
	s.mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// Log requests cleanly
		start := time.Now()
		defer func() {
			log.Printf("%s %s %s", r.Method, r.URL.Path, time.Since(start))
		}()

		// Set security and caching headers
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "SAMEORIGIN")

		path := r.URL.Path

		// Check if requesting an existing file in dist/
		targetPath := filepath.Join(s.distDir, filepath.Clean(path))
		info, err := os.Stat(targetPath)
		if err == nil {
			if info.IsDir() {
				// Serve directory index.html
				indexPath := filepath.Join(targetPath, "index.html")
				if _, err := os.Stat(indexPath); err == nil {
					http.ServeFile(w, r, indexPath)
					return
				}
			} else {
				// Cache static assets like CSS and JS
				if strings.HasPrefix(path, "/css/") || strings.HasPrefix(path, "/js/") || path == "/favicon.svg" {
					w.Header().Set("Cache-Control", "public, max-age=3600")
				}
				fileServer.ServeHTTP(w, r)
				return
			}
		}

		// Clean URL fallback: e.g. /blog/the-architecture-of-silence -> /blog/the-architecture-of-silence/index.html
		cleanIndexPath := filepath.Join(s.distDir, filepath.Clean(path), "index.html")
		if _, err := os.Stat(cleanIndexPath); err == nil {
			http.ServeFile(w, r, cleanIndexPath)
			return
		}

		// 404 handler
		http.NotFound(w, r)
	})
}

// handleArticleAPI routes /api/articles/{slug}/data, /clap, /comments
func (s *Server) handleArticleAPI(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusOK)
		return
	}

	trimmed := strings.TrimPrefix(r.URL.Path, "/api/articles/")
	parts := strings.Split(trimmed, "/")
	if len(parts) < 2 {
		http.Error(w, `{"error":"invalid endpoint"}`, http.StatusBadRequest)
		return
	}

	slug := parts[0]
	action := parts[1]

	switch action {
	case "data":
		if r.Method != http.MethodGet {
			http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
			return
		}
		data, err := s.store.GetArticleData(slug)
		if err != nil {
			http.Error(w, fmt.Sprintf(`{"error":"%s"}`, err.Error()), http.StatusInternalServerError)
			return
		}
		_ = json.NewEncoder(w).Encode(data)

	case "clap":
		if r.Method != http.MethodPost {
			http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
			return
		}
		var req struct {
			Count int `json:"count"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		if req.Count <= 0 {
			req.Count = 1
		}

		newTotal, err := s.store.AddClaps(slug, req.Count)
		if err != nil {
			http.Error(w, fmt.Sprintf(`{"error":"%s"}`, err.Error()), http.StatusInternalServerError)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"slug":  slug,
			"claps": newTotal,
		})

	case "comments":
		if r.Method == http.MethodGet {
			data, err := s.store.GetArticleData(slug)
			if err != nil {
				http.Error(w, fmt.Sprintf(`{"error":"%s"}`, err.Error()), http.StatusInternalServerError)
				return
			}
			_ = json.NewEncoder(w).Encode(data.Comments)
			return
		}

		if r.Method != http.MethodPost {
			http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
			return
		}

		var req struct {
			Author  string `json:"author"`
			Content string `json:"content"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, `{"error":"invalid json body"}`, http.StatusBadRequest)
			return
		}

		author := strings.TrimSpace(req.Author)
		content := strings.TrimSpace(req.Content)

		if author == "" || content == "" {
			http.Error(w, `{"error":"author and content cannot be empty"}`, http.StatusBadRequest)
			return
		}

		if len(author) > 100 {
			author = author[:100]
		}
		if len(content) > 4000 {
			content = content[:4000]
		}

		comment, err := s.store.AddComment(slug, author, content)
		if err != nil {
			http.Error(w, fmt.Sprintf(`{"error":"%s"}`, err.Error()), http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"success": true,
			"comment": comment,
		})

	default:
		http.Error(w, `{"error":"unknown action"}`, http.StatusNotFound)
	}
}

func (s *Server) handleStatsAPI(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	files, err := os.ReadDir(s.store.dataDir)
	if err != nil {
		http.Error(w, `{"error":"unable to read data dir"}`, http.StatusInternalServerError)
		return
	}

	totalClaps := 0
	totalComments := 0
	articlesTracked := 0

	for _, f := range files {
		if !f.IsDir() && strings.HasSuffix(f.Name(), ".yaml") {
			slug := strings.TrimSuffix(f.Name(), ".yaml")
			data, err := s.store.GetArticleData(slug)
			if err == nil {
				articlesTracked++
				totalClaps += data.Claps
				totalComments += len(data.Comments)
			}
		}
	}

	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"articles_tracked": articlesTracked,
		"total_claps":      totalClaps,
		"total_comments":   totalComments,
	})
}

func main() {
	port := flag.String("port", "", "Server HTTP port (defaults to $PORT or 8080)")
	dataDir := flag.String("data", "data/blog", "Data directory for article YAML files")
	distDir := flag.String("dist", "dist", "Directory of compiled static site")
	autoBuild := flag.Bool("build", false, "Force rebuilding the static site on startup")
	flag.Parse()

	// Port resolution
	resolvedPort := *port
	if resolvedPort == "" {
		resolvedPort = os.Getenv("PORT")
	}
	if resolvedPort == "" {
		resolvedPort = "8080"
	}
	if !strings.HasPrefix(resolvedPort, ":") {
		resolvedPort = ":" + resolvedPort
	}

	// Verify or generate dist directory
	indexFile := filepath.Join(*distDir, "index.html")
	if _, err := os.Stat(indexFile); os.IsNotExist(err) || *autoBuild {
		log.Println("==> Static site not yet generated or build flag set. Compiling site...")
		cfg := generator.DefaultConfig()
		cfg.DistDir = *distDir
		cfg.DataDir = *dataDir
		if err := generator.Build(cfg); err != nil {
			log.Fatalf("Fatal error generating static site: %v", err)
		}
	}

	// Initialize data store
	store, err := NewDataStore(*dataDir)
	if err != nil {
		log.Fatalf("Failed to initialize data store: %v", err)
	}

	server := NewServer(*distDir, store)

	log.Printf("=====================================================")
	log.Printf("  AWEN — Static Web + Monochromatic Editorial Backend")
	log.Printf("  Listening on http://localhost%s", resolvedPort)
	log.Printf("  Static Site: %s", *distDir)
	log.Printf("  Dynamic Data: %s/*.yaml", *dataDir)
	log.Printf("=====================================================")

	if err := http.ListenAndServe(resolvedPort, server.mux); err != nil {
		log.Fatalf("Server terminated: %v", err)
	}
}
