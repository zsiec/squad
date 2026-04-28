package server

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"path/filepath"
	"strings"
)

// AmbiguousRepoError is returned by resolveItemRepo when an item id
// appears in more than one repo and no ?repo_id= disambiguator was
// supplied. The handler maps it to a 409 with a structured body so the
// SPA can render a "pick one" affordance without parsing prose.
type AmbiguousRepoError struct {
	ItemID     string
	Candidates []string
}

func (e *AmbiguousRepoError) Error() string {
	return fmt.Sprintf("item %s exists in multiple repos %v; pass ?repo_id=<one> to disambiguate",
		e.ItemID, e.Candidates)
}

// writeResolveErr is the canonical failure-path for resolveItemRepo
// callers. AmbiguousRepoError gets a structured body so the SPA can
// render a "pick one" affordance; all other errors flatten to writeErr.
func writeResolveErr(w http.ResponseWriter, statusCode int, err error) {
	var ambiguous *AmbiguousRepoError
	if errors.As(err, &ambiguous) {
		writeJSON(w, http.StatusConflict, map[string]any{
			"error":      "ambiguous",
			"message":    ambiguous.Error(),
			"item_id":    ambiguous.ItemID,
			"candidates": ambiguous.Candidates,
		})
		return
	}
	writeErr(w, statusCode, err.Error())
}

// resolveItemRepo returns the repo identifier and the on-disk .squad
// directory that owns itemID. The lookup branches on cfg.RepoID:
// single-repo mode short-circuits to cfg values; workspace mode joins
// items × repos and uses the optional requestedRepoID (?repo_id=) to
// disambiguate collisions. Returns (0, nil) on success; on failure,
// the int is the HTTP status the caller should write and err carries
// the body. *AmbiguousRepoError signals a 409 collision.
func (s *Server) resolveItemRepo(ctx context.Context, itemID, requestedRepoID string) (repoID, squadDir string, statusCode int, err error) {
	if s.cfg.RepoID != "" {
		dir := s.cfg.SquadDir
		if dir == "" || dir == ".squad" {
			dir = filepath.Join(s.cfg.LearningsRoot, ".squad")
		}
		return s.cfg.RepoID, dir, 0, nil
	}
	q := `SELECT i.repo_id, COALESCE(r.root_path, '')
	      FROM items i LEFT JOIN repos r ON r.id = i.repo_id
	      WHERE i.item_id = ?`
	args := []any{itemID}
	if requestedRepoID != "" {
		q += ` AND i.repo_id = ?`
		args = append(args, requestedRepoID)
	}
	q += ` ORDER BY i.repo_id`
	rows, qerr := s.db.QueryContext(ctx, q, args...)
	if qerr != nil {
		return "", "", http.StatusInternalServerError, qerr
	}
	defer rows.Close()
	var matches []struct{ repo, root string }
	for rows.Next() {
		var rid, root string
		if scanErr := rows.Scan(&rid, &root); scanErr != nil {
			return "", "", http.StatusInternalServerError, scanErr
		}
		matches = append(matches, struct{ repo, root string }{rid, root})
	}
	switch len(matches) {
	case 0:
		return "", "", http.StatusNotFound, errors.New("item not found")
	case 1:
		root := matches[0].root
		if root == "" {
			return "", "", http.StatusInternalServerError,
				fmt.Errorf("repo %q has no root_path; cannot resolve squad dir", matches[0].repo)
		}
		return matches[0].repo, filepath.Join(root, ".squad"), 0, nil
	default:
		repos := make([]string, len(matches))
		for i, m := range matches {
			repos[i] = m.repo
		}
		return "", "", http.StatusConflict, &AmbiguousRepoError{ItemID: itemID, Candidates: repos}
	}
}

// resolveCreateRepo returns the repo + squadDir + repo root for new
// items. Single-repo mode falls back to cfg values. Workspace mode
// requires requestedRepoID (?repo_id= query param) — there is no
// implicit "first repo" because the consequence (a new item created in
// the wrong repo's tree) is recoverable but expensive.
func (s *Server) resolveCreateRepo(ctx context.Context, requestedRepoID string) (repoID, squadDir, repoRoot string, statusCode int, err error) {
	if s.cfg.RepoID != "" {
		root := s.cfg.LearningsRoot
		dir := s.cfg.SquadDir
		if dir == "" || dir == ".squad" {
			dir = filepath.Join(root, ".squad")
		}
		return s.cfg.RepoID, dir, root, 0, nil
	}
	if strings.TrimSpace(requestedRepoID) == "" {
		return "", "", "", http.StatusBadRequest,
			errors.New("workspace mode: ?repo_id= is required when creating items")
	}
	var root string
	scanErr := s.db.QueryRowContext(ctx,
		`SELECT COALESCE(root_path, '') FROM repos WHERE id = ?`,
		requestedRepoID).Scan(&root)
	if errors.Is(scanErr, sql.ErrNoRows) {
		return "", "", "", http.StatusNotFound,
			fmt.Errorf("no repo registered with id %q", requestedRepoID)
	}
	if scanErr != nil {
		return "", "", "", http.StatusInternalServerError, scanErr
	}
	if root == "" {
		return "", "", "", http.StatusInternalServerError,
			fmt.Errorf("repo %q has no root_path; cannot resolve squad dir", requestedRepoID)
	}
	return requestedRepoID, filepath.Join(root, ".squad"), root, 0, nil
}
