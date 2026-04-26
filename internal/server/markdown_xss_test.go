package server

import (
	"encoding/json"
	"os/exec"
	"strings"
	"testing"
)

// TestMarkdownLinkXSS runs the SPA's markdown renderer through Node and
// asserts that javascript:/data:/vbscript: URLs do not produce a clickable
// <a href> — they fall through to plain text. Skipped if node is missing.
func TestMarkdownLinkXSS(t *testing.T) {
	if _, err := exec.LookPath("node"); err != nil {
		t.Skip("node not available")
	}

	cases := []struct {
		name      string
		input     string
		wantHref  bool   // expect an <a href="..."> in output
		wantHrefV string // if wantHref, the href value should equal this
		wantText  string // a substring that must appear in the output
	}{
		{name: "javascript URL", input: "[click](javascript:alert(1))", wantHref: false, wantText: "[click](javascript:alert(1))"},
		{name: "data URL", input: "[x](data:text/html,<script>alert(1)</script>)", wantHref: false, wantText: "[x]"},
		{name: "vbscript URL", input: "[x](vbscript:msgbox)", wantHref: false, wantText: "[x](vbscript:msgbox)"},
		{name: "case-insensitive js scheme", input: "[x](JaVaScRiPt:alert(1))", wantHref: false, wantText: "[x]("},
		{name: "leading whitespace js", input: "[x]( javascript:alert(1))", wantHref: false, wantText: "[x]("},
		{name: "tab-in-scheme js", input: "[x](\tjavascript:alert(1))", wantHref: false, wantText: "[x]("},
		{name: "protocol-relative blocked", input: "[x](//evil.com/foo)", wantHref: false, wantText: "[x](//evil.com/foo)"},
		{name: "https allowed", input: "[ok](https://example.com)", wantHref: true, wantHrefV: "https://example.com"},
		{name: "http allowed", input: "[ok](http://example.com/path)", wantHref: true, wantHrefV: "http://example.com/path"},
		{name: "mailto allowed", input: "[mail](mailto:a@b.com)", wantHref: true, wantHrefV: "mailto:a@b.com"},
		{name: "relative allowed", input: "[rel](/foo/bar)", wantHref: true, wantHrefV: "/foo/bar"},
		{name: "anchor allowed", input: "[a](#section)", wantHref: true, wantHrefV: "#section"},
	}

	inputs := make([]string, len(cases))
	for i, c := range cases {
		inputs[i] = c.input
	}
	inJSON, err := json.Marshal(inputs)
	if err != nil {
		t.Fatal(err)
	}

	// util.js touches browser globals at module load, so stub them before import.
	const harness = `
globalThis.location = { search: '' };
globalThis.localStorage = { getItem: () => null, setItem: () => {} };
const { renderMarkdown } = await import('./markdown.js');
const inputs = JSON.parse(process.argv[1]);
const out = inputs.map(i => renderMarkdown(i));
process.stdout.write(JSON.stringify(out));
`
	cmd := exec.Command("node", "--input-type=module", "--eval", harness, string(inJSON))
	cmd.Dir = "web"
	stdout, err := cmd.Output()
	if err != nil {
		var stderr string
		if ee, ok := err.(*exec.ExitError); ok {
			stderr = string(ee.Stderr)
		}
		t.Fatalf("node harness failed: %v\nstderr: %s", err, stderr)
	}
	var results []string
	if err := json.Unmarshal(stdout, &results); err != nil {
		t.Fatalf("decode: %v\nstdout: %s", err, stdout)
	}
	if len(results) != len(cases) {
		t.Fatalf("results=%d want %d", len(results), len(cases))
	}

	for i, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			html := results[i]
			gotHref := strings.Contains(html, "<a href=")
			if gotHref != c.wantHref {
				t.Fatalf("wantHref=%v gotHref=%v\nhtml: %s", c.wantHref, gotHref, html)
			}
			if c.wantHref && c.wantHrefV != "" {
				want := `href="` + c.wantHrefV + `"`
				if !strings.Contains(html, want) {
					t.Fatalf("wanted %q in html: %s", want, html)
				}
			}
			if c.wantText != "" && !strings.Contains(html, c.wantText) {
				t.Fatalf("wanted text %q in html: %s", c.wantText, html)
			}
		})
	}
}
