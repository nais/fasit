package server

import (
	"io"
	"io/fs"
	"net/http"
	"path/filepath"

	"github.com/nais/fasit/internal/database"
	"github.com/nais/fasit/internal/ui/layout"
	"github.com/sirupsen/logrus"
)

var mime = map[string]string{
	".js":   "application/javascript",
	".css":  "text/css",
	".html": "text/html",
	"":      "text/plain",
}

type Server struct {
	siteFS fs.FS
	repo   database.Repo
}

func New(siteFS fs.FS, repo database.Repo) *Server {
	return &Server{
		siteFS: siteFS,
		repo:   repo,
	}
}

func (s *Server) renderPage(w http.ResponseWriter, props layout.Props) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	err := layout.Page(props).Render(w)
	if err != nil {
		logrus.WithError(err).Error("error rendering page")
	}
}

func (s *Server) serveFile(w http.ResponseWriter, file string) {
	data, err := s.siteFS.Open("site/" + file)
	if err != nil {
		logrus.WithFields(logrus.Fields{"file": file}).WithError(err).Error("error opening file")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	ext := filepath.Ext(file)
	w.Header().Set("Content-Type", mime[ext])
	_, err = io.Copy(w, data)
	if err != nil {
		logrus.WithFields(logrus.Fields{"file": file}).WithError(err).Error("error serving file")
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}
