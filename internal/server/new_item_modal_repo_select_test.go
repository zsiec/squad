package server

import (
	"regexp"
	"strings"
	"testing"
)

// TestNewItemModalThreadsRepoIDInWorkspaceMode pins the SPA contract for
// the new-item modal's workspace-mode wiring. Pre-fix, the modal in
// actions.js POSTed to /api/items with no repo_id and no UI affordance
// for picking one — under workspace mode resolveCreateRepo would 400,
// and even when it didn't, users had no way to choose where the file
// lands. The wire contract is structural — same webFS.ReadFile
// approach as TestRepoBadgeCssIsDistinctlyStyled in
// repo_badge_css_test.go — and asserts actions.js
//  1. fetches /api/repos so the modal can learn what repos exist,
//  2. exposes a select named repo_id when more than one repo is present,
//  3. threads ?repo_id=<value> onto POST /api/items when a repo is
//     selected, so resolveCreateRepo routes the file correctly.
//
// Single-repo create stays green in TestItemsCreate_HappyPath
// (items_create_test.go); workspace-mode integration coverage lives in
// workspace_mutation_routes_test.go's TestHandleItemsCreate_WorkspaceMode_*
// family.
func TestNewItemModalThreadsRepoIDInWorkspaceMode(t *testing.T) {
	body, err := webFS.ReadFile("web/actions.js")
	if err != nil {
		t.Fatalf("read embedded web/actions.js: %v", err)
	}
	src := string(body)

	if !strings.Contains(src, "/api/repos") {
		t.Errorf("actions.js: new-item modal must fetch /api/repos to learn the workspace's repo set")
	}

	selectRe := regexp.MustCompile(`name=["']repo_id["']`)
	if !selectRe.MatchString(src) {
		t.Errorf("actions.js: new-item modal must expose a control with name=\"repo_id\" so workspace-mode users can pick a target repo")
	}

	// The POST in the new-item flow is built around `/api/items`. Find the
	// submit handler and assert it threads `?repo_id=` somewhere within
	// reach of that endpoint string. The check tolerates conditional
	// concatenation (e.g. `'/api/items' + (repoID ? '?repo_id=' + … : '')`)
	// without binding to one exact spelling.
	postIdx := strings.Index(src, "'/api/items'")
	if postIdx < 0 {
		postIdx = strings.Index(src, `"/api/items"`)
	}
	if postIdx < 0 {
		t.Fatalf("actions.js: POST endpoint /api/items not found; new-item modal contract is broken")
	}
	window := src[postIdx:]
	if len(window) > 600 {
		window = window[:600]
	}
	if !strings.Contains(window, "?repo_id=") {
		t.Errorf("actions.js: new-item modal must thread ?repo_id=<selected> onto POST /api/items in workspace mode; nearby source:\n%s", window)
	}
}
