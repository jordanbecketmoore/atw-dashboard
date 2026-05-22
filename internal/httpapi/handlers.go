package httpapi

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/jordanm/atw-dashboard/internal/hub"
)

const heartbeatInterval = 15 * time.Second

type Server struct {
	hub *hub.Hub
}

func NewServer(h *hub.Hub) *Server {
	return &Server{hub: h}
}

func (s *Server) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/state", s.handleState)
	mux.HandleFunc("GET /events", s.handleEvents)
	mux.HandleFunc("GET /healthz", s.handleHealth)
	mux.Handle("GET /", StaticHandler())
}

func (s *Server) handleState(w http.ResponseWriter, r *http.Request) {
	snap := s.hub.Snapshot()
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	if err := json.NewEncoder(w).Encode(snap); err != nil {
		log.Printf("encode state: %v", err)
	}
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
}

func (s *Server) handleEvents(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	flusher.Flush()

	id, ch, unsub := s.hub.Subscribe()
	defer unsub()
	log.Printf("events: subscriber %d connected from %s", id, r.RemoteAddr)
	defer log.Printf("events: subscriber %d disconnected", id)

	ticker := time.NewTicker(heartbeatInterval)
	defer ticker.Stop()

	ctx := r.Context()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if _, err := fmt.Fprintf(w, ": ping\n\n"); err != nil {
				return
			}
			flusher.Flush()
		case ev, open := <-ch:
			if !open {
				return
			}
			payload, err := json.Marshal(ev.Payload)
			if err != nil {
				log.Printf("events: subscriber %d marshal %s failed: %v", id, ev.Type, err)
				continue
			}
			if _, err := fmt.Fprintf(w, "event: %s\ndata: %s\n\n", ev.Type, payload); err != nil {
				log.Printf("events: subscriber %d write %s failed: %v", id, ev.Type, err)
				return
			}
			preview := payload
			if len(preview) > 300 {
				preview = preview[:300]
			}
			log.Printf("events: subscriber %d -> %s (%d bytes): %s", id, ev.Type, len(payload), preview)
			flusher.Flush()
		}
	}
}
