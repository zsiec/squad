package stats

// scopeSQL returns the WHERE-clause fragment that pins a query to a single
// repo when repoID is set, or "1=1" (always true) when repoID is empty —
// the workspace-mode sentinel: a daemon launched outside any repo
// aggregates rows across every repo in the global DB. qualifier is an
// optional table-alias prefix like "ch." for joined queries.
func scopeSQL(qualifier, repoID string) string {
	if repoID == "" {
		return "1=1"
	}
	return qualifier + "repo_id = ?"
}

// scopeArgs returns the bind args that match scopeSQL for the same repoID.
// Empty repoID → no bind; non-empty → []any{repoID}. Designed to be
// prepended to the query's arg list.
func scopeArgs(repoID string) []any {
	if repoID == "" {
		return nil
	}
	return []any{repoID}
}
