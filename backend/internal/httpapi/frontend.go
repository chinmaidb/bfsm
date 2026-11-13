package httpapi

import (
	"fmt"
	"mime"
	"net/http"
	"os"
	"path"
	"strings"

	"github.com/labstack/echo/v5"
)

func ServeFrontend(e *echo.Echo, directory string) error {
	root := http.Dir(directory)
	index, err := root.Open("index.html")
	if err != nil {
		return fmt.Errorf("open frontend index in %q: %w", directory, err)
	}
	info, err := index.Stat()
	index.Close()
	if err != nil {
		return fmt.Errorf("stat frontend index in %q: %w", directory, err)
	}
	if info.IsDir() {
		return fmt.Errorf("frontend index in %q is not a file", directory)
	}

	serve := func(c *echo.Context) error {
		request := c.Request()
		if request.Method != http.MethodGet && request.Method != http.MethodHead {
			return echo.ErrNotFound
		}

		requestPath := strings.TrimPrefix(request.URL.Path, "/")
		if requestPath == "" {
			requestPath = "index.html"
		}
		if requestPath == "api" || strings.HasPrefix(requestPath, "api/") {
			return echo.ErrNotFound
		}

		file, err := root.Open(requestPath)
		if err == nil {
			info, statErr := file.Stat()
			if statErr == nil && !info.IsDir() {
				defer file.Close()
				if contentType := mime.TypeByExtension(path.Ext(requestPath)); contentType != "" {
					c.Response().Header().Set(echo.HeaderContentType, contentType)
				}
				http.ServeContent(c.Response(), request, path.Base(requestPath), info.ModTime(), file)
				return nil
			}
			file.Close()
			if statErr != nil {
				return statErr
			}
		} else if !os.IsNotExist(err) {
			return err
		}

		if path.Ext(requestPath) != "" || !strings.Contains(request.Header.Get(echo.HeaderAccept), "text/html") {
			return echo.ErrNotFound
		}

		index, err := root.Open("index.html")
		if err != nil {
			return fmt.Errorf("open frontend index in %q: %w", directory, err)
		}
		defer index.Close()
		info, err := index.Stat()
		if err != nil {
			return err
		}
		c.Response().Header().Set(echo.HeaderContentType, "text/html; charset=utf-8")
		http.ServeContent(c.Response(), request, "index.html", info.ModTime(), index)
		return nil
	}

	e.GET("/", serve)
	e.GET("/*", serve)
	return nil
}
