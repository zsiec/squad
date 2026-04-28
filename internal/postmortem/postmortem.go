// Package postmortem owns the artifact-presence detector that decides
// whether a closed-without-done claim warrants a follow-up subagent.
// The detector is deterministic, read-only on the ledger and the
// repo, and configurable via .squad/config.yaml's `postmortem:` block.
package postmortem

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const (
	DefaultMinChatMessages = 2
	DefaultMinChatChars    = 50
)

// Decision is the detector's output. Dispatch true means "no durable
// learning artifacts exist for this claim window — fire the postmortem
// subagent." Dispatch false means at least one suppression signal was
// found; Reason names which one. Signals carries the full list for
// callers that want to log every signal, not just the first.
type Decision struct {
	Dispatch bool
	Reason   string
	Signals  []Signal
}

type Signal struct {
	Kind   string
	Detail string
}

const (
	SignalDisabled         = "disabled"
	SignalItemFileEdit     = "item_file_edit"
	SignalLearningArtifact = "learning_artifact"
	SignalSubstantiveChat  = "substantive_chat"
	SignalAlreadyRan       = "postmortem_already_exists"
)

// Opts parameterises Detect. The window is [ClaimedAt, ReleasedAt]
// inclusive on both ends. AgentID names the claimant (used in the
// dispatched-prompt body and the signal Detail strings); chat
// counting and learning-artifact detection both ignore authorship —
// any durable record on the thread or under .squad/learnings/
// suppresses dispatch, regardless of who wrote it.
type Opts struct {
	DB              *sql.DB
	RepoID          string
	ItemID          string
	AgentID         string
	ClaimedAt       int64
	ReleasedAt      int64
	RepoRoot        string
	ItemPath        string
	Enabled         bool
	MinChatMessages int
	MinChatChars    int
}

// Detect returns the dispatch decision. It does not write anything.
func Detect(ctx context.Context, opts Opts) (Decision, error) {
	if !opts.Enabled {
		return Decision{
			Dispatch: false,
			Reason:   "postmortem.enabled is false",
			Signals:  []Signal{{Kind: SignalDisabled, Detail: "config short-circuit"}},
		}, nil
	}

	minMsgs := opts.MinChatMessages
	if minMsgs <= 0 {
		minMsgs = DefaultMinChatMessages
	}
	minChars := opts.MinChatChars
	if minChars <= 0 {
		minChars = DefaultMinChatChars
	}

	var signals []Signal

	if opts.RepoRoot != "" && opts.ItemID != "" {
		existing, err := existingPostmortemForItem(opts.RepoRoot, opts.ItemID)
		if err != nil {
			return Decision{}, fmt.Errorf("scan existing postmortems: %w", err)
		}
		if existing != "" {
			return Decision{
				Dispatch: false,
				Reason:   "postmortem already filed for this item",
				Signals: []Signal{{
					Kind:   SignalAlreadyRan,
					Detail: fmt.Sprintf("found %s — rerun would duplicate", filepath.Base(existing)),
				}},
			}, nil
		}
	}

	if opts.ItemPath != "" && opts.RepoRoot != "" {
		edited, hash, err := itemFileEditedInWindow(opts.RepoRoot, opts.ItemPath, opts.ClaimedAt, opts.ReleasedAt)
		if err != nil {
			return Decision{}, fmt.Errorf("scan item-file git history: %w", err)
		}
		if edited {
			signals = append(signals, Signal{
				Kind:   SignalItemFileEdit,
				Detail: fmt.Sprintf("commit %s touched %s in window", hash, filepath.Base(opts.ItemPath)),
			})
		}
	}

	if opts.RepoRoot != "" {
		artifacts, err := learningArtifactsInWindow(opts.RepoRoot, opts.ClaimedAt, opts.ReleasedAt)
		if err != nil {
			return Decision{}, fmt.Errorf("scan learnings dir: %w", err)
		}
		for _, a := range artifacts {
			signals = append(signals, Signal{
				Kind:   SignalLearningArtifact,
				Detail: fmt.Sprintf("learning %s created in window", filepath.Base(a)),
			})
		}
	}

	if opts.DB != nil {
		count, err := substantiveChatCount(ctx, opts.DB, opts.RepoID, opts.ItemID, opts.ClaimedAt, opts.ReleasedAt, minChars)
		if err != nil {
			return Decision{}, fmt.Errorf("count chat: %w", err)
		}
		if count >= minMsgs {
			signals = append(signals, Signal{
				Kind:   SignalSubstantiveChat,
				Detail: fmt.Sprintf("%d messages on thread %s with body >= %d chars", count, opts.ItemID, minChars),
			})
		}
	}

	if len(signals) > 0 {
		return Decision{
			Dispatch: false,
			Reason:   "durable learning already captured: " + signals[0].Kind,
			Signals:  signals,
		}, nil
	}
	return Decision{
		Dispatch: true,
		Reason:   "no durable artifacts found in claim window",
		Signals:  nil,
	}, nil
}

