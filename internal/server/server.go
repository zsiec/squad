// Package server hosts the HTTP + SSE dashboard for squad. The server takes
// a *sql.DB directly (no store wrapper) and owns one chat.Chat instance whose
// bus is shared with the SSE handler.
package server

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/zsiec/squad/internal/chat"
)

type Config struct {
	Token        string
	Host         string
	Port         int
	SquadDir     string
	RepoID       string
	pingInterval time.Duration
}

type Server struct {
	db          *sql.DB
	chat        *chat.Chat
	cfg         Config
	callerAgent string
	rlMu        sync.Mutex
	rl          map[string][]time.Time
}

func New(db *sql.DB, repoID string, cfg Config) *Server {
	if cfg.SquadDir == "" {
		cfg.SquadDir = ".squad"
	}
	if cfg.pingInterval == 0 {
		cfg.pingInterval = 15 * time.Second
	}
	c := chat.New(db, repoID)
	return &Server{db: db, chat: c, cfg: cfg}
}

func (s *Server) Bus() *chat.Bus { return s.chat.Bus() }

func (s *Server) WithCallerAgent(id string) *Server {
	s.callerAgent = id
	return s
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/health", s.handleHealth)
	mux.HandleFunc("GET /api/items", s.handleItemsList)
	mux.HandleFunc("GET /api/items/{id}", s.handleItemDetail)
	mux.HandleFunc("GET /api/agents", s.handleAgents)
	mux.HandleFunc("GET /api/whoami", s.handleWhoami)
	mux.HandleFunc("GET /api/claims", s.handleClaims)
	mux.HandleFunc("GET /api/messages", s.handleMessagesList)
	mux.HandleFunc("POST /api/messages", s.handleMessagesPost)
	mux.HandleFunc("GET /api/repos", s.handleRepos)
	mux.HandleFunc("GET /api/workspace/status", s.handleWorkspaceStatus)
	mux.HandleFunc("GET /api/events", s.handleEvents)
	return s.authMiddleware(mux)
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) actingAgent(r *http.Request) string {
	if h := r.Header.Get("X-Squad-Agent"); h != "" {
		return h
	}
	return s.callerAgent
}

func (s *Server) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if s.cfg.Token == "" {
			next.ServeHTTP(w, r)
			return
		}
		got := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		if got == "" && r.Method == http.MethodGet {
			if q := r.URL.Query().Get("token"); q != "" {
				got = q
			}
		}
		if got != s.cfg.Token {
			writeErr(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]any{"error": msg})
}
