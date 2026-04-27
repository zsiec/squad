package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/zsiec/squad/internal/items"
)

// AutoRefineTimeout bounds the spawned `claude -p` subprocess. Set as a
// var so tests can shorten it; production callers do not change it.
var AutoRefineTimeout = 90 * time.Second

// AutoRefineNarrowTools is the closed set of MCP tools the auto-refine
// subprocess is allowed to call. The squad mcp subprocess reads this list
// from $SQUAD_MCP_TOOLS and self-restricts.
var AutoRefineNarrowTools = []string{
	"squad_get_item",
	"squad_inbox",
	"squad_history",
	"squad_auto_refine_apply",
}

// errClaudeNotFound is returned by the default runner when `claude` is
// not on PATH; the handler maps it to 503.
var errClaudeNotFound = errors.New("claude CLI not found on PATH")

// autoRefineRunResult captures the subprocess outcome for the handler.
type autoRefineRunResult struct {
	Stderr  []byte
	Err     error
	TimedOut bool
}

// autoRefineRunner is the seam tests inject to bypass the real subprocess.
type autoRefineRunner func(ctx context.Context, prompt, mcpConfigPath string) autoRefineRunResult

// autoRefineDefaultRunner spawns `claude -p` with the narrow MCP config and
// a process group so timeout-kill takes any spawned helpers with it.
var autoRefineDefaultRunner autoRefineRunner = func(ctx context.Context, prompt, mcpConfigPath string) autoRefineRunResult {
	if _, err := exec.LookPath("claude"); err != nil {
		return autoRefineRunResult{Err: errClaudeNotFound}
	}
	cmd := exec.CommandContext(ctx, "claude", "-p", prompt, "--mcp-config", mcpConfigPath)
	autoRefineSetProcessGroup(cmd)
	// Output() captures stderr into *exec.ExitError on non-zero exit, which
	// is the path we care about; stdout is incidental and discarded.
	if _, err := cmd.Output(); err != nil {
		res := autoRefineRunResult{Err: err}
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			res.Stderr = exitErr.Stderr
		}
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			res.TimedOut = true
		}
		return res
	}
	return autoRefineRunResult{}
}

// SetAutoRefineRunner overrides the subprocess runner for this server
// (used by tests).
func (s *Server) SetAutoRefineRunner(r autoRefineRunner) {
	s.autoRefineMu.Lock()
	defer s.autoRefineMu.Unlock()
	s.autoRefineRunner = r
}

func (s *Server) handleItemsAutoRefine(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeErr(w, http.StatusBadRequest, "id required")
		return
	}

	if !s.acquireAutoRefineSlot(id) {
		writeErr(w, http.StatusConflict, fmt.Sprintf("auto-refine already in flight for %s", id))
		return
	}
	defer s.releaseAutoRefineSlot(id)

	path, _, err := items.FindByID(s.cfg.SquadDir, id)
	if err != nil {
		writeErr(w, http.StatusNotFound, "item not found")
		return
	}
	before, err := items.Parse(path)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	if before.Status != "captured" {
		writeErr(w, http.StatusConflict,
			fmt.Sprintf("auto-refine only applies to captured items; current status: %s", before.Status))
		return
	}

	cfgPath, cleanup, err := writeAutoRefineNarrowMCPConfig()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "build mcp config: "+err.Error())
		return
	}
	defer cleanup()

	prompt := autoRefinePromptFor(id)
	ctx, cancel := context.WithTimeout(r.Context(), AutoRefineTimeout)
	defer cancel()

	runner := s.autoRefineRunnerOrDefault()
	res := runner(ctx, prompt, cfgPath)

	if res.TimedOut {
		writeErr(w, http.StatusGatewayTimeout,
			fmt.Sprintf("auto-refine timed out after %s", AutoRefineTimeout))
		return
	}
	if errors.Is(res.Err, errClaudeNotFound) {
		writeErr(w, http.StatusServiceUnavailable, "claude CLI not found on PATH")
		return
	}
	if res.Err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]any{
			"error":  res.Err.Error(),
			"stderr": string(autoRefineTruncate(res.Stderr, 512)),
		})
		return
	}

	after, err := items.Parse(path)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	if after.AutoRefinedAt <= before.AutoRefinedAt {
		writeErr(w, http.StatusInternalServerError, "claude exited without drafting; run again")
		return
	}

	s.publishInboxChanged(id, "auto-refine")
	writeJSON(w, http.StatusOK, autoRefineEntry(after))
}

