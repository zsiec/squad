// Package api carries shapes that travel across the squad HTTP boundary
// and need to round-trip cleanly between the server's response builders
// and the TUI client's decoders. Keeping the wire-shape types here
// prevents server-internal and client-facing structs from drifting out
// of sync when fields are added.
package api

// InboxEntry is the per-row shape returned by GET /api/inbox. Both
// internal/server (encode) and internal/tui/client (decode) reference
// this single declaration so a field added on one side is observable
// to the other at compile time.
type InboxEntry struct {
	ID            string `json:"id"`
	Title         string `json:"title"`
	CapturedBy    string `json:"captured_by,omitempty"`
	CapturedAt    int64  `json:"captured_at,omitempty"`
	ParentSpec    string `json:"parent_spec,omitempty"`
	DoRPass       bool   `json:"dor_pass"`
	Path          string `json:"path"`
	AutoRefinedAt int64  `json:"auto_refined_at,omitempty"`
	AutoRefinedBy string `json:"auto_refined_by,omitempty"`
}
