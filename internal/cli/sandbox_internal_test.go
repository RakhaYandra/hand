package cli

import (
	"os"
	"path/filepath"
	"testing"
)

func mkdirs(t *testing.T, paths ...string) {
	t.Helper()
	for _, path := range paths {
		if err := os.MkdirAll(path, 0o700); err != nil {
			t.Fatal(err)
		}
	}
}

func TestLinkedGitDirMustBeADirectChildOfWorktrees(t *testing.T) {
	root := realPath(t.TempDir())
	common := filepath.Join(root, "repo", ".git")
	mkdirs(t, common, filepath.Join(common, "worktrees", "t1-a2"), filepath.Join(common, "hooks"))

	if got := linkedGitDir(common, filepath.Join(common, "worktrees", "t1-a2")); got != filepath.Join(common, "worktrees", "t1-a2") {
		t.Fatalf("linked git dir = %q", got)
	}
	for _, private := range []string{
		common,
		filepath.Join(common, "hooks"),
		filepath.Join(common, "worktrees"),
		filepath.Join(common, "worktrees", "t1-a2", "nested"),
		filepath.Join(root, "repo", "other"),
	} {
		if got := linkedGitDir(common, private); got != "" {
			t.Fatalf("%s was accepted as the linked git dir: %q", private, got)
		}
	}
}

func TestLocalRemoteNeedsARepository(t *testing.T) {
	root := realPath(t.TempDir())
	t.Setenv("HOME", filepath.Join(root, "home"))
	worktree := filepath.Join(root, "work")
	mkdirs(t, worktree)
	repo := filepath.Join(root, "repo")
	bare := filepath.Join(root, "bare.git")
	mkdirs(t, filepath.Join(repo, ".git"), worktree+"/relative", filepath.Join(root, "plain"))
	mkdirs(t, bare, filepath.Join(bare, "objects"))
	if err := os.WriteFile(filepath.Join(bare, "HEAD"), []byte("ref: refs/heads/main\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	for _, c := range []struct {
		url  string
		want string
	}{
		{repo, repo},
		{bare, bare},
		{"file://" + repo, repo},
		{"../repo", repo},
	} {
		if got, ok := localRemote(c.url, worktree, func() []string { return nil }); !ok || got != realPath(c.want) {
			t.Fatalf("localRemote(%q) = %q, %v", c.url, got, ok)
		}
	}
	for _, url := range []string{
		filepath.Join(root, "plain"),
		"https://example.com/repo.git",
		"git@example.com:repo.git",
		filepath.Join(root, "missing"),
	} {
		if got, ok := localRemote(url, worktree, func() []string { return nil }); ok {
			t.Fatalf("localRemote(%q) accepted %q", url, got)
		}
	}
}

func TestLocalRemoteUnderHomeNeedsARegisteredProject(t *testing.T) {
	home := realPath(t.TempDir())
	t.Setenv("HOME", home)
	worktree := filepath.Join(home, "work")
	repo := filepath.Join(home, "src", "repo")
	mkdirs(t, worktree, filepath.Join(repo, ".git"))

	if got, ok := localRemote(repo, worktree, func() []string { return nil }); ok {
		t.Fatalf("an unregistered remote under HOME was accepted: %q", got)
	}
	if got, ok := localRemote(repo, worktree, func() []string { return []string{repo} }); !ok || got != repo {
		t.Fatalf("a registered remote under HOME = %q, %v", got, ok)
	}
	if got, ok := localRemote(repo, worktree, func() []string { return []string{filepath.Join(home, "src", "other")} }); ok {
		t.Fatalf("a different registered project accepted %q", got)
	}
}

func TestGitCommonMustBelongToTheWorktreeOrAProject(t *testing.T) {
	root := realPath(t.TempDir())
	repo := filepath.Join(root, "repo")
	wt := filepath.Join(root, "wt")
	other := filepath.Join(root, "other")
	mkdirs(t, filepath.Join(repo, ".git", "objects"), filepath.Join(wt, ".git", "objects"), filepath.Join(other, ".git"))
	bare := filepath.Join(root, "bare.git")
	mkdirs(t, bare, filepath.Join(bare, "objects"))
	if err := os.WriteFile(filepath.Join(bare, "HEAD"), []byte("ref: refs/heads/main\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	common := realPath(filepath.Join(repo, ".git"))
	if !trustedGitCommon(wt, common, []string{repo}) {
		t.Fatal("a project's git dir was not trusted")
	}
	if trustedGitCommon(wt, common, nil) {
		t.Fatal("a project's git dir was trusted without the project")
	}
	if !trustedGitCommon(wt, realPath(filepath.Join(wt, ".git")), nil) {
		t.Fatal("the worktree's own git dir was not trusted")
	}
	if trustedGitCommon(wt, realPath(filepath.Join(other, ".git")), []string{repo}) {
		t.Fatal("an unrelated git dir was trusted")
	}
	if !trustedGitCommon(wt, bare, []string{bare}) {
		t.Fatal("a bare project was not trusted")
	}

	link := filepath.Join(root, "linked")
	mkdirs(t, link)
	if err := os.Symlink(common, filepath.Join(link, ".git")); err != nil {
		t.Fatal(err)
	}
	if trustedGitCommon(link, common, nil) {
		t.Fatal("a symlinked git dir was trusted")
	}
}
