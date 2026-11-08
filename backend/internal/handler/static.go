package handler

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"flowmetrics/internal/apperr"
	"flowmetrics/internal/respond"

	"github.com/gin-gonic/gin"
)

// Same policy the nginx image applies. The access token lives in page memory, so no inline
// scripts and no third-party origins are allowed. (style-src permits inline style attributes.)
const contentSecurityPolicy = "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; " +
	"img-src 'self' data:; connect-src 'self'; base-uri 'self'; form-action 'self'; frame-ancestors 'none'"

// ServeStatic makes the API process also serve the built web UI from dir, so one service can host
// both (the Render deployment). The browser then sees a single origin, which is what lets the
// SameSite=Strict refresh cookie work with no CORS at all. Docker Compose does not use this: it
// keeps nginx in front instead.
//
// Anything under /api/ that matches no route stays a JSON 404; a missing hashed asset is a plain
// 404 (never HTML, which would mask a broken deploy); every other GET falls back to index.html so
// client-side routes survive a refresh.
func ServeStatic(r *gin.Engine, dir string) error {
	index, err := os.ReadFile(filepath.Join(dir, "index.html"))
	if err != nil {
		return fmt.Errorf("STATIC_DIR %q must contain the built web UI (index.html): %w", dir, err)
	}
	files := http.Dir(dir) // Dir.Open rejects ".." traversal

	r.NoRoute(func(c *gin.Context) {
		p := c.Request.URL.Path
		if (c.Request.Method != http.MethodGet && c.Request.Method != http.MethodHead) || strings.HasPrefix(p, "/api/") {
			respond.Error(c, apperr.NotFound("route"))
			return
		}
		c.Header("Content-Security-Policy", contentSecurityPolicy)

		if p != "/" && p != "/index.html" {
			if f, err := files.Open(p); err == nil {
				st, statErr := f.Stat()
				_ = f.Close()
				if statErr == nil && !st.IsDir() {
					if strings.HasPrefix(p, "/assets/") {
						c.Header("Cache-Control", "public, max-age=31536000, immutable") // content-hashed names never change
					} else {
						c.Header("Cache-Control", "no-cache")
					}
					c.FileFromFS(p, files)
					return
				}
			}
			if strings.HasPrefix(p, "/assets/") {
				c.Status(http.StatusNotFound)
				return
			}
		}
		c.Header("Cache-Control", "no-cache")
		c.Data(http.StatusOK, "text/html; charset=utf-8", index)
	})
	return nil
}
