package app

import (
	"bytes"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"time"

	ui "ipmanlk/breeze/ui"

	"github.com/go-chi/chi/v5"
)

func setupUIHandler(r chi.Router, logger *slog.Logger) error {
	spa, err := newSPAHandler(ui.Dist, logger)
	if err != nil {
		return err
	}
	r.NotFound(spa.serve)
	return nil
}

type spaHandler struct {
	static    fs.FS
	indexData []byte
}

func newSPAHandler(distFS fs.FS, logger *slog.Logger) (*spaHandler, error) {
	static, err := fs.Sub(distFS, "dist")
	if err != nil {
		return nil, fmt.Errorf("embedded UI: failed to resolve dist/: %w", err)
	}
	indexData, err := fs.ReadFile(static, "index.html")
	if err != nil {
		return nil, fmt.Errorf("embedded UI: failed to read index.html (build the UI first: make build-ui): %w", err)
	}
	return &spaHandler{static: static, indexData: indexData}, nil
}

func (s *spaHandler) serve(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		if _, err := fs.Stat(s.static, r.URL.Path[1:]); err == nil {
			http.FileServer(http.FS(s.static)).ServeHTTP(w, r)
			return
		}
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	http.ServeContent(w, r, "index.html", time.Time{}, bytes.NewReader(s.indexData))
}
