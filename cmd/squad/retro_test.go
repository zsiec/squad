package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestRetro_WritesAndRegenSkill exercises the AC contract for the
// cobra-side write path: .squad/retros/YYYY-WNN.md exists, contains
// the rendered body, and the skill stub at
// .claude/skills/squad-retros.md points at the most-recent file.
func TestRetro_WritesAndRegenSkill(t *testing.T) {
	env := newTestEnv(t)
	for i := 0; i < 6; i++ {
		id := "FEAT-9" + string(rune('0'+i))
		_, err := env.DB.Exec(`INSERT INTO items (repo_id, item_id, title, type, priority, area,
			status, estimate, risk, ac_total, ac_checked, archived, path, updated_at)
			VALUES (?, ?, 't', 'feat', 'P2', 'core', 'done', '', 'low', 0, 0, 0, '', ?)`,
			env.RepoID, id, time.Now().Unix())
		if err != nil {
			t.Fatal(err)
		}
		base := time.Date(2026, 4, 20, 12, 0, 0, 0, time.UTC).Unix()
		_, err = env.DB.Exec(`INSERT INTO claim_history (repo_id, item_id, agent_id,
			claimed_at, released_at, outcome) VALUES (?, ?, 'agent-a', ?, ?, 'done')`,
			env.RepoID, id, base+int64(i)*3600, base+int64(i)*3600+1800)
		if err != nil {
			t.Fatal(err)
		}
	}
	res, err := RunRetro(context.Background(), RetroArgs{
		DB: env.DB, RepoID: env.RepoID, RepoRoot: env.Root,
		Week: "2026-W17", MinItems: 5,
	})
	if err != nil {
		t.Fatalf("RunRetro: %v", err)
	}
	if !res.Sufficient {
		t.Errorf("Sufficient=false; want true (6 closes >= 5)")
	}
	if res.Period != "2026-W17" {
		t.Errorf("Period=%q want 2026-W17", res.Period)
	}
	body, err := os.ReadFile(res.Path)
	if err != nil {
		t.Fatalf("read retro file: %v", err)
	}
	if !strings.Contains(string(body), "# Retro 2026-W17") {
		t.Errorf("retro file missing header:\n%s", body)
	}
	skillPath := filepath.Join(env.Root, retroSkillRel)
	skill, err := os.ReadFile(skillPath)
	if err != nil {
		t.Fatalf("read skill: %v", err)
	}
	if !strings.Contains(string(skill), "2026-W17.md") {
		t.Errorf("skill stub does not reference 2026-W17.md:\n%s", skill)
	}
	if !strings.Contains(string(skill), "name: squad-retros") {
		t.Errorf("skill stub missing frontmatter name:\n%s", skill)
	}
}

func TestRetro_IdempotentOverwrite(t *testing.T) {
	env := newTestEnv(t)
	for i := 0; i < 6; i++ {
		id := "BUG-7" + string(rune('0'+i))
		_, err := env.DB.Exec(`INSERT INTO items (repo_id, item_id, title, type, priority, area,
			status, estimate, risk, ac_total, ac_checked, archived, path, updated_at)
			VALUES (?, ?, 't', 'bug', 'P2', 'core', 'done', '', 'low', 0, 0, 0, '', ?)`,
			env.RepoID, id, time.Now().Unix())
		if err != nil {
			t.Fatal(err)
		}
		base := time.Date(2026, 4, 20, 12, 0, 0, 0, time.UTC).Unix()
		_, err = env.DB.Exec(`INSERT INTO claim_history (repo_id, item_id, agent_id,
			claimed_at, released_at, outcome) VALUES (?, ?, 'agent-a', ?, ?, 'done')`,
			env.RepoID, id, base+int64(i)*3600, base+int64(i)*3600+1800)
		if err != nil {
			t.Fatal(err)
		}
	}
	for i := 0; i < 3; i++ {
		_, err := RunRetro(context.Background(), RetroArgs{
			DB: env.DB, RepoID: env.RepoID, RepoRoot: env.Root,
			Week: "2026-W17", MinItems: 5,
		})
		if err != nil {
			t.Fatalf("RunRetro #%d: %v", i, err)
		}
	}
	entries, err := os.ReadDir(filepath.Join(env.Root, ".squad", "retros"))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Errorf("retros dir contains %d files, want 1 (idempotent overwrite)", len(entries))
	}
}
