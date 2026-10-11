//go:build ignore

package main

import (
	"flag"
	"fmt"
	"os"

	"awen/pkg/generator"
)

func main() {
	blogDir := flag.String("blog", "blog", "Directory containing markdown blog articles")
	publicDir := flag.String("public", "public", "Directory containing static assets and templates")
	dataDir := flag.String("data", "data/blog", "Directory containing dynamic article YAML data")
	distDir := flag.String("dist", "dist", "Output directory for the compiled static site")
	baseURL := flag.String("baseurl", "http://localhost:8080", "Base URL for canonical links and RSS feeds")
	pageSize := flag.Int("pagesize", 3, "Number of articles per paginated page")
	flag.Parse()

	cfg := generator.Config{
		BlogDir:   *blogDir,
		PublicDir: *publicDir,
		DataDir:   *dataDir,
		DistDir:   *distDir,
		BaseURL:   *baseURL,
		PageSize:  *pageSize,
	}

	if err := generator.Build(cfg); err != nil {
		fmt.Fprintf(os.Stderr, "Error generating site: %v\n", err)
		os.Exit(1)
	}
}
