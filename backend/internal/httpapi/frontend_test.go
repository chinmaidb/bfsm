package httpapi

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/labstack/echo/v5"
)

func TestServeFrontend(t *testing.T) {
	directory := t.TempDir()
	if err := os.WriteFile(filepath.Join(directory, "index.html"), []byte("<html>app</html>"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(directory, "assets"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, "assets", "app.js"), []byte("app()"), 0o600); err != nil {
		t.Fatal(err)
	}

	e := echo.New()
	if err := ServeFrontend(e, directory); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name        string
		path        string
		accept      string
		status      int
		body        string
		contentType string
	}{
		{name: "root", path: "/", accept: "text/html", status: http.StatusOK, body: "<html>app</html>", contentType: "text/html"},
		{name: "browser route", path: "/accounts", accept: "text/html", status: http.StatusOK, body: "<html>app</html>", contentType: "text/html"},
		{name: "asset", path: "/assets/app.js", status: http.StatusOK, body: "app()", contentType: "text/javascript"},
		{name: "missing asset", path: "/assets/missing.js", status: http.StatusNotFound},
		{name: "unknown api route", path: "/api/v1/missing", accept: "text/html", status: http.StatusNotFound},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, test.path, nil)
			if test.accept != "" {
				request.Header.Set(echo.HeaderAccept, test.accept)
			}
			response := httptest.NewRecorder()
			e.ServeHTTP(response, request)
			if response.Code != test.status {
				t.Fatalf("status = %d, want %d; body: %s", response.Code, test.status, response.Body.String())
			}
			if test.body != "" && !strings.Contains(response.Body.String(), test.body) {
				t.Fatalf("body = %q, want it to contain %q", response.Body.String(), test.body)
			}
			if test.contentType != "" && !strings.Contains(response.Header().Get(echo.HeaderContentType), test.contentType) {
				t.Fatalf("content type = %q, want it to contain %q", response.Header().Get(echo.HeaderContentType), test.contentType)
			}
		})
	}
}

func TestServeFrontendRequiresIndex(t *testing.T) {
	if err := ServeFrontend(echo.New(), t.TempDir()); err == nil {
		t.Fatal("ServeFrontend() succeeded without index.html")
	}
}
