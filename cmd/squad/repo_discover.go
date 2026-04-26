package main

import (
	"os"

	"github.com/zsiec/squad/internal/repo"
)

// discoverRepoRoot resolves the squad repo root from the current working
// directory. Both errors are real boundary failures: cwd unavailable or
// no .squad/ ancestor found.
func discoverRepoRoot() (string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	return repo.Discover(wd)
}