// itemFileEditedInWindow runs `git log` against the item file path
// scoped to the claim window. Returns true if at least one commit
// touched the file. The hash is the most recent commit (for the
// signal's detail field).
func itemFileEditedInWindow(repoRoot, itemPath string, since, until int64) (bool, string, error) {
	c := exec.Command("git", "log",
		fmt.Sprintf("--since=@%d", since),
		fmt.Sprintf("--until=@%d", until),
		"--format=%H",
		"-n", "1",
		"--", itemPath,
	)
	c.Dir = repoRoot
	out, err := c.Output()
	if err != nil {
		// `git log` exits 0 with empty output when no commits match.
		// Any error here is a real failure (not-a-repo, missing path).
		return false, "", err
	}
	hash := strings.TrimSpace(string(out))
	if hash == "" {
		return false, "", nil
	}
	return true, hash, nil
}

// learningArtifactsInWindow returns every learning markdown file
// added under .squad/learnings/ during [since, until]. We use
// `git log --diff-filter=A` because filesystem mtime is unreliable
// across `git clone` and worktree checkouts (mtime resets to checkout
// time). Untracked files in .squad/learnings/ are also surfaced via
// the porcelain status — those represent in-progress proposals the
// claimant filed but didn't commit yet.
func learningArtifactsInWindow(repoRoot string, since, until int64) ([]string, error) {
	root := filepath.Join(repoRoot, ".squad", "learnings")
	if _, err := os.Stat(root); err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var hits []string

	c := exec.Command("git", "log",
		fmt.Sprintf("--since=@%d", since),
		fmt.Sprintf("--until=@%d", until),
		"--diff-filter=A",
		"--name-only",
		"--format=",
		"--", ".squad/learnings/",
	)
	c.Dir = repoRoot
	out, err := c.Output()
	if err != nil {
		return nil, err
	}
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || filepath.Ext(line) != ".md" {
			continue
		}
		hits = append(hits, filepath.Join(repoRoot, line))
	}

	// Also catch untracked .md files in the learnings tree —
	// claimant may have filed a proposal but not committed it. We
	// can't time-bound these without mtime, so we accept them
	// only if the file mtime falls in the window. mtime is
	// unreliable post-clone but reliable for a file the claimant
	// just wrote in their own session.
	untracked, err := untrackedLearnings(repoRoot)
	if err != nil {
		return nil, err
	}
	for _, p := range untracked {
		info, err := os.Stat(p)
		if err != nil {
			continue
		}
		ts := info.ModTime().Unix()
		if ts >= since && ts <= until {
			hits = append(hits, p)
		}
	}
	return hits, nil
}

// existingPostmortemForItem returns the path of any learning markdown
// file under .squad/learnings/ whose basename starts with
// "<itemID>-postmortem-". Either tracked or untracked counts. Empty
// string means "no prior postmortem found" — the detector continues.
func existingPostmortemForItem(repoRoot, itemID string) (string, error) {
	prefix := itemID + "-postmortem-"
	root := filepath.Join(repoRoot, ".squad", "learnings")
	if _, err := os.Stat(root); err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}
	var found string
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}
		name := info.Name()
		if filepath.Ext(name) == ".md" && strings.HasPrefix(name, prefix) {
			found = path
			return filepath.SkipAll
		}
		return nil
	})
	return found, err
}

func untrackedLearnings(repoRoot string) ([]string, error) {
	c := exec.Command("git", "ls-files", "--others", "--exclude-standard", "--", ".squad/learnings/")
	c.Dir = repoRoot
	out, err := c.Output()
	if err != nil {
		return nil, err
	}
	var paths []string
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || filepath.Ext(line) != ".md" {
			continue
		}
		paths = append(paths, filepath.Join(repoRoot, line))
	}
	return paths, nil
}

// substantiveChatCount returns the number of messages on the item
// thread within the window whose body length (in bytes) is at least
// minChars. Authorship is intentionally ignored: the question is
// whether the durable record exists, not whether the claimant wrote
// it. A peer's analysis posted to the thread captures the lesson
// just as well as the claimant's would.
func substantiveChatCount(ctx context.Context, db *sql.DB, repoID, itemID string, since, until int64, minChars int) (int, error) {
	var n int
	err := db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM messages
		WHERE repo_id = ? AND thread = ?
		  AND ts >= ? AND ts <= ?
		  AND length(body) >= ?
	`, repoID, itemID, since, until, minChars).Scan(&n)
	if err != nil {
		return 0, err
	}
	return n, nil
}
