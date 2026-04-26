package main

import "errors"

// Sentinel errors shared by the learning approve/reject pure functions.
// The cobra wrappers preserve pre-lift user-facing messages by composing
// new error strings; MCP callers can errors.Is to discriminate.
var (
	ErrLearningNotFound = errors.New("learning not found")
	ErrAmbiguousSlug    = errors.New("learning slug ambiguous")
	ErrNotProposed      = errors.New("learning is not in proposed state")
	ErrReasonRequired   = errors.New("--reason is required")
)
