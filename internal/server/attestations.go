package server

import (
	"net/http"

	"github.com/zsiec/squad/internal/attest"
)

type attestRow struct {
	ID         int64  `json:"id"`
	Kind       string `json:"kind"`
	Command    string `json:"command"`
	ExitCode   int    `json:"exit_code"`
	OutputHash string `json:"output_hash"`
	OutputPath string `json:"output_path"`
	CreatedAt  int64  `json:"created_at"`
	AgentID    string `json:"agent_id"`
	RepoID     string `json:"repo_id"`
}

func (s *Server) handleAttestationsForItem(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	repoID, _, statusCode, rerr := s.resolveItemRepo(r.Context(), id, r.URL.Query().Get("repo_id"))
	if rerr != nil {
		writeResolveErr(w, statusCode, rerr)
		return
	}
	ledger := attest.New(s.db, repoID, nil)
	recs, err := ledger.ListForItem(r.Context(), id)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	out := make([]attestRow, 0, len(recs))
	for _, rec := range recs {
		out = append(out, attestRow{
			ID: rec.ID, Kind: string(rec.Kind), Command: rec.Command,
			ExitCode: rec.ExitCode, OutputHash: rec.OutputHash, OutputPath: rec.OutputPath,
			CreatedAt: rec.CreatedAt, AgentID: rec.AgentID, RepoID: rec.RepoID,
		})
	}
	writeJSON(w, http.StatusOK, out)
}
