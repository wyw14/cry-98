package api

import (
	"net/http"
)

func (s *Server) health(writer http.ResponseWriter, request *http.Request) {
	writeJSON(writer, http.StatusOK, s.deps.Health.Report())
}

func (s *Server) listVessels(writer http.ResponseWriter, request *http.Request) {
	writeJSON(writer, http.StatusOK, map[string]any{"items": s.deps.Vessels.List()})
}
