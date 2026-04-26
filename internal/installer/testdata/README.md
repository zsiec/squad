# Testdata

## `plugin-manifest.schema.json`

The Claude Code plugin manifest JSON Schema, vendored as a test fixture so
`internal/installer/manifest_schema_test.go::TestManifest_ValidatesAgainstUpstreamSchema`
can validate `plugin/.claude-plugin/plugin.json` against the upstream contract
without a network round-trip on every test run.

### Re-fetch

Re-fetch on plugin-spec major bumps. Squad pins to the schema version it was
tested against; bumping it is a deliberate act.

```bash
curl -fsSL https://code.claude.com/schemas/plugin-manifest.json \
  > internal/installer/testdata/plugin-manifest.schema.json
```

If the upstream URL ever moves, update this command and the test's reference
URL together.

### Behaviour when missing

The validation test calls `t.Skip` when the fixture is absent so a developer
without network access can still run `go test ./...`. CI environments should
fetch the schema before running tests if strict validation is required;
omitting the fetch downgrades the validation to a soft check.
