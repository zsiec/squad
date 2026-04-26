package installer

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

func TestManifest_ValidatesAgainstUpstreamSchema(t *testing.T) {
	schemaPath := filepath.Join("testdata", "plugin-manifest.schema.json")
	raw, err := os.ReadFile(schemaPath)
	if err != nil {
		t.Skipf("schema fixture missing: %v", err)
	}
	// santhosh-tekuri/jsonschema uses Go's RE2 regexp for the `regex` format,
	// which rejects ECMAScript negative lookahead. The vendored community
	// schema uses one such pattern in $defs/hooksInline to forbid the literal
	// key "description" inside an inline-hooks object. Squad references hooks
	// by file ("hooks": "hooks.json"), so the inline form is unreachable from
	// our manifest; relaxing the pattern to drop the lookahead lets the
	// meta-schema accept it without weakening any check that applies here.
	patched := bytes.Replace(
		raw,
		[]byte(`"^(?!description$)[A-Za-z][A-Za-z0-9]*$"`),
		[]byte(`"^[A-Za-z][A-Za-z0-9]*$"`),
		1,
	)

	const schemaURL = "https://squad.local/plugin-manifest.schema.json"

	schemaDoc, err := jsonschema.UnmarshalJSON(bytes.NewReader(patched))
	if err != nil {
		t.Fatalf("parse schema: %v", err)
	}
	compiler := jsonschema.NewCompiler()
	if err := compiler.AddResource(schemaURL, schemaDoc); err != nil {
		t.Fatal(err)
	}
	schema, err := compiler.Compile(schemaURL)
	if err != nil {
		t.Fatal(err)
	}

	manifestRaw, err := os.ReadFile(filepath.Join("..", "..", "plugin", ".claude-plugin", "plugin.json"))
	if err != nil {
		t.Fatal(err)
	}
	var doc any
	if err := json.Unmarshal(manifestRaw, &doc); err != nil {
		t.Fatal(err)
	}
	if err := schema.Validate(doc); err != nil {
		t.Fatalf("manifest invalid: %v", err)
	}
}
