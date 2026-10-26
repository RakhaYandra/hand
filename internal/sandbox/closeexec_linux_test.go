//go:build linux

package sandbox

import (
	"os"
	"testing"

	"golang.org/x/sys/unix"
)

func TestCloseOnExecMarksDescriptorsAboveTwo(t *testing.T) {
	f, err := os.Open("/dev/null")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if _, err := unix.FcntlInt(f.Fd(), unix.F_SETFD, 0); err != nil {
		t.Fatal(err)
	}
	if err := CloseOnExec(); err != nil {
		t.Fatal(err)
	}
	flags, err := unix.FcntlInt(f.Fd(), unix.F_GETFD, 0)
	if err != nil {
		t.Fatal(err)
	}
	if flags&unix.FD_CLOEXEC == 0 {
		t.Fatal("descriptor above 2 kept across exec")
	}
}
