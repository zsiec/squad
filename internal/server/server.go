// Package server hosts the HTTP + SSE dashboard for squad. The server takes
// a *sql.DB directly (no store wrapper) and owns one chat.Chat instance whose
// bus is shared with the SSE handler.
package server

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"github.com/zsiec/squad/internal/chat"
)

type Config struct {
	Host             string
	Port             int
	SquadDir         string
	RepoID           string
	LearningsRoot    string
	Version          string
	BinaryPath       string
	pingInterval     time.Duration
	lagFlushInterval time.Duration
}

type Server struct {
	db                 *sql.DB
	chat               *chat.Chat
	cfg                Config
	callerAgent        string
	startedAt          time.Time
	exitFunc           func()
	restartExitDelay   time.Duration
	rlMu               sync.Mutex
	rl                 map[string][]time.Time
	pump               *messagesPump
	claimsPump         *claimsPump
	agentsPump         *agentsPump
	activityPump       *activityPump
	autoRefineMu       sync.Mutex
	autoRefineRunner   autoRefineRunner
	autoRefineInFlight map[string]struct{}
}

func New(db *sql.DB, repoID string, cfg Config) *Server {
	if cfg.SquadDir == "" {
		cfg.SquadDir = ".squad"
	}
	if cfg.RepoID == "" {
		cfg.RepoID = repoID
	}
	if cfg.LearningsRoot == "" {
		cfg.LearningsRoot = "."
	}
	if cfg.pingInterval == 0 {
		cfg.pingInterval = 15 * time.Second
	}
	if cfg.lagFlushInterval == 0 {
		// Independent of heartbeat: drops must surface within ~1 second
		// even when the publisher has gone quiet, so a paused-then-
		// resumed dashboard sees the gap immediately.
		cfg.lagFlushInterval = time.Second
	}
	c := chat.New(db, repoID)
	s := &Server{db: db, chat: c, cfg: cfg, startedAt: time.Now()}
	s.pump = newMessagesPump(db, repoID, c.Bus())
	s.pump.start()
	s.claimsPump = newClaimsPump(db, repoID, c.Bus())
	s.claimsPump.start()
	s.agentsPump = newAgentsPump(db, repoID, c.Bus())
	s.agentsPump.start()
	s.activityPump = newActivityPump(db, repoID, c.Bus())
	s.activityPump.start()
	return s
}

// Close stops background goroutines. Call from the same lifecycle that closes
// the http.Server. Safe to call multiple times.
func (s *Server) Close() {
	if s.pump != nil {
		s.pump.stop()
	}
	if s.claimsPump != nil {
		s.claimsPump.stop()
	}
	if s.agentsPump != nil {
		s.agentsPump.stop()
	}
	if s.activityPump != nil {
		s.activityPump.stop()
	}
}

func (s *Server) Bus() *chat.Bus { return s.chat.Bus() }

func (s *Server) WithCallerAgent(id string) *Server {
	s.callerAgent = id
	return s
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/health", s.handleHealth)
	mux.HandleFunc("GET /api/version", s.handleVersion)
	mux.HandleFunc("POST /api/_internal/restart", s.handleInternalRestart)
	mux.HandleFunc("GET /api/items", s.handleItemsList)
	mux.HandleFunc("POST /api/items", s.handleItemsCreate)
	mux.HandleFunc("GET /api/inbox", s.handleInbox)
	mux.HandleFunc("GET /api/items/{id}", s.handleItemDetail)
	mux.HandleFunc("GET /api/items/{id}/attestations", s.handleAttestationsForItem)
	mux.HandleFunc("GET /api/items/{id}/activity", s.handleItemActivity)
	mux.HandleFunc("GET /api/items/{id}/links", s.handleItemLinks)
	mux.HandleFunc("POST /api/items/{id}/accept", s.handleItemsAccept)
	mux.HandleFunc("POST /api/items/{id}/auto-refine", s.handleItemsAutoRefine)
	mux.HandleFunc("POST /api/items/{id}/reject", s.handleItemsReject)
	mux.HandleFunc("POST /api/items/{id}/claim", s.handleItemClaim)
	mux.HandleFunc("POST /api/items/{id}/assign", s.handleItemAssign)
	mux.HandleFunc("POST /api/items/{id}/release", s.handleItemRelease)
	mux.HandleFunc("POST /api/items/{id}/done", s.handleItemDone)
	mux.HandleFunc("POST /api/items/{id}/blocked", s.handleItemBlocked)
	mux.HandleFunc("POST /api/items/{id}/handoff", s.handleItemHandoff)
	mux.HandleFunc("POST /api/items/{id}/touch", s.handleItemTouch)
	mux.HandleFunc("POST /api/items/{id}/force-release", s.handleItemForceRelease)
	mux.HandleFunc("GET /api/specs", s.handleSpecsList)
	mux.HandleFunc("GET /api/specs/{name}", s.handleSpecDetail)
	mux.HandleFunc("GET /api/epics", s.handleEpicsList)
	mux.HandleFunc("GET /api/epics/{name}", s.handleEpicDetail)
	mux.HandleFunc("GET /api/learnings", s.handleLearningsList)
	mux.HandleFunc("GET /api/learnings/{slug}", s.handleLearningDetail)
	mux.HandleFunc("GET /api/agents", s.handleAgents)
	mux.HandleFunc("GET /api/agents/{id}/events", s.handleAgentEvents)
	mux.HandleFunc("GET /api/agents/{id}/timeline", s.handleAgentTimeline)
	mux.HandleFunc("GET /api/agents/{id}/diff", s.handleAgentDiff)
	mux.HandleFunc("GET /api/whoami", s.handleWhoami)
	mux.HandleFunc("GET /api/claims", s.handleClaims)
	mux.HandleFunc("GET /api/messages", s.handleMessagesList)
	mux.HandleFunc("POST /api/messages", s.handleMessagesPost)
	mux.HandleFunc("GET /api/search", s.handleSearch)
	mux.HandleFunc("GET /api/repos", s.handleRepos)
	mux.HandleFunc("GET /api/stats", s.handleStats)
	mux.HandleFunc("GET /api/workspace/status", s.handleWorkspaceStatus)
	mux.HandleFunc("GET /api/events", s.handleEvents)
	mux.Handle("GET /metrics", s.prometheusHandler())
	mux.Handle("/", staticHandler())
	return mux
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

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]any{"error": msg})
}
