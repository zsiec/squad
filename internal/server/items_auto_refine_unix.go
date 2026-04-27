//go:build !windows

package server

import (
	"os/exec"
	"syscall"
)

// autoRefineSetProcessGroup runs the spawned `claude -p` in its own
// process group and overrides the default cmd.Cancel so context-cancel
// signals the whole group, not just the leader. The default exec.Cmd
// cancellation calls Process.Kill on the leader pid; helpers `claude -p`
// forks (e.g. an MCP subprocess) would survive as orphans. squad does
// not target windows; the build tag scopes the syscall to unix.
func autoRefineSetProcessGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		if cmd.Process == nil {
			return nil
		}
		return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	}
}
