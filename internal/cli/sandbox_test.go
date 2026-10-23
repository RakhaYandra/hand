package cli_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSandboxPrintShowsThePolicy(t *testing.T) {
	h := newHarness(t)
	home := t.TempDir()
	h.vars["HOME"] = home
	wt := filepath.Join(t.TempDir(), "t1-a2")
	if err := os.MkdirAll(wt, 0o700); err != nil {
		t.Fatal(err)
	}
	out := h.ok("sandbox", "exec", "--print", "--access", "boxed", "--worktree", wt, "--", "/bin/true")
	for _, want := range []string{"access: boxed", "backend: landlock", "worktree: " + wt, "rules["} {
		if !strings.Contains(out, want) {
			t.Fatalf("output lacks %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, ".ssh") {
		t.Fatalf("output grants the ssh folder:\n%s", out)
	}
}

func TestSandboxPrintNamesOpen(t *testing.T) {
	h := newHarness(t)
	out := h.ok("sandbox", "exec", "--print", "--worktree", t.TempDir(), "--", "/bin/true")
	if !strings.HasPrefix(out, "access: open\n") {
		t.Fatalf("output = %q", out)
	}
}

func TestSandboxCheckReportsTheBackend(t *testing.T) {
	h := newHarness(t)
	out := h.ok("sandbox", "check")
	if !strings.Contains(out, "backend: landlock") || !strings.Contains(out, "landlock,ok") {
		t.Fatalf("output = %q", out)
	}
}

func TestSandboxExecRejectsBadInput(t *testing.T) {
	h := newHarness(t)
	wt := t.TempDir()
	if _, errOut, code := h.run("sandbox", "exec", "--access", "jailed", "--worktree", wt, "--", "/bin/true"); code != 2 || !strings.Contains(errOut, "--access must be one of") {
		t.Fatalf("code=%d stderr=%q", code, errOut)
	}
	if _, errOut, code := h.run("sandbox", "exec", "--worktree", wt, "--access", "boxed", "--attempt", "a1", "--", "/bin/true"); code != 2 || !strings.Contains(errOut, "not both") {
		t.Fatalf("code=%d stderr=%q", code, errOut)
	}
	if _, errOut, code := h.run("sandbox", "exec", "--worktree", wt); code != 2 || !strings.Contains(errOut, "give a command") {
		t.Fatalf("code=%d stderr=%q", code, errOut)
	}
}

func TestSandboxExecOpenRunsTheCommand(t *testing.T) {
	h := newHarness(t)
	wt := t.TempDir()
	if _, errOut, code := h.run("sandbox", "exec", "--worktree", wt, "--", "/bin/true", "one"); code != 0 {
		t.Fatalf("exit %d: %s", code, errOut)
	}
	if len(h.exec) != 1 || strings.Join(h.exec[0], " ") != "/bin/true /bin/true one" {
		t.Fatalf("exec = %v", h.exec)
	}
}
