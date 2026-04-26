package installer

import (
	"archive/tar"
	"compress/gzip"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	squad "github.com/zsiec/squad"
)

func TestPluginTarball_ResemblesClaudeInstallTarget(t *testing.T) {
	src, err := squad.PluginFS()
	if err != nil {
		t.Fatal(err)
	}

	tmp := t.TempDir()
	tarball := filepath.Join(tmp, "squad-plugin.tgz")
	if err := writeTarball(src, tarball); err != nil {
		t.Fatalf("tarball: %v", err)
	}

	extracted := filepath.Join(tmp, "extracted")
	if err := extractTarball(tarball, extracted); err != nil {
		t.Fatalf("extract: %v", err)
	}

	must := []string{
		".claude-plugin/plugin.json",
		".mcp.json",
		"hooks.json",
		"hooks/session_start.sh",
		"skills/squad-loop/SKILL.md",
		"skills/squad-handoff/SKILL.md",
		"skills/squad-done/SKILL.md",
		"commands/done.md",
	}
	for _, p := range must {
		if _, err := os.Stat(filepath.Join(extracted, p)); err != nil {
			t.Errorf("missing in extracted tarball: %s (%v)", p, err)
		}
	}

	if err := ValidatePluginRoot(extracted); err != nil {
		t.Errorf("ValidatePluginRoot: %v", err)
	}
}

func writeTarball(src fs.FS, path string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	gz := gzip.NewWriter(f)
	defer gz.Close()
	tw := tar.NewWriter(gz)
	defer tw.Close()
	return fs.WalkDir(src, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if p == "." {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		hdr, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return err
		}
		hdr.Name = p
		if d.IsDir() {
			hdr.Name += "/"
		}
		if err := tw.WriteHeader(hdr); err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		r, err := src.Open(p)
		if err != nil {
			return err
		}
		defer func() { _ = r.Close() }()
		_, err = io.Copy(tw, r)
		return err
	})
}

func extractTarball(tgz, dst string) error {
	if err := os.MkdirAll(dst, 0o755); err != nil {
		return err
	}
	f, err := os.Open(tgz)
	if err != nil {
		return err
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		target := filepath.Join(dst, hdr.Name)
		if hdr.FileInfo().IsDir() {
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		w, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, hdr.FileInfo().Mode())
		if err != nil {
			return err
		}
		if _, err := io.Copy(w, tr); err != nil {
			w.Close()
			return err
		}
		if err := w.Close(); err != nil {
			return err
		}
	}
}
