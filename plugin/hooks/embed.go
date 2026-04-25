package hooks

import "embed"

//go:embed *.sh
var FS embed.FS

type Hook struct {
	Name        string
	Filename    string
	DefaultOn   bool
	Description string
	TradeOff    string
	EventType   string
	Matcher     string
}

var All = []Hook{
	{"session-start", "session_start.sh", true, "Auto-register and tick at session start; inject a one-line context block.", "Adds ~150ms to session startup. Always recommended.", "SessionStart", "*"},
	{"user-prompt-tick", "user_prompt_submit.sh", true, "Auto-tick before Claude reads each prompt; inject pending mentions/knocks/handoffs as context.", "Adds ~50ms per turn; the tradeoff is Claude sees peer chat without the user having to type 'squad tick'. Recommended.", "UserPromptSubmit", "*"},
	{"pre-compact", "pre_compact.sh", true, "Inject the agent's current claim + recent chat as additionalContext before context compaction so identity survives.", "Adds ~100ms when Claude compacts. The tradeoff is the post-compact context still knows what claim it owns. Recommended.", "PreCompact", "*"},
	{"stop-listen", "stop_listen.sh", true, "Stop hook: long-block on a localhost TCP listener; wake on any peer say/ask/fyi and inject the inbox.", "Adds a per-session listener bound to 127.0.0.1:0; install probe disables it on hosts that deny loopback bind.", "Stop", "*"},
	{"post-tool-flush", "post_tool_flush.sh", true, "Post-tool mailbox flush: deliver peer chat as additionalContext between tool calls without waiting for Stop.", "Adds ~5ms per tool call (no-op when inbox empty); the tradeoff is sub-second mid-turn delivery.", "PostToolUse", "*"},
	{"session-end-cleanup", "session_end_cleanup.sh", true, "SessionEnd hook: drop this session's notify_endpoints rows so peer senders stop dialing a dead port.", "Necessary companion to stop-listen; harmless on its own.", "SessionEnd", "*"},
	{"async-rewake", "async_rewake.sh", false, "asyncRewake background hook: wake an idle session from outside the IDE within seconds.", "Opt-in until asyncRewake's contract stabilizes; spawns one background process per session.", "asyncRewake", "*"},
	{"pre-commit-pm-traces", "pre_commit_pm_traces.sh", false, "Block git commit if backlog IDs leak into staged diff or commit message.", "Catches PM noise pre-commit; harmless if you follow the no-PM-traces rule.", "PreToolUse", "Bash"},
	{"pre-edit-touch-check", "pre_edit_touch_check.sh", false, "Warn (do not block) if another agent is touching the same file.", "Useful in multi-agent setups; pure noise solo.", "PreToolUse", "Edit|Write"},
	{"stop-handoff", "stop_handoff.sh", false, "Stop hook variant: auto-handoff a stale claim if the session ends with no recent activity. Mutually exclusive with stop-listen — only enable one.", "Prevents silent drops; redundant once stop-listen is on.", "Stop", "*"},
}
