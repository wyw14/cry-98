package api

import (
	"encoding/json"
	"net/http"
)

func (s *Server) streamVessels(writer http.ResponseWriter, request *http.Request) {
	writer.Header().Set("Content-Type", "application/x-ndjson")
	writer.WriteHeader(http.StatusOK)
	snapshot := s.deps.Telemetry.Snapshot()
	encoder := json.NewEncoder(writer)
	for id, vessel := range snapshot.Vessels {
		if err := encoder.Encode(map[string]any{"id": id, "vessel": vessel, "snapshot_at": snapshot.UpdatedAt}); err != nil {
			return
		}
	}
}
