package server

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"io/fs"
	"net/http"
	"path/filepath"

	"github.com/nais/fasit/internal/auth"
	"github.com/nais/fasit/internal/database"
	"github.com/nais/fasit/internal/ui/layout"
	"github.com/sirupsen/logrus"
)

var mime = map[string]string{
	".js":   "application/javascript",
	".css":  "text/css",
	".html": "text/html",
	".ico":  "image/x-icon",
	"":      "text/plain",
}

type Server struct {
	siteFS       fs.FS
	repo         database.Repo
	assetVersion string
}

func New(siteFS fs.FS, repo database.Repo) *Server {
	return &Server{
		siteFS:       siteFS,
		repo:         repo,
		assetVersion: computeAssetVersion(siteFS),
	}
}

func computeAssetVersion(siteFS fs.FS) string {
	h := sha256.New()
	for _, file := range []string{"site/style.css", "site/site.js", "site/favicon.ico"} {
		f, err := siteFS.Open(file)
		if err != nil {
			continue
		}
		_, _ = io.Copy(h, f)
		_ = f.Close()
	}
	return hex.EncodeToString(h.Sum(nil))[:12]
}

func (s *Server) renderPage(w http.ResponseWriter, r *http.Request, props layout.Props) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	props.UserEmail = auth.GetEmail(r.Context())
	props.AssetVersion = s.assetVersion
	err := layout.Page(props).Render(w)
	if err != nil {
		logrus.WithError(err).Error("error rendering page")
	}
}

func (s *Server) serveFile(w http.ResponseWriter, r *http.Request, file string) {
	data, err := s.siteFS.Open("site/" + file)
	if err != nil {
		logrus.WithFields(logrus.Fields{"file": file}).WithError(err).Error("error opening file")
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer func() { _ = data.Close() }()
	ext := filepath.Ext(file)
	w.Header().Set("Content-Type", mime[ext])
	if r.URL.Query().Get("v") == s.assetVersion {
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	} else {
		w.Header().Set("Cache-Control", "public, max-age=300, must-revalidate")
		w.Header().Set("ETag", `"`+s.assetVersion+`"`)
		if match := r.Header.Get("If-None-Match"); match == `"`+s.assetVersion+`"` {
			w.WriteHeader(http.StatusNotModified)
			return
		}
	}
	_, err = io.Copy(w, data)
	if err != nil {
		logrus.WithFields(logrus.Fields{"file": file}).WithError(err).Error("error serving file")
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}
