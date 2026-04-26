package server

import (
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

	walk, err := items.Walk(s.cfg.SquadDir)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	found := false
	for _, group := range [][]items.Item{walk.Active, walk.Done} {
		for _, it := range group {
			if it.ID == id {
				found = true
				break
			}
		}
		if found {
			break
		}
	}
	if !found {
		writeErr(w, http.StatusNotFound, "no item "+id)
		return
	}

	resp := linksResponse{Commits: []linkCommit{}}

	var remoteURL string
	if err := s.db.QueryRowContext(r.Context(),
		`SELECT COALESCE(remote_url, '') FROM repos WHERE id = ?`,
		s.cfg.RepoID).Scan(&remoteURL); err != nil {
		writeJSON(w, http.StatusOK, resp)
		return
	}
	base := prmark.GitHubBaseURL(remoteURL)
	if base == "" {
		writeJSON(w, http.StatusOK, resp)
		return
	}

	pendingPath := filepath.Join(s.cfg.SquadDir, "pending-prs.json")
	pending, _ := prmark.ReadPending(pendingPath)
	for _, e := range pending {
		if e.ItemID == id {
			compare := base + "/compare/" + e.Branch + "?expand=1"
			resp.PR = &linkPR{URL: &compare, Number: nil, Branch: e.Branch}
			break
		}
	}

	commits, err := commitlog.ListForItem(r.Context(), s.db, s.cfg.RepoID, id)
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
