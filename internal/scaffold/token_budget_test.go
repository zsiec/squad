package scaffold

import (
	"testing"
)

// AGENTS.md is a single document — the former fast/deep tier split was
// merged. Budget exists to prevent unbounded drift; raise deliberately if
// new doctrine genuinely warrants the cost of always-loaded context.
const maxAgentsTokens = 4000

// approxTokens uses the rule of thumb 1 token ≈ 4 bytes of UTF-8 English
// text. Real tokenizer counts vary; this is conservative — actual GPT/Claude
// tokenizers usually produce ~10% more tokens for prose like AGENTS.md, so a
// rendered file at maxAgentsTokens by this estimate already gives slack.
func approxTokens(s string) int {
	return (len(s) + 3) / 4
}

func TestAgentsTemplate_WithinTokenBudget(t *testing.T) {
	raw, err := Templates.ReadFile("templates/AGENTS.md.tmpl")
	if err != nil {
		t.Fatal(err)
	}
	rendered, err := Render(string(raw), Data{
		ProjectName: "TestProject",
		IDPrefixes:  []string{"BUG", "FEAT", "TASK", "CHORE"},
	})
	if err != nil {
		t.Fatal(err)
	}
	got := approxTokens(rendered)
	if got > maxAgentsTokens {
		t.Fatalf("AGENTS.md.tmpl rendered to ~%d tokens, want ≤%d. Trim before raising the budget.",
			got, maxAgentsTokens)
	}
	t.Logf("AGENTS.md.tmpl ~%d tokens (budget %d)", got, maxAgentsTokens)
}
