package items

import "testing"

func TestValidateID_AllowsConfiguredPrefix(t *testing.T) {
	if err := ValidateID("BUG-007", []string{"BUG", "FEAT"}); err != nil {
		t.Fatalf("BUG-007 should be valid: %v", err)
	}
	if err := ValidateID("FEAT-100", []string{"BUG", "FEAT"}); err != nil {
		t.Fatalf("FEAT-100 should be valid: %v", err)
	}
}

func TestValidateID_RejectsUnknownPrefix(t *testing.T) {
	err := ValidateID("STORY-1", []string{"BUG", "FEAT"})
	if err == nil {
		t.Fatal("STORY-1 should be invalid against [BUG FEAT]")
	}
}

func TestValidateID_RejectsMalformedID(t *testing.T) {
	cases := []string{"", "BUG", "BUG-", "BUG-abc", "-7", "bug-7"}
	for _, c := range cases {
		if err := ValidateID(c, []string{"BUG"}); err == nil {
			t.Fatalf("expected error for %q", c)
		}
	}
}

func TestParseID_SplitsPrefixAndNumber(t *testing.T) {
	prefix, n, err := ParseID("FEAT-042")
	if err != nil {
		t.Fatal(err)
	}
	if prefix != "FEAT" || n != 42 {
		t.Fatalf("got %q %d", prefix, n)
	}
}
