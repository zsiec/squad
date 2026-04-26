# Testdata

## `plugin-manifest.schema.json`

JSON Schema for the Claude Code plugin manifest (`.claude-plugin/plugin.json`),
vendored as a test fixture so
`internal/installer/manifest_schema_test.go::TestManifest_ValidatesAgainstUpstreamSchema`
can validate `plugin/.claude-plugin/plugin.json` against a documented contract
without a network round-trip on every test run.

### Provenance

The schema is **community-maintained** at
[hesreallyhim/claude-code-json-schema](https://github.com/hesreallyhim/claude-code-json-schema).
Anthropic has not yet published an official schema URL; the community schema
tracks the official plugin reference at
https://code.claude.com/docs/en/plugins-reference. Switch to the official
schema if/when one ships.

### Re-fetch

```bash
curl -fsSL https://raw.githubusercontent.com/hesreallyhim/claude-code-json-schema/main/schemas/plugin.schema.json \
  > internal/installer/testdata/plugin-manifest.schema.json
```

Re-fetch on community-schema major bumps. Squad pins to the schema version it
was tested against; bumping is a deliberate act.

### Schema regex compatibility note

The community schema's `$defs/hooksInline` uses ECMAScript negative-lookahead
in a `patternProperties` key (`^(?!description$)...`). Go's RE2 (used by
`santhosh-tekuri/jsonschema/v5`) rejects lookaround. The validation test
patches that one pattern in memory before compilation; the on-disk fixture
stays verbatim. The patched branch (`hooksInline`) is unreachable from squad's
manifest because we use `"hooks": "./hooks.json"` (string form), not the
inline-object form. If squad ever switches to the inline form, revisit.

### Behaviour when missing

The validation test calls `t.Skip` when the fixture is absent so a developer
without network access can still run `go test ./...`. CI environments should
re-fetch the schema before running tests if strict validation is required;
omitting the fetch downgrades the validation to a soft check.
