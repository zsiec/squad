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
	{"pre-commit-tick", "pre_commit_tick.sh", false, "Block git commit unless squad tick ran in the last 5 minutes.", "Surfaces peer chat before sealing work; can interrupt fast solo loops.", "PreToolUse", "Bash"},
	{"pre-commit-pm-traces", "pre_commit_pm_traces.sh", false, "Block git commit if backlog IDs leak into staged diff or commit message.", "Catches PM noise pre-commit; harmless if you follow the no-PM-traces rule.", "PreToolUse", "Bash"},
	{"pre-edit-touch-check", "pre_edit_touch_check.sh", false, "Warn (do not block) if another agent is touching the same file.", "Useful in multi-agent setups; pure noise solo.", "PreToolUse", "Edit|Write"},
	{"stop-handoff", "stop_handoff.sh", false, "Auto-handoff your open claim if the session ends with no recent activity.", "Prevents silent drops; can fire when you stop briefly to think.", "Stop", "*"},
}
