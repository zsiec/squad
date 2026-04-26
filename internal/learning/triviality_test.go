package learning

import (
	"strings"
	"testing"
)

func TestNonTrivial_TenLinesInGoFile(t *testing.T) {
	in := strings.Join([]string{
		"5\t2\tinternal/store/store.go",
		"6\t0\tinternal/store/store.go",
	}, "\n")
	if !NonTrivialDiff(in) {
		t.Errorf("11 net adds in store.go should count as non-trivial")
	}
}

func TestTrivial_NineLines(t *testing.T) {
	if NonTrivialDiff("9\t0\tinternal/store/store.go") {
		t.Errorf("9 net adds is trivial")
	}
}

func TestTrivial_TestsOnly(t *testing.T) {
	in := strings.Join([]string{
		"50\t0\tinternal/store/store_test.go",
		"30\t0\tcmd/squad/done_test.go",
	}, "\n")
	if NonTrivialDiff(in) {
		t.Errorf("test-only churn should be trivial")
	}
}

func TestTrivial_DocsOnly(t *testing.T) {
	in := strings.Join([]string{
		"100\t10\tdocs/concepts/learnings.md",
		"40\t0\tREADME.md",
	}, "\n")
	if NonTrivialDiff(in) {
		t.Errorf("docs-only churn should be trivial")
	}
}

func TestTrivial_VendoredCode(t *testing.T) {
	if NonTrivialDiff("500\t0\tvendor/foo/bar.go") {
		t.Errorf("vendored churn should be trivial")
	}
}

func TestTrivial_BinaryOnly(t *testing.T) {
	if NonTrivialDiff("-\t-\timages/logo.png") {
		t.Errorf("binary churn should be trivial")
	}
}

func TestNonTrivial_ProductionAndTestMixed(t *testing.T) {
	in := strings.Join([]string{
		"15\t0\tinternal/store/store.go",
		"50\t0\tinternal/store/store_test.go",
	}, "\n")
	if !NonTrivialDiff(in) {
		t.Errorf("15 prod-line adds should clear the bar even with tests in mix")
	}
}

func TestTrivial_Empty(t *testing.T) {
	if NonTrivialDiff("") {
		t.Errorf("empty diff is trivial")
	}
}

func TestTrivial_RenameIntoVendoredBraceForm(t *testing.T) {
	// `git diff --numstat` rename of internal/foo/bar.go → vendor/foo/bar.go
	if NonTrivialDiff("50\t0\t{internal => vendor}/foo/bar.go") {
		t.Errorf("rename INTO vendor/ should be excluded; the new path is vendored")
	}
}

func TestTrivial_RenameIntoTestFileBraceForm(t *testing.T) {
	if NonTrivialDiff("50\t0\tinternal/foo/{foo.go => foo_test.go}") {
		t.Errorf("rename to *_test.go should be excluded")
	}
}

func TestTrivial_RenameArrowFormToTestFile(t *testing.T) {
	// `-C` arrow form when paths share no common prefix.
	if NonTrivialDiff("50\t0\told.go => new_test.go") {
		t.Errorf("arrow-form rename to *_test.go should be excluded")
	}
}

func TestNonTrivial_RenameWithinProductionStaysCounted(t *testing.T) {
	if !NonTrivialDiff("15\t0\t{internal/foo => internal/bar}/baz.go") {
		t.Errorf("production-to-production rename should still count")
	}
}
