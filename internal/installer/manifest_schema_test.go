package installer

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v5"
)

func TestManifest_ValidatesAgainstUpstreamSchema(t *testing.T) {
	schemaPath := filepath.Join("testdata", "plugin-manifest.schema.json")
	if _, err := os.Stat(schemaPath); err != nil {
		t.Skipf("schema fixture missing: %v", err)
	}
	compiler := jsonschema.NewCompiler()
	if err := compiler.AddResource("plugin-manifest", mustOpen(t, schemaPath)); err != nil {
		t.Fatal(err)
	}
	schema, err := compiler.Compile("plugin-manifest")
	if err != nil {
		t.Fatal(err)
	}

	raw, err := os.ReadFile(filepath.Join("..", "..", "plugin", ".claude-plugin", "plugin.json"))
	if err != nil {
		t.Fatal(err)
	}
	var doc any
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	if err := schema.Validate(doc); err != nil {
		t.Fatalf("manifest invalid: %v", err)
	}
}

func mustOpen(t *testing.T, path string) *os.File {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { f.Close() })
	return f
}
