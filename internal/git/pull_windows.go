//go:build windows

package git

import (
	"os/exec"
	"syscall"
)

// detachedProcess is Windows' DETACHED_PROCESS creation flag.
const detachedProcess = 0x00000008

// detach runs cmd without a console, so ssh has nowhere to ask for a
// passphrase and fails instead of waiting.
func detach(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: detachedProcess}
}
