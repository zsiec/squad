package hygiene

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/zsiec/squad/plugin/hooks"
)

type DriftKind int

const (
	DriftModified DriftKind = iota + 1
	DriftMissing
)

func (k DriftKind) String() string {
	switch k {
	case DriftModified:
		return "modified"
	case DriftMissing:
		return "missing"
	default:
		return "unknown"
	}
}

type HookFinding struct {
	Filename      string
	Kind          DriftKind
	EmbeddedHash  string
	InstalledHash string
}

func DetectHookDrift(installDir string) ([]HookFinding, error) {
	if _, err := os.Stat(installDir); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	var out []HookFinding
	embedFiles, err := fs.ReadDir(hooks.FS, ".")
	if err != nil {
		return nil, err
	}
	for _, e := range embedFiles {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if filepath.Ext(name) != ".sh" {
			continue
		}
		embedded, err := fs.ReadFile(hooks.FS, name)
		if err != nil {
			return nil, err
		}
		embedHash := hookHash(embedded)
		installed, err := os.ReadFile(filepath.Join(installDir, name))
		if errors.Is(err, os.ErrNotExist) {
			out = append(out, HookFinding{Filename: name, Kind: DriftMissing, EmbeddedHash: embedHash})
			continue
		}
		if err != nil {
			return nil, err
		}
		if h := hookHash(installed); h != embedHash {
			out = append(out, HookFinding{
				Filename:      name,
				Kind:          DriftModified,
				EmbeddedHash:  embedHash,
				InstalledHash: h,
			})
		}
	}
	return out, nil
}

func hookHash(b []byte) string {
	s := sha256.Sum256(b)
	return hex.EncodeToString(s[:8])
}
