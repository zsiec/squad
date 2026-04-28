package server

import (
	"strings"
	"testing"
)

// TestInboxRefineComposerDocumentsCLIAlternative pins the SPA-side doc
// comment that points operators at the peer-reviewer alternative
// (`squad refine` CLI) when the claude-driven Send-for-refinement flow
// is the wrong tool. The two refinement paths serve different
// purposes — claude redraft vs. peer markdown edit — and dual-flow
// ambiguity is a known operator tax. The peer path stays alive via the
// CLI (registered in `cmd/squad/main.go`, documented in
// `plugin/skills/squad-loop/SKILL.md`); this comment is the only
// pointer to it from the SPA. Removing the pointer reverts the
// ambiguity, so a future refactor that drops it must re-decide here.
//
// Same `webFS.ReadFile` structural-pin pattern as
// TestInboxRefineComposerUsesAutoRefineContract.
func TestInboxRefineComposerDocumentsCLIAlternative(t *testing.T) {
	body, err := webFS.ReadFile("web/inbox.js")
	if err != nil {
		t.Fatalf("read embedded web/inbox.js: %v", err)
	}
	src := string(body)

	if !strings.Contains(src, "`squad refine <ID>`") {
		t.Errorf("inbox.js must mention `squad refine <ID>` near openRefineComposer " +
			"so operators can find the peer-reviewer alternative when claude " +
			"redraft is the wrong tool")
	}
}