// autoRefineEntry shapes the response so the SPA inbox can re-render the
// row directly from the payload (no extra fetch). DoRPass is true by
// construction — items.AutoRefineApply already validated the new body.
// `body_markdown` is included so the row composer can show the new body
// without an item-detail round-trip; the field is non-standard for the
// usual InboxEntry shape but is harmless additional context.
func autoRefineEntry(it items.Item) map[string]any {
	return map[string]any{
		"id":              it.ID,
		"title":           it.Title,
		"captured_by":     it.CapturedBy,
		"captured_at":     it.CapturedAt,
		"parent_spec":     it.ParentSpec,
		"dor_pass":        true,
		"path":            it.Path,
		"auto_refined_at": it.AutoRefinedAt,
		"auto_refined_by": it.AutoRefinedBy,
		"body_markdown":   it.Body,
	}
}


func (s *Server) autoRefineRunnerOrDefault() autoRefineRunner {
	s.autoRefineMu.Lock()
	defer s.autoRefineMu.Unlock()
	if s.autoRefineRunner != nil {
		return s.autoRefineRunner
	}
	return autoRefineDefaultRunner
}

func (s *Server) acquireAutoRefineSlot(id string) bool {
	s.autoRefineMu.Lock()
	defer s.autoRefineMu.Unlock()
	if s.autoRefineInFlight == nil {
		s.autoRefineInFlight = map[string]struct{}{}
	}
	if _, ok := s.autoRefineInFlight[id]; ok {
		return false
	}
	s.autoRefineInFlight[id] = struct{}{}
	return true
}

func (s *Server) releaseAutoRefineSlot(id string) {
	s.autoRefineMu.Lock()
	defer s.autoRefineMu.Unlock()
	delete(s.autoRefineInFlight, id)
}

func writeAutoRefineNarrowMCPConfig() (path string, cleanup func(), err error) {
	binary, err := os.Executable()
	if err != nil {
		return "", func() {}, err
	}
	cfg := map[string]any{
		"mcpServers": map[string]any{
			"squad": map[string]any{
				"command": binary,
				"args":    []string{"mcp"},
				"env": map[string]string{
					"SQUAD_MCP_TOOLS": strings.Join(AutoRefineNarrowTools, ","),
				},
			},
		},
	}
	body, err := json.Marshal(cfg)
	if err != nil {
		return "", func() {}, err
	}
	f, err := os.CreateTemp(filepath.Join(os.TempDir()), "squad-auto-refine-*.json")
	if err != nil {
		return "", func() {}, err
	}
	if _, err := f.Write(body); err != nil {
		_ = f.Close()
		_ = os.Remove(f.Name())
		return "", func() {}, err
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(f.Name())
		return "", func() {}, err
	}
	return f.Name(), func() { _ = os.Remove(f.Name()) }, nil
}

func autoRefineTruncate(b []byte, n int) []byte {
	if len(b) <= n {
		return b
	}
	return b[:n]
}

func autoRefinePromptFor(itemID string) string {
	return fmt.Sprintf(`You are auto-refining a captured squad item.

Read the item with squad_get_item(item_id="%s"). Inspect related items via squad_inbox or squad_history if useful for context.

Replace the item's body with a fresh Problem / Context / Acceptance criteria block that satisfies squad's Definition of Ready (no template-not-placeholder violations, no vague-acceptance-bullet violations: each AC bullet is at least six words and contains a verb).

Call squad_auto_refine_apply(item_id="%s", new_body=...) exactly once with the drafted body and then stop. Do not call any other write tools.`, itemID, itemID)
}

