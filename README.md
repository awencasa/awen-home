# Awen — Inquiries into Politics, Technology & History

**Awen** (*Welsh: poetic inspiration, creative flow, divine spark*) is a monochromatic, refined static blog and publication engine built with Go.

It pairs the permanence and speed of ahead-of-time static generation with lightweight, sovereign dynamic interactions—allowing unlimited claps and reader reflections saved directly in transparent, human-readable YAML files.

---

## Highlights

- **Monochromatic & Refined Aesthetic:** Pure black and white editorial palette, careful typographic hierarchy, hairline dividers, and balanced negative space.
- **Light & Dark Mode:** Zero-flash theme switching (`system`, `light`, `dark`), persistent across sessions via `localStorage`.
- **Ahead-of-Time Static Generation (`generator.go`):** Compiles Markdown articles from `blog/` with YAML frontmatter, generating complete HTML pages, RSS feeds (`feed.xml` and `rss.xml`), and search index into `dist/`.
- **Interactive Backend (`main.go`):** Hosts the static site while providing real-time REST API endpoints for claps and comments.
- **YAML Data Persistence (`data/blog/`):** Dynamic article data is stored in individual YAML files (`data/blog/<slug>.yaml`) tracking claps, reader comments, and future metadata.
- **Unlimited Claps:** Readers can tap the clap button indefinitely with fluid animations and debounced batch synchronization.
- **Reader Reflections / Comments:** Minimalist commenting system with validation, unique ID generation, and live insertion.
- **Full Paging & Tag Filtering:** Paginated archive (`/page/2/`), static tag archives (`/tags/<tag>/`), and instantaneous client-side filtering.
- **Instant Search:** Zero-dependency search querying titles, summaries, tags, and full text with instant result previews.
- **Standard RSS 2.0:** Auto-generated RSS 2.0 feed with full article content, RFC822 dates, and categories.

---

## Project Structure

```
awen-home/
├── blog/                      # Markdown articles with YAML frontmatter
│   ├── the-architecture-of-silence.md
│   ├── on-monochrome-and-clarity.md
│   ├── the-craft-of-digital-tools.md
│   ├── rhythm-in-code-and-prose.md
│   ├── impermanence-and-the-web.md
│   └── a-dialogue-with-shadows.md
├── public/                    # Static assets and template definitions
│   ├── css/
│   │   └── style.css          # Monochromatic design system & themes
│   ├── js/
│   │   └── main.js            # Theme toggle, search, claps, comments
│   ├── templates/             # HTML templates (layout, index, article, about)
│   │   ├── layout.html
│   │   ├── index.html
│   │   ├── article.html
│   │   └── about.html
│   └── favicon.svg            # Three rays of Awen icon
├── data/
│   └── blog/                  # Dynamic data (one YAML file per article)
│       ├── the-architecture-of-silence.yaml
│       └── ...
├── pkg/
│   └── generator/             # Core static generation logic
│       └── generator.go
├── dist/                      # Compiled production static site
├── generator.go               # Static site generator CLI entrypoint
├── main.go                    # HTTP server & dynamic API backend
├── Makefile                   # Helper targets
└── README.md
```

---

## Quick Start

### 1. Generate the Static Site

Compile markdown from `blog/` and assets from `public/` into `dist/`:

```bash
go run generator.go
```

Or using `make`:

```bash
make generate
```

### 2. Run the Server

Start the backend server on `http://localhost:8080`:

```bash
go run main.go
```

Or on a custom port:

```bash
go run main.go -port 3000
```

*(Note: If `dist/` has not yet been generated, `main.go` will automatically build it on startup).*

---

## API Reference

| Endpoint | Method | Description |
| :--- | :--- | :--- |
| `GET /api/articles/{slug}/data` | `GET` | Returns article dynamic data (claps count and comment list) |
| `POST /api/articles/{slug}/clap` | `POST` | Increments claps (unlimited) with `{ "count": N }` |
| `POST /api/articles/{slug}/comments` | `POST` | Posts a new reflection with `{ "author": "...", "content": "..." }` |
| `GET /api/stats` | `GET` | Returns total articles, total claps, and total comments |

---

## Adding New Essays

To publish a new essay, create a markdown file in `blog/<slug>.md`:

```markdown
---
title: "The Silent Geometry of Type"
slug: "the-silent-geometry-of-type"
date: "2026-10-10"
author: "Awen Studio"
summary: "An exploration of font metrics, x-height, and rhythm in printed prose."
tags: ["Design", "Typography"]
reading_time: "4 min"
featured: true
---

# The Silent Geometry of Type

Your article content here in standard Markdown...
```

Run `go run generator.go` to rebuild `dist/`. Dynamic data will be automatically tracked in `data/blog/the-silent-geometry-of-type.yaml`.
