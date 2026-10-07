//go:build unix

package git

import (
	"os/exec"
	"syscall"
)

// detach runs cmd in a new session with no controlling terminal, so ssh
// can't open /dev/tty to ask for a passphrase.
func detach(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
}
