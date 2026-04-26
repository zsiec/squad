package main

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

const readyItemPass = `---
id: FEAT-001
title: Wire the ready check verb against the captured backlog
type: feature
priority: P1
area: auth
status: captured
created: 2026-04-26
updated: 2026-04-26
---

## Acceptance criteria
- [ ] does the thing
`

const readyItemFail = `---
id: FEAT-002
title: tiny
type: feature
priority: P1
area: <fill-in>
status: captured
created: 2026-04-26
updated: 2026-04-26
---

## Acceptance criteria
no checkboxes here
`

const readyItemPassB = `---
id: FEAT-003
title: Second passing item to verify multi-row ready listings
type: feature
priority: P1
area: auth
status: captured
created: 2026-04-26
updated: 2026-04-26
---

## Acceptance criteria
- [ ] another acceptance criterion
`

func TestRunReadyCheck_NoIDsNoCapturedItemsIsFriendly(t *testing.T) {
	setupAcceptRepo(t, "test-ready-empty")

	var stdout, stderr bytes.Buffer
	code := runReadyCheck(context.Background(), nil, false, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit=%d want 0\nstdout=%s\nstderr=%s", code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stdout.String(), "no captured items to check") {
		t.Fatalf("stdout missing 'no captured items to check': %s", stdout.String())
	}
}

func TestRunReadyCheck_NoIDsListsAllCapturedMixedStatuses(t *testing.T) {
	repoDir := setupAcceptRepo(t, "test-ready-list-all")
	writeItemFile(t, repoDir, "FEAT-001-pass.md", readyItemPass)
	writeItemFile(t, repoDir, "FEAT-002-fail.md", readyItemFail)
	persistAcceptFixture(t, repoDir, "FEAT-001-pass.md")
	persistAcceptFixture(t, repoDir, "FEAT-002-fail.md")

	var stdout, stderr bytes.Buffer
	code := runReadyCheck(context.Background(), nil, false, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit=%d want 0 (default not strict)\nstdout=%s\nstderr=%s",
			code, stdout.String(), stderr.String())
	}
	out := stdout.String()
	for _, want := range []string{"FEAT-001", "FEAT-002", "PASS", "FAIL", "ID", "STATUS", "VIOLATIONS"} {
		if !strings.Contains(out, want) {
			t.Fatalf("stdout missing %q:\n%s", want, out)
		}
	}
}

func TestRunReadyCheck_SpecificPassingIDExitsZero(t *testing.T) {
	repoDir := setupAcceptRepo(t, "test-ready-pass")
	writeItemFile(t, repoDir, "FEAT-001-pass.md", readyItemPass)
	persistAcceptFixture(t, repoDir, "FEAT-001-pass.md")

	var stdout, stderr bytes.Buffer
	code := runReadyCheck(context.Background(), []string{"FEAT-001"}, false, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit=%d want 0\nstdout=%s\nstderr=%s", code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stdout.String(), "PASS") {
		t.Fatalf("stdout missing PASS: %s", stdout.String())
	}
	if strings.Contains(stdout.String(), "FAIL") {
		t.Fatalf("stdout should not contain FAIL: %s", stdout.String())
	}
}

func TestRunReadyCheck_SpecificFailingIDExitsZeroByDefault(t *testing.T) {
	repoDir := setupAcceptRepo(t, "test-ready-fail-default")
	writeItemFile(t, repoDir, "FEAT-002-fail.md", readyItemFail)
	persistAcceptFixture(t, repoDir, "FEAT-002-fail.md")

	var stdout, stderr bytes.Buffer
	code := runReadyCheck(context.Background(), []string{"FEAT-002"}, false, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit=%d want 0 (default lint, not enforcement)\nstdout=%s\nstderr=%s",
			code, stdout.String(), stderr.String())
	}
	out := stdout.String()
	if !strings.Contains(out, "FAIL") {
		t.Fatalf("stdout missing FAIL: %s", out)
	}
	if !strings.Contains(out, "area") {
		t.Fatalf("stdout missing 'area' violation message: %s", out)
	}
}

func TestRunReadyCheck_StrictWithFailureExitsOne(t *testing.T) {
	repoDir := setupAcceptRepo(t, "test-ready-strict-fail")
	writeItemFile(t, repoDir, "FEAT-001-pass.md", readyItemPass)
	writeItemFile(t, repoDir, "FEAT-002-fail.md", readyItemFail)
	persistAcceptFixture(t, repoDir, "FEAT-001-pass.md")
	persistAcceptFixture(t, repoDir, "FEAT-002-fail.md")

	var stdout, stderr bytes.Buffer
	code := runReadyCheck(context.Background(), nil, true, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("exit=%d want 1 (strict + violations)\nstdout=%s\nstderr=%s",
			code, stdout.String(), stderr.String())
	}
}

func TestRunReadyCheck_StrictAllPassingExitsZero(t *testing.T) {
	repoDir := setupAcceptRepo(t, "test-ready-strict-pass")
	writeItemFile(t, repoDir, "FEAT-001-pass.md", readyItemPass)
	writeItemFile(t, repoDir, "FEAT-003-passb.md", readyItemPassB)
	persistAcceptFixture(t, repoDir, "FEAT-001-pass.md")
	persistAcceptFixture(t, repoDir, "FEAT-003-passb.md")

	var stdout, stderr bytes.Buffer
	code := runReadyCheck(context.Background(), nil, true, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit=%d want 0 (strict + all pass)\nstdout=%s\nstderr=%s",
			code, stdout.String(), stderr.String())
	}
}

func TestRunReadyCheck_UnknownIDExitsOne(t *testing.T) {
	setupAcceptRepo(t, "test-ready-unknown")

	var stdout, stderr bytes.Buffer
	code := runReadyCheck(context.Background(), []string{"FEAT-999"}, false, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("exit=%d want 1 (unknown ID is always failure)\nstdout=%s\nstderr=%s",
			code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stderr.String(), "FEAT-999: not found") {
		t.Fatalf("stderr missing 'FEAT-999: not found': %s", stderr.String())
	}
}
