package identity

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/zsiec/squad/internal/store"
)

func AgentID(worktree string) (string, error) {
	if env := strings.TrimSpace(os.Getenv("SQUAD_AGENT")); env != "" {
		return env, nil
	}
	if persisted := readPersistedAgentID(); persisted != "" {
		return persisted, nil
	}
	if worktree == "" {
		return "", errors.New("cannot infer agent id: SQUAD_AGENT unset, no persisted id, and worktree empty")
	}
	return filepath.Base(worktree), nil
}

// PersistedAgentID exposes the session-keyed persisted id for callers that
// need to compare against an --as override before writing. Returns "" when
// no persisted file exists.
func PersistedAgentID() string {
	return readPersistedAgentID()
}

func readPersistedAgentID() string {
	home, err := store.Home()
	if err != nil {
		return ""
	}
	if sessionPath := sessionAgentIDPath(home); sessionPath != "" {
		if data, err := os.ReadFile(sessionPath); err == nil {
			return strings.TrimSpace(string(data))
		}
	}
	data, err := os.ReadFile(filepath.Join(home, "agent-id.txt"))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

func WritePersistedAgentID(id string) error {
	home, err := store.Home()
	if err != nil {
		return err
	}
	path := sessionAgentIDPath(home)
	if path == "" {
		path = filepath.Join(home, "agent-id.txt")
	}
	return os.WriteFile(path, []byte(id+"\n"), 0o644)
}

func sessionAgentIDPath(home string) string {
	key := sessionKey()
	if key == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(key))
	return filepath.Join(home, "agent-id."+hex.EncodeToString(sum[:6])+".txt")
}

func sessionKey() string {
	for _, env := range []string{
		"SQUAD_SESSION_ID",
		"TERM_SESSION_ID",
		"ITERM_SESSION_ID",
		"TMUX_PANE",
		"STY",
		"WT_SESSION",
	} {
		if v := strings.TrimSpace(os.Getenv(env)); v != "" {
			return v
		}
	}
	return ""
}

// SessionSuffix returns a stable 4-hex-char suffix derived from the session
// key. Same session => same suffix. Two sessions in one worktree => two
// distinct suffixes. Falls back to a hash of pid+worktree when no session
// env signals are present so callers never see an empty value.
func SessionSuffix() string {
	key := sessionKey()
	if key == "" {
		wd, _ := os.Getwd()
		key = fmt.Sprintf("pid:%d:wd:%s", os.Getpid(), wd)
	}
	sum := sha256.Sum256([]byte(key))
	return hex.EncodeToString(sum[:2])
}

// DerivedAgentID returns "agent-XXXX" where XXXX is SessionSuffix(). Honors
// the same precedence as AgentID for SQUAD_AGENT and persisted ids — only
// derives a fresh value when neither is set. The worktree argument is kept
// for symmetry with AgentID; it's used only when both env and persisted
// state are empty.
func DerivedAgentID(worktree string) (string, error) {
	if env := strings.TrimSpace(os.Getenv("SQUAD_AGENT")); env != "" {
		return env, nil
	}
	if persisted := readPersistedAgentID(); persisted != "" {
		return persisted, nil
	}
	return "agent-" + SessionSuffix(), nil
}

func DetectWorktree() string {
	wd, err := os.Getwd()
	if err != nil {
		return ""
	}
	return wd
}
