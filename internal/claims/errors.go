package claims

import "errors"

var (
	ErrClaimTaken     = errors.New("claims: item already claimed")
	ErrNotYours       = errors.New("claims: claim held by another agent")
	ErrNotClaimed     = errors.New("claims: no active claim on item")
	ErrReasonRequired = errors.New("claims: --reason is required for force-release")
	ErrBlockedByOpen  = errors.New("claims: item has unresolved blockers")
)
