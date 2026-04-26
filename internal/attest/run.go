package attest

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

type RunOpts struct {
	ItemID   string
	Kind     Kind
	Command  string
	AgentID  string
	AttDir   string
	RepoRoot string
}

func (l *Ledger) Run(ctx context.Context, opts RunOpts) (Record, error) {
	if opts.ItemID == "" || opts.Command == "" || opts.AgentID == "" || opts.AttDir == "" {
		return Record{}, fmt.Errorf("attest.Run: ItemID, Command, AgentID, AttDir required")
	}
	if !opts.Kind.Valid() {
		return Record{}, fmt.Errorf("attest.Run: invalid kind %q", opts.Kind)
	}

	var buf bytes.Buffer
	cmd := exec.CommandContext(ctx, "sh", "-c", opts.Command)
	cmd.Dir = opts.RepoRoot
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	exitCode := 0
	if err := cmd.Run(); err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			exitCode = ee.ExitCode()
		} else {
			return Record{}, fmt.Errorf("attest.Run exec: %w (output: %s)", err, buf.String())
		}
	}

	hash := l.Hash(buf.Bytes())
	if err := os.MkdirAll(opts.AttDir, 0o755); err != nil {
		return Record{}, fmt.Errorf("mkdir attestations dir: %w", err)
	}
	out := filepath.Join(opts.AttDir, hash+".txt")
	if err := os.WriteFile(out, buf.Bytes(), 0o644); err != nil {
		return Record{}, fmt.Errorf("write attestation file: %w", err)
	}

	rec := Record{
		ItemID:     opts.ItemID,
		Kind:       opts.Kind,
		Command:    opts.Command,
		ExitCode:   exitCode,
		OutputHash: hash,
		OutputPath: out,
		AgentID:    opts.AgentID,
	}
	id, err := l.Insert(ctx, rec)
	if err != nil {
		return Record{}, err
	}
	rec.ID = id
	rec.RepoID = l.repoID
	return rec, nil
}
