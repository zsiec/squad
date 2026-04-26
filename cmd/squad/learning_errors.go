package main

import (
	"errors"

	"github.com/zsiec/squad/internal/learning"
)

// Sentinel errors shared by the learning approve/reject pure functions.
// ErrLearningNotFound and ErrAmbiguousSlug alias the internal/learning
// sentinels so callers can errors.Is against either name; ResolveSingle
// is the single source of truth for the wrapped value.
var (
	ErrLearningNotFound = learning.ErrNotFound
	ErrAmbiguousSlug    = learning.ErrAmbiguous
	ErrNotProposed      = errors.New("learning is not in proposed state")
	ErrReasonRequired   = errors.New("--reason is required")
)
