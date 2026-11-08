package handler_test

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"flowmetrics/internal/handler"

	"github.com/gin-gonic/gin"
)

// staticSite builds a fake "built UI" in a temp dir, with a secret file one level ABOVE it.
func staticSite(t *testing.T) (router *gin.Engine, root string) {
	t.Helper()
	parent := t.TempDir()
	root = filepath.Join(parent, "public")
	if err := os.MkdirAll(filepath.Join(root, "assets"), 0o755); err != nil {
		t.Fatal(err)
	}
	write := func(p, body string) {
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write(filepath.Join(root, "index.html"), `<!doctype html><div id="root"></div>`)
	write(filepath.Join(root, "assets", "app-abc123.js"), `console.log("app")`)
	write(filepath.Join(root, "robots.txt"), "User-agent: *")
	write(filepath.Join(parent, "secret.txt"), "TOP-SECRET")

	gin.SetMode(gin.ReleaseMode)
	router = gin.New()
	if err := handler.ServeStatic(router, root); err != nil {
		t.Fatal(err)
	}
	return router, root
}

func get(r http.Handler, method, path string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(method, path, nil))
	return w
}

func TestServeStatic(t *testing.T) {
	r, _ := staticSite(t)

	t.Run("root and client-side routes get index.html, with CSP, never cached", func(t *testing.T) {
		for _, p := range []string{"/", "/index.html", "/orgs", "/o/6af03a3775ab9e160f5e8e2e/projects/abc"} {
			w := get(r, "GET", p)
			if w.Code != 200 || !strings.Contains(w.Body.String(), `id="root"`) {
				t.Errorf("GET %s = %d %q", p, w.Code, w.Body.String())
			}
			if csp := w.Header().Get("Content-Security-Policy"); !strings.Contains(csp, "default-src 'self'") || !strings.Contains(csp, "frame-ancestors 'none'") {
				t.Errorf("GET %s: CSP = %q", p, csp)
			}
			if cc := w.Header().Get("Cache-Control"); cc != "no-cache" {
				t.Errorf("GET %s: Cache-Control = %q, want no-cache", p, cc)
			}
		}
		if w := get(r, "HEAD", "/"); w.Code != 200 {
			t.Errorf("HEAD / = %d", w.Code)
		}
	})

	t.Run("hashed assets are served and cached forever", func(t *testing.T) {
		w := get(r, "GET", "/assets/app-abc123.js")
		if w.Code != 200 || w.Body.String() != `console.log("app")` {
			t.Fatalf("asset = %d %q", w.Code, w.Body.String())
		}
		if cc := w.Header().Get("Cache-Control"); !strings.Contains(cc, "immutable") || !strings.Contains(cc, "max-age=31536000") {
			t.Errorf("asset Cache-Control = %q", cc)
		}
		if ct := w.Header().Get("Content-Type"); !strings.Contains(ct, "javascript") {
			t.Errorf("asset Content-Type = %q", ct)
		}
	})

	t.Run("a missing asset is a 404, not HTML (HTML would hide a broken deploy)", func(t *testing.T) {
		for _, p := range []string{"/assets/missing.js", "/assets/"} {
			w := get(r, "GET", p)
			if w.Code != 404 || strings.Contains(w.Body.String(), `id="root"`) {
				t.Errorf("GET %s = %d %q", p, w.Code, w.Body.String())
			}
		}
	})

	t.Run("other real files are served, revalidated", func(t *testing.T) {
		w := get(r, "GET", "/robots.txt")
		if w.Code != 200 || w.Body.String() != "User-agent: *" || w.Header().Get("Cache-Control") != "no-cache" {
			t.Errorf("robots.txt = %d %q cc=%q", w.Code, w.Body.String(), w.Header().Get("Cache-Control"))
		}
	})

	t.Run("unknown API routes stay JSON 404, and non-GET methods never get HTML", func(t *testing.T) {
		w := get(r, "GET", "/api/v1/does-not-exist")
		if w.Code != 404 || !strings.Contains(w.Body.String(), `"code":"not_found"`) || strings.Contains(w.Body.String(), `id="root"`) {
			t.Errorf("GET /api/v1/does-not-exist = %d %q", w.Code, w.Body.String())
		}
		if w := get(r, "POST", "/"); w.Code != 404 {
			t.Errorf("POST / = %d, want 404", w.Code)
		}
	})

	t.Run("path traversal cannot read files outside the web root", func(t *testing.T) {
		for _, p := range []string{"/../secret.txt", "/assets/../../secret.txt", "/%2e%2e/secret.txt", "/assets/..%2f..%2fsecret.txt", "/..%2fsecret.txt"} {
			w := get(r, "GET", p)
			if strings.Contains(w.Body.String(), "TOP-SECRET") {
				t.Errorf("GET %s leaked a file outside the web root", p)
			}
		}
	})
}

func TestServeStaticRequiresBuiltUI(t *testing.T) {
	gin.SetMode(gin.ReleaseMode)
	if err := handler.ServeStatic(gin.New(), t.TempDir()); err == nil || !strings.Contains(err.Error(), "index.html") {
		t.Fatalf("an empty directory must be rejected at startup with a clear error, got %v", err)
	}
}
