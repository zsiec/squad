package config

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestLoad_MissingFileReturnsDefaults(t *testing.T) {
	dir := t.TempDir()
	cfg, err := Load(dir)
	if err != nil {
		t.Fatalf("Load returned error for missing config: %v", err)
	}
	want := []string{"BUG", "FEAT", "TASK", "CHORE"}
	if !reflect.DeepEqual(cfg.IDPrefixes, want) {
		t.Fatalf("default prefixes = %v, want %v", cfg.IDPrefixes, want)
	}
}

func TestLoad_CustomPrefixes(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".squad"), 0o755); err != nil {
		t.Fatal(err)
	}
	body := "id_prefixes: [STORY, SPIKE, INC]\n"
	if err := os.WriteFile(filepath.Join(dir, ".squad", "config.yaml"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(cfg.IDPrefixes, []string{"STORY", "SPIKE", "INC"}) {
		t.Fatalf("got %v", cfg.IDPrefixes)
	}
}

func TestLoad_MalformedYAMLIsError(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".squad"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".squad", "config.yaml"), []byte("id_prefixes: ["), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(dir); err == nil {
		t.Fatal("expected error for malformed yaml")
	}
}

func TestLoadTouchConfig(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".squad"), 0o755); err != nil {
		t.Fatal(err)
	}
	body := "touch:\n  enforcement: deny\n  enforcement_paths:\n    - go.mod\n    - \"**/*.lock\"\n"
	if err := os.WriteFile(filepath.Join(dir, ".squad", "config.yaml"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Touch.Enforcement != "deny" {
		t.Fatalf("Touch.Enforcement=%q want \"deny\"", cfg.Touch.Enforcement)
	}
	if !reflect.DeepEqual(cfg.Touch.EnforcementPaths, []string{"go.mod", "**/*.lock"}) {
		t.Fatalf("Touch.EnforcementPaths=%v want [go.mod **/*.lock]", cfg.Touch.EnforcementPaths)
	}
}

func TestValidateTouch(t *testing.T) {
	cases := []struct {
		name        string
		cfg         TouchConfig
		wantWarn    bool
		wantContain []string
	}{
		{name: "empty", cfg: TouchConfig{}},
		{name: "warn", cfg: TouchConfig{Enforcement: TouchEnforcementWarn}},
		{name: "deny", cfg: TouchConfig{Enforcement: TouchEnforcementDeny}},
		{
			name:        "typo_denied",
			cfg:         TouchConfig{Enforcement: "denied"},
			wantWarn:    true,
			wantContain: []string{"denied", "warn", "deny"},
		},
		{
			name:        "typo_warning",
			cfg:         TouchConfig{Enforcement: "warning"},
			wantWarn:    true,
			wantContain: []string{"warning", "warn", "deny"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			warns := ValidateTouch(tc.cfg)
			if tc.wantWarn {
				if len(warns) == 0 {
					t.Fatalf("expected warning, got none")
				}
				joined := warns[0]
				for _, sub := range tc.wantContain {
					if !contains(joined, sub) {
						t.Fatalf("warning %q missing %q", joined, sub)
					}
				}
				return
			}
			if len(warns) != 0 {
				t.Fatalf("expected no warnings, got %v", warns)
			}
		})
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

func TestLoad_DefaultsEvidenceRequired(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".squad"), 0o755); err != nil {
		t.Fatal(err)
	}
	body := "defaults:\n  evidence_required: [test, review]\n"
	if err := os.WriteFile(filepath.Join(dir, ".squad", "config.yaml"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	got := cfg.Defaults.EvidenceRequired
	if len(got) != 2 || got[0] != "test" || got[1] != "review" {
		t.Fatalf("EvidenceRequired=%v want [test review]", got)
	}
}

func TestLoad_HygieneKnobs(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".squad"), 0o755); err != nil {
		t.Fatal(err)
	}
	body := "hygiene:\n  stale_claim_minutes: 5\n  sweep_on_every_command: false\n"
	if err := os.WriteFile(filepath.Join(dir, ".squad", "config.yaml"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Hygiene.StaleClaimMinutes != 5 {
		t.Fatalf("StaleClaimMinutes=%d want 5", cfg.Hygiene.StaleClaimMinutes)
	}
	if cfg.Hygiene.SweepOnEveryCommand == nil || *cfg.Hygiene.SweepOnEveryCommand {
		t.Fatalf("SweepOnEveryCommand: want false, got %v", cfg.Hygiene.SweepOnEveryCommand)
	}
}
