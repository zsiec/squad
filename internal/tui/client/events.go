package client

import "encoding/json"

// Event is the generic envelope for an SSE message. View modules pattern-match
// on Kind and decode Payload via the typed structs below as needed.
type Event struct {
	Kind    string
	Payload json.RawMessage
}

// MessagePayload mirrors what internal/chat.Post and the server pump publish
// for chat writes. Server-side pump.go also includes "ts"; the chat.Post path
// omits it. Both fields are tagged omitempty so either shape decodes cleanly.
type MessagePayload struct {
	ID      int64  `json:"id"`
	TS      int64  `json:"ts,omitempty"`
	AgentID string `json:"agent_id"`
	Thread  string `json:"thread"`
	Kind    string `json:"kind"`
	Body    string `json:"body"`
}

// HandoffPayload mirrors what internal/chat.PostHandoff publishes alongside
// the underlying message row.
type HandoffPayload struct {
	AgentID string `json:"agent_id"`
	Summary string `json:"summary"`
}

// ProgressPayload mirrors what internal/chat.ReportProgress publishes.
type ProgressPayload struct {
	ItemID  string `json:"item_id"`
	AgentID string `json:"agent_id"`
	Pct     int    `json:"pct"`
	Note    string `json:"note"`
}

// AttestationRecordedPayload mirrors what internal/attest.Insert publishes
// after a successful insert when a bus is wired.
type AttestationRecordedPayload struct {
	ItemID    string `json:"item_id"`
	Kind      string `json:"kind"`
	ID        int64  `json:"id"`
	CreatedAt int64  `json:"created_at"`
}

// LearningStateChangedPayload mirrors what internal/learning.Promote publishes
// when a non-nil bus is passed.
type LearningStateChangedPayload struct {
	Slug      string `json:"slug"`
	Kind      string `json:"kind"`
	FromState string `json:"from_state"`
	ToState   string `json:"to_state"`
	Path      string `json:"path"`
}

// LagPayload mirrors the chat.Bus drop sentinel surfaced by the SSE pump.
type LagPayload struct {
	Dropped int64 `json:"dropped"`
}
