package server

import "net/http"

func (s *Server) CSS(w http.ResponseWriter, _ *http.Request) {
	s.serveFile(w, "style.css")
}

func (s *Server) JS(w http.ResponseWriter, _ *http.Request) {
	s.serveFile(w, "site.js")
}
