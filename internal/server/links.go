package server

import (
	"net/http"
	"path/filepath"
	"time"

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
	var item items.Item
	var found bool
	for _, group := range [][]items.Item{walk.Active, walk.Done} {
		for _, it := range group {
			if it.ID == id {
				item, found = it, true
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

	var rootPath, remoteURL string
	if err := s.db.QueryRowContext(r.Context(),
		`SELECT COALESCE(root_path, ''), COALESCE(remote_url, '') FROM repos WHERE id = ?`,
		s.cfg.RepoID).Scan(&rootPath, &remoteURL); err != nil {
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
	var branch string
	for _, e := range pending {
		if e.ItemID == id {
			branch = e.Branch
			break
		}
	}
	if branch != "" {
		compare := base + "/compare/" + branch + "?expand=1"
		resp.PR = &linkPR{URL: &compare, Number: nil, Branch: branch}
	}

	if branch == "" {
		writeJSON(w, http.StatusOK, resp)
		return
	}

	rows, err := s.db.QueryContext(r.Context(),
		`SELECT DISTINCT path FROM touches WHERE repo_id = ? AND item_id = ?`,
		s.cfg.RepoID, id)
	if err != nil {
		writeJSON(w, http.StatusOK, resp)
		return
	}
	var touched []string
	for rows.Next() {
		var p string
		if err := rows.Scan(&p); err == nil {
			touched = append(touched, p)
		}
	}
	rows.Close()

	q := prmark.CommitQuery{
		RepoRoot:     rootPath,
		Branch:       branch,
		TouchedFiles: touched,
		Limit:        20,
	}
	if item.AcceptedAt > 0 {
		q.Since = time.Unix(item.AcceptedAt, 0)
	}
	commits, err := prmark.ResolveCommits(r.Context(), q)
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
