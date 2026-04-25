package repo

import (
	"os/exec"
	"strings"
)

// DiscoverAndRegister reads the git remote at rootPath and registers the
// repo with r. Returns (id, warning, error). Idempotent: running twice from
// the same path yields the same id and an empty warning the second time.
func DiscoverAndRegister(rootPath string, r *Registry) (string, string, error) {
	cmd := exec.Command("git", "-C", rootPath, "config", "--get", "remote.origin.url")
	out, _ := cmd.Output()
	remote := strings.TrimSpace(string(out))
	return r.Register(remote, rootPath)
}
