package server

import (
	"errors"
	"net/http"
	"path/filepath"

	"github.com/zsiec/squad/internal/commitlog"
	"github.com/zsiec/squad/internal/items"
	"github.com/zsiec/squad/internal/prmark"
)

type linkPR struct {
	URL    *string `json:"url"`
	Number *int    `json:"number"`
	Branch string  `json:"branch"`
}

type linkCommit struct {
	Sha     string `json:"sha"`
	Subject string `json:"subject"`
	URL     string `json:"url"`
}

type linksResponse struct {
	PR      *linkPR      `json:"pr"`
	Commits []linkCommit `json:"commits"`
}

func (s *Server) handleItemLinks(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	repoID, squadDir, statusCode, rerr := s.resolveItemRepo(r.Context(), id, r.URL.Query().Get("repo_id"))
	if rerr != nil {
		writeResolveErr(w, statusCode, rerr)
		return
	}
	// Single-repo mode resolveItemRepo short-circuits without checking item
	// existence, so verify the item file is on disk before continuing.
	// Workspace mode already validates via the items-table query.
	if _, _, ferr := items.FindByID(squadDir, id); ferr != nil {
		if errors.Is(ferr, items.ErrItemNotFound) {
			writeErr(w, http.StatusNotFound, "no item "+id)
			return
		}
		writeErr(w, http.StatusInternalServerError, ferr.Error())
		return
	}

	resp := linksResponse{Commits: []linkCommit{}}

	var remoteURL string
	if err := s.db.QueryRowContext(r.Context(),
		`SELECT COALESCE(remote_url, '') FROM repos WHERE id = ?`,
		repoID).Scan(&remoteURL); err != nil {
		writeJSON(w, http.StatusOK, resp)
		return
	}
	base := prmark.GitHubBaseURL(remoteURL)
	if base == "" {
		writeJSON(w, http.StatusOK, resp)
		return
	}

	pendingPath := filepath.Join(squadDir, "pending-prs.json")
	pending, _ := prmark.ReadPending(pendingPath)
	for _, e := range pending {
		if e.ItemID == id {
			compare := base + "/compare/" + e.Branch + "?expand=1"
			resp.PR = &linkPR{URL: &compare, Number: nil, Branch: e.Branch}
			break
		}
	}

	commits, err := commitlog.ListForItem(r.Context(), s.db, repoID, id)
	if err != nil {
		writeJSON(w, http.StatusOK, resp)
		return
	}
	for _, c := range commits {
		resp.Commits = append(resp.Commits, linkCommit{
			Sha:     c.Sha,
			Subject: c.Subject,
			URL:     base + "/commit/" + c.Sha,
		})
	}
	writeJSON(w, http.StatusOK, resp)
}
