package server

import (
	"encoding/json"
	"net/http"
	"time"
)

func (s *Server) handleEvents(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeErr(w, http.StatusInternalServerError, "streaming unsupported")
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)
	flusher.Flush()

	ch := s.Bus().Subscribe()
	defer s.Bus().Unsubscribe(ch)

	ping := time.NewTicker(s.cfg.pingInterval)
	defer ping.Stop()

	// Dedupe by message id: server-mediated POSTs publish via chat.Post AND
	// the pump re-publishes from the DB on its next tick. Both paths carry
	// the row's id in the payload; track what we've already emitted.
	seen := make(map[float64]bool)

	for {
		select {
		case <-r.Context().Done():
			return
		case <-ping.C:
			// Surface any drops that the publish-side lag injection missed
			// (e.g., a burst-then-quiesce pattern where no later Publish
			// carries the sentinel). PullDropped atomically claims the
			// current counter; a positive value means the client should
			// refetch from MAX(id) to recover state.
			if n := s.Bus().PullDropped(ch); n > 0 {
				if _, err := w.Write([]byte("event: lag\ndata: ")); err != nil {
					return
				}
				payload, _ := json.Marshal(map[string]any{"dropped": n})
				if _, err := w.Write(payload); err != nil {
					return
				}
				if _, err := w.Write([]byte("\n\n")); err != nil {
					return
				}
			}
			if _, err := w.Write([]byte(": ping\n\n")); err != nil {
				return
			}
			flusher.Flush()
		case e, ok := <-ch:
			if !ok {
				return
			}
			if v, ok := e.Payload["id"]; ok {
				var f float64
				switch id := v.(type) {
				case float64:
					f = id
				case int64:
					f = float64(id)
				}
				if f != 0 {
					if seen[f] {
						continue
					}
					seen[f] = true
				}
			}
			payload, _ := json.Marshal(e)
			if _, err := w.Write([]byte("event: " + e.Kind + "\ndata: ")); err != nil {
				return
			}
			if _, err := w.Write(payload); err != nil {
				return
			}
			if _, err := w.Write([]byte("\n\n")); err != nil {
				return
			}
			flusher.Flush()
		}
	}
}
