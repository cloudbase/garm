package assets

import (
	"embed"
	"net/http"
	"path/filepath"
	"strings"
)

//go:generate go run github.com/go-swagger/go-swagger/cmd/swagger@v0.31.0 generate spec --output=../swagger.yaml --scan-models --work-dir=../../
//go:generate go run github.com/go-swagger/go-swagger/cmd/swagger@v0.31.0 validate ../swagger.yaml
//go:generate rm -rf ../src/lib/api/generated
//go:generate openapi-generator-cli generate --skip-validate-spec -i ../swagger.yaml -g typescript-axios -o ../src/lib/api/generated

//go:embed all:*
var EmbeddedSPA embed.FS

// GetSPAFileSystem returns the embedded SPA file system for use with http.FileServer
func GetSPAFileSystem() http.FileSystem {
	return http.FS(EmbeddedSPA)
}

// ServeSPA serves the embedded SPA with proper content types and SPA routing
// This is kept for backward compatibility
func ServeSPA(w http.ResponseWriter, r *http.Request) {
	ServeSPAWithPath(w, r, "/ui/")
}

// ServeSPAWithPath serves the embedded SPA with a custom webapp path
func ServeSPAWithPath(w http.ResponseWriter, r *http.Request, webappPath string) {
	filename := strings.TrimPrefix(r.URL.Path, webappPath)

	// Handle root path and SPA routing - serve index.html for all routes
	if filename == "" || !strings.Contains(filename, ".") {
		filename = "index.html"
	}

	// Security check - prevent directory traversal
	if strings.Contains(filename, "..") {
		http.NotFound(w, r)
		return
	}

	// Read file from embedded filesystem
	content, err := EmbeddedSPA.ReadFile(filename)
	if err != nil {
		// If file not found, serve index.html for SPA routing
		content, err = EmbeddedSPA.ReadFile("index.html")
		if err != nil {
			http.NotFound(w, r)
			return
		}
		filename = "index.html"
	}

	// Set appropriate content type based on file extension
	ext := strings.ToLower(filepath.Ext(filename))
	switch ext {
	case ".html":
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
	case ".js":
		w.Header().Set("Content-Type", "application/javascript")
	case ".css":
		w.Header().Set("Content-Type", "text/css")
	case ".json":
		w.Header().Set("Content-Type", "application/json")
	case ".svg":
		w.Header().Set("Content-Type", "image/svg+xml")
	case ".png":
		w.Header().Set("Content-Type", "image/png")
	default:
		w.Header().Set("Content-Type", "text/plain")
	}

	// Set cache headers for static assets (but not for HTML to ensure fresh content)
	if ext != ".html" {
		w.Header().Set("Cache-Control", "public, max-age=3600")
	} else {
		w.Header().Set("Cache-Control", "no-cache, must-revalidate")
	}

	w.Write(content)
}
