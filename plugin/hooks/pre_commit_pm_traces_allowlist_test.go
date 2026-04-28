package hooks

import (
	"os"
	"strings"
	"testing"
)

// TestPMTraces_AllowsChoreSquadPrefix pins the allowlist carve-out for
// squad's own bookkeeping commits. The squad CLI emits `chore(squad):
// close FEAT-NNN` and similar subjects when state-machine transitions
// touch ledger files; an agent that hand-types the same prefix
// (because they're updating an item file or fixing a squad-itself
// ledger inconsistency) must be able to land that commit without the
// gate refusing on grounds of the very ID-pattern squad's own commits
// rely on.
func TestPMTraces_AllowsChoreSquadPrefix(t *testing.T) {
	p := writeFixtureScript(t, "pre_commit_pm_traces.sh")
	repo := setupGitRepo(t, "")
	out, err := runHookInDirWithStdin(t, p, repo,
		`{"tool_name":"Bash","tool_input":{"command":"git commit -m 'chore(squad): close FEAT-100'"}}`,
		append(os.Environ(), "PATH=/usr/bin:/bin"))
	if err != nil {
		t.Fatalf("chore(squad): subject must be allowlisted; got error %v: %s", err, out)
	}
}

// TestPMTraces_AllowsIDInBodyOnly pins that PM-traces in the *body* of
// a multi-line commit message do NOT trigger the gate — only the
// subject (first line) is scanned for IDs. Body-level mentions are
// often legitimate context.
func TestPMTraces_AllowsIDInBodyOnly(t *testing.T) {
	p := writeFixtureScript(t, "pre_commit_pm_traces.sh")
	repo := setupGitRepo(t, "")
	out, err := runHookInDirWithStdin(t, p, repo,
		`{"tool_name":"Bash","tool_input":{"command":"git commit -m 'fix: clean subject' -m 'body referencing FEAT-100 here'"}}`,
		append(os.Environ(), "PATH=/usr/bin:/bin"))
	if err != nil {
		t.Fatalf("body-only ID must not trip the gate (subject-only scan); got error %v: %s", err, out)
	}
}

// TestPMTraces_FailsOnIDInSubjectEvenWithMultipleM pins the converse:
// when the *subject* (first -m argument) carries the ID, the gate
// fires regardless of body content.
func TestPMTraces_FailsOnIDInSubjectEvenWithMultipleM(t *testing.T) {
	p := writeFixtureScript(t, "pre_commit_pm_traces.sh")
	repo := setupGitRepo(t, "")
	out, err := runHookInDirWithStdin(t, p, repo,
		`{"tool_name":"Bash","tool_input":{"command":"git commit -m 'fix: address BUG-200' -m 'clean body'"}}`,
		append(os.Environ(), "PATH=/usr/bin:/bin"))
	if err == nil {
		t.Fatalf("subject-level BUG-200 must trip the gate; got success: %s", out)
	}
	if !strings.Contains(string(out), "BUG-200") {
		t.Errorf("expected stderr to mention BUG-200; got %q", out)
	}
}
