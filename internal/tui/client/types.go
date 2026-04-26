package client

import "encoding/json"

// --- Items ---

type Item struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	Type      string `json:"type"`
	Priority  string `json:"priority"`
	Area      string `json:"area"`
	Status    string `json:"status"`
	Estimate  string `json:"estimate"`
	Risk      string `json:"risk"`
	ACTotal   int    `json:"ac_total"`
	ACChecked int    `json:"ac_checked"`
	Progress  int    `json:"progress_pct"`
}

type ItemDetail struct {
	Item
	Created      int64    `json:"created"`
	Updated      int64    `json:"updated"`
	BodyMarkdown string   `json:"body_markdown"`
	AC           []ACItem `json:"ac"`
	BlockedBy    []string `json:"blocked_by"`
	RelatesTo    []string `json:"relates_to"`
	References   []string `json:"references"`
	CurrentClaim *Claim   `json:"current_claim"`
}

type ACItem struct {
	Text    string `json:"text"`
	Checked bool   `json:"checked"`
}

type Claim struct {
	ItemID    string `json:"item_id,omitempty"`
	AgentID   string `json:"agent_id"`
	Intent    string `json:"intent"`
	ClaimedAt int64  `json:"claimed_at"`
	LastTouch int64  `json:"last_touch,omitempty"`
}

// --- Agents ---

type Agent struct {
	AgentID     string `json:"agent_id"`
	DisplayName string `json:"display_name"`
	Worktree    string `json:"worktree"`
	LastTickAt  int64  `json:"last_tick_at"`
	Status      string `json:"status"`
}

type Whoami struct {
	AgentID     string `json:"agent_id"`
	DisplayName string `json:"display_name"`
}

// --- Messages ---

type Message struct {
	ID       int64           `json:"id"`
	TS       int64           `json:"ts"`
	AgentID  string          `json:"agent_id"`
	Thread   string          `json:"thread"`
	Kind     string          `json:"kind"`
	Body     string          `json:"body"`
	Mentions json.RawMessage `json:"mentions"`
	Priority string          `json:"priority"`
}

type PostMessageReq struct {
	Thread   string   `json:"thread"`
	Body     string   `json:"body"`
	Mentions []string `json:"mentions,omitempty"`
	Kind     string   `json:"kind,omitempty"` // defaults server-side to "say"
}

// ClaimReq is the body for POST /api/items/{id}/claim.
type ClaimReq struct {
	Intent  string   `json:"intent,omitempty"`
	Long    bool     `json:"long,omitempty"`
	Touches []string `json:"touches,omitempty"`
}

// HandoffReq is the body for POST /api/items/{id}/handoff.
type HandoffReq struct {
	To      string `json:"to"`
	Summary string `json:"summary,omitempty"`
}

// ForceReleaseResp captures the body of POST /api/items/{id}/force-release.
type ForceReleaseResp struct {
	OK          bool   `json:"ok"`
	PriorHolder string `json:"prior_holder,omitempty"`
}

// --- Specs ---

type Spec struct {
	Name  string `json:"name"`
	Title string `json:"title"`
	Path  string `json:"path"`
}

type SpecDetail struct {
	Spec
	Motivation   string   `json:"motivation"`
	Acceptance   []string `json:"acceptance"`
	NonGoals     []string `json:"non_goals"`
	Integration  []string `json:"integration"`
	BodyMarkdown string   `json:"body_markdown"`
}

// --- Epics ---

type Epic struct {
	Name        string `json:"name"`
	Spec        string `json:"spec"`
	Status      string `json:"status"`
	Parallelism string `json:"parallelism"`
	Path        string `json:"path"`
}

type EpicDetail struct {
	Epic
	BodyMarkdown string `json:"body_markdown"`
}

// --- Attestations ---

type Attestation struct {
	ID         int64  `json:"id"`
	Kind       string `json:"kind"`
	Command    string `json:"command"`
	ExitCode   int    `json:"exit_code"`
	OutputHash string `json:"output_hash"`
	OutputPath string `json:"output_path"`
	CreatedAt  int64  `json:"created_at"`
	AgentID    string `json:"agent_id"`
}

// --- Learnings ---

type Learning struct {
	ID        string   `json:"id"`
	Kind      string   `json:"kind"`
	Slug      string   `json:"slug"`
	Title     string   `json:"title"`
	Area      string   `json:"area"`
	State     string   `json:"state"`
	Created   string   `json:"created"`
	CreatedBy string   `json:"created_by"`
	Paths     []string `json:"paths"`
	Related   []string `json:"related_items"`
}

type LearningDetail struct {
	Learning
	Session      string   `json:"session"`
	Evidence     []string `json:"evidence"`
	BodyMarkdown string   `json:"body_markdown"`
	Path         string   `json:"path"`
}

// --- Repos ---

type Repo struct {
	RepoID string `json:"repo_id"`
	Path   string `json:"path"`
	Remote string `json:"remote"`
}

// --- Stats ---

// Stats decodes the top-level envelope eagerly and leaves the deep sub-trees
// as RawMessage. The full snapshot is large and view modules only need
// individual sections at a time; lazy decoding keeps the cost local.
type Stats struct {
	SchemaVersion int             `json:"schema_version"`
	GeneratedAt   int64           `json:"generated_at"`
	RepoID        string          `json:"repo_id"`
	Window        json.RawMessage `json:"window"`
	Items         json.RawMessage `json:"items"`
	Claims        json.RawMessage `json:"claims"`
	Verification  json.RawMessage `json:"verification"`
	Learnings     json.RawMessage `json:"learnings"`
	Tokens        json.RawMessage `json:"tokens"`
	ByAgent       json.RawMessage `json:"by_agent"`
	ByEpic        json.RawMessage `json:"by_epic"`
	Series        json.RawMessage `json:"series"`
}

// --- Workspace ---

// WorkspaceStatus mirrors the {repos, summary} envelope emitted by
// /api/workspace/status. Both halves are RawMessage; downstream code
// decodes the structure it needs.
type WorkspaceStatus struct {
	Repos   json.RawMessage `json:"repos"`
	Summary json.RawMessage `json:"summary"`
}
