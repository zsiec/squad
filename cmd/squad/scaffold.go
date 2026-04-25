package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/zsiec/squad/internal/repo"
)

func discoverSquadDir() (string, int) {
	wd, _ := os.Getwd()
	root, err := repo.Discover(wd)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return "", 4
	}
	return filepath.Join(root, ".squad"), 0
}

func writeScaffold(stdout io.Writer, squadDir, subdir, name, body, kind string, refuseExisting bool) (string, int) {
	dir := filepath.Join(squadDir, subdir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return "", 4
	}
	path := filepath.Join(dir, name+".md")
	if refuseExisting {
		if _, err := os.Stat(path); err == nil {
			fmt.Fprintf(os.Stderr, "%s %s already exists\n", kind, path)
			return "", 4
		}
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return "", 4
	}
	fmt.Fprintln(stdout, path)
	return path, 0
}
