package server

import "net/http"

func (s *Server) CSS(w http.ResponseWriter, r *http.Request) {
	s.serveFile(w, r, "style.css")
}

func (s *Server) JS(w http.ResponseWriter, r *http.Request) {
	s.serveFile(w, r, "site.js")
}

func (s *Server) PageJS(w http.ResponseWriter, r *http.Request) {
	file := r.URL.Path[1:] // strip leading /
	s.serveFile(w, r, file)
}

func (s *Server) Favicon(w http.ResponseWriter, r *http.Request) {
	s.serveFile(w, r, "favicon.ico")
}

func (s *Server) FaviconSVG(w http.ResponseWriter, r *http.Request) {
	s.serveFile(w, r, "favicon.svg")
}
