package tui_test

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

var bannedImports = []string{
	"github.com/zsiec/squad/internal/store",
	"github.com/zsiec/squad/internal/items",
	"github.com/zsiec/squad/internal/chat",
	"github.com/zsiec/squad/internal/specs",
	"github.com/zsiec/squad/internal/epics",
	"github.com/zsiec/squad/internal/learning",
	"github.com/zsiec/squad/internal/attest",
	"github.com/zsiec/squad/internal/claims",
	"github.com/zsiec/squad/internal/touch",
	"github.com/zsiec/squad/internal/hygiene",
	"github.com/zsiec/squad/internal/identity",
	"github.com/zsiec/squad/internal/repo",
	"github.com/zsiec/squad/internal/workspace",
	"github.com/zsiec/squad/internal/notify",
	"github.com/zsiec/squad/internal/listener",
	"github.com/zsiec/squad/internal/stats",
	"github.com/zsiec/squad/internal/installer",
	"github.com/zsiec/squad/internal/server",
	"github.com/zsiec/squad/internal/mcp",
}

var enforcedDirs = []string{
	"internal/tui/views",
	"internal/tui/components",
}

func TestImportBoundary(t *testing.T) {
	root := repoRoot(t)
	for _, rel := range enforcedDirs {
		dir := filepath.Join(root, rel)
		err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if info.IsDir() {
				return nil
			}
			if !strings.HasSuffix(path, ".go") {
				return nil
			}
			fset := token.NewFileSet()
			f, err := parser.ParseFile(fset, path, nil, parser.ImportsOnly)
			if err != nil {
				return err
			}
			for _, imp := range f.Imports {
				p := strings.Trim(imp.Path.Value, `"`)
				for _, banned := range bannedImports {
					if p == banned {
						t.Errorf("%s imports %s — only internal/tui/client may speak to storage layer", path, banned)
					}
				}
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
}

func repoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("could not find repo root")
		}
		dir = parent
	}
}
