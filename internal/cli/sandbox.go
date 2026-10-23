package cli

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/atqamz/hand/internal/fleet"
	"github.com/atqamz/hand/internal/sandbox"
	"github.com/atqamz/hand/internal/state"
	"github.com/atqamz/hand/internal/toon"
)

var sandboxCommands = map[string]handler{
	"exec":  cmdSandboxExec,
	"check": cmdSandboxCheck,
}

func init() {
	commands["sandbox"] = func(r *runner, args []string) error { return sub(r, args, "sandbox", sandboxCommands) }
}

func cmdSandboxExec(r *runner, args []string) error {
	fs := flags("sandbox exec")
	printOnly := fs.Bool("print", false, "show the resolved policy and run nothing")
	attempt := fs.String("attempt", "", "attempt id; the policy then covers its fleet and harness")
	worktree := fs.String("worktree", "", "worktree to confine; default: the current folder")
	harness := fs.String("harness", "", "harness whose login and config folders are granted")
	access := fs.String("access", sandbox.Open, "open (no confinement) or boxed")
	if err := fs.Parse(args); err != nil {
		return usageError{fmt.Sprintf("sandbox exec: %v", err)}
	}
	argv := fs.Args()
	if len(argv) == 0 {
		return usageError{"sandbox exec: give a command after --"}
	}
	if *attempt != "" && *worktree != "" {
		return usageError{"sandbox exec: give --attempt or --worktree, not both"}
	}
	if !sandbox.ValidMode(*access) {
		return usageError{fmt.Sprintf("sandbox exec: --access must be one of %s", strings.Join(sandbox.Modes, ", "))}
	}
	bin, err := exec.LookPath(argv[0])
	if err != nil {
		return fmt.Errorf("%w: %v", state.ErrInvalid, err)
	}
	wt, spec, err := r.sandboxTarget(*attempt, *worktree, *harness)
	if err != nil {
		return err
	}
	home := r.sandboxHome(wt)
	projects := func() []string { return sandboxProjects(r, home) }
	common, private := gitDirs(wt, projects)
	env := r.env.Environ()
	if env == nil {
		env = os.Environ()
	}
	policy, err := sandbox.Resolve(sandbox.Request{
		Mode:       *access,
		Worktree:   wt,
		GitCommon:  common,
		GitPrivate: private,
		Remotes:    gitRemotes(wt, projects),
		Harness:    spec,
		Home:       home,
		Bin:        bin,
		Getenv:     r.env.Getenv,
		Environ:    env,
	})
	if err != nil {
		return err
	}
	if *printOnly {
		return r.print(sandboxDoc(policy))
	}
	if *access == sandbox.Boxed {
		if p := sandbox.Probe(); !p.UnixSockets {
			return fmt.Errorf("%w: boxed is unavailable: %s", state.ErrInvalid, p.Reason)
		}
	}
	if err := sandbox.Apply(policy); err != nil {
		return fmt.Errorf("%w: %v", state.ErrInvalid, err)
	}
	argv[0] = bin
	if r.env.Exec != nil {
		return r.env.Exec(argv[0], argv, policy.Environ)
	}
	return replaceProcess(argv, policy.Environ)
}

func (r *runner) sandboxTarget(attempt, worktree, harness string) (string, string, error) {
	if attempt != "" {
		st, err := r.store()
		if err != nil {
			return "", "", err
		}
		defer st.Close()
		id, err := parseID("a", attempt)
		if err != nil {
			return "", "", err
		}
		a, err := st.Attempt(r.ctx(), id)
		if err != nil {
			return "", "", err
		}
		if harness == "" {
			harness = a.Harness
		}
		return a.Worktree, harness, nil
	}
	dir := worktree
	if dir == "" {
		wd, err := r.getwd()
		if err != nil {
			return "", "", err
		}
		dir = wd
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		return "", "", fmt.Errorf("%w: %v", state.ErrInvalid, err)
	}
	return abs, harness, nil
}

func (r *runner) sandboxHome(worktree string) string {
	if r.home != "" {
		return r.home
	}
	id := filepath.Base(filepath.Dir(worktree))
	if filepath.Base(filepath.Dir(filepath.Dir(worktree))) != "worktrees" {
		return ""
	}
	root, err := fleet.Root(r.env.Getenv)
	if err != nil {
		return ""
	}
	home, err := fleet.Home(root, id)
	if err != nil {
		return ""
	}
	return home
}

func sandboxProjects(r *runner, home string) []string {
	if home == "" {
		return nil
	}
	st, err := state.Open(filepath.Join(home, "hand.db"), r.env.Now)
	if err != nil {
		return nil
	}
	defer st.Close()
	projects, err := st.Projects(r.ctx())
	if err != nil {
		return nil
	}
	paths := make([]string, 0, len(projects))
	for _, p := range projects {
		paths = append(paths, p.Repo)
	}
	return paths
}

func gitDirs(worktree string, projects func() []string) (common, private string) {
	common, _ = gitPath(worktree, "--git-common-dir")
	private, _ = gitPath(worktree, "--git-dir")
	if common == "" {
		return "", ""
	}
	common = realPath(common)
	if !trustedGitCommon(worktree, common, projects()) {
		return "", ""
	}
	if private == "" {
		return common, ""
	}
	return common, linkedGitDir(common, private)
}

func trustedGitCommon(worktree, common string, projects []string) bool {
	for _, base := range append([]string{worktree}, projects...) {
		if dir := gitDirAt(base); dir != "" && realPath(dir) == common {
			return true
		}
	}
	return false
}

func gitDirAt(base string) string {
	path := filepath.Join(base, ".git")
	if fi, err := os.Lstat(path); err == nil && fi.IsDir() {
		return path
	}
	fi, err := os.Lstat(base)
	if err != nil || !fi.IsDir() {
		return ""
	}
	if head, err := os.Stat(filepath.Join(base, "HEAD")); err != nil || head.IsDir() {
		return ""
	}
	if objects, err := os.Stat(filepath.Join(base, "objects")); err != nil || !objects.IsDir() {
		return ""
	}
	return base
}

func linkedGitDir(common, private string) string {
	rel, err := filepath.Rel(filepath.Join(realPath(common), "worktrees"), realPath(private))
	if err != nil || rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || strings.Contains(rel, string(filepath.Separator)) {
		return ""
	}
	return realPath(private)
}

func gitRemotes(worktree string, projects func() []string) []string {
	cmd := exec.Command("git", "-C", worktree, "config", "--local", "--get-regexp", `^remote\..*\.url$`)
	out, err := cmd.Output()
	if err != nil {
		return nil
	}
	var paths []string
	for _, line := range strings.Split(string(out), "\n") {
		_, url, ok := strings.Cut(line, " ")
		if !ok {
			continue
		}
		if p, ok := localRemote(strings.TrimSpace(url), worktree, projects); ok {
			paths = append(paths, p)
		}
	}
	return paths
}

func localRemote(url, worktree string, projects func() []string) (string, bool) {
	switch {
	case strings.HasPrefix(url, "file://"):
		url = strings.TrimPrefix(url, "file://")
	case strings.Contains(url, "://"):
		return "", false
	case strings.Contains(url, ":") && !strings.HasPrefix(url, "/"):
		return "", false
	}
	url = strings.TrimSuffix(url, "/")
	if strings.HasPrefix(url, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", false
		}
		url = filepath.Join(home, url[2:])
	}
	if !filepath.IsAbs(url) {
		url = filepath.Join(worktree, url)
	}
	real, err := filepath.EvalSymlinks(url)
	if err != nil {
		return "", false
	}
	if !isRepository(real) {
		return "", false
	}
	if home, err := os.UserHomeDir(); err == nil && within(real, realPath(home)) && !registered(projects(), real) {
		return "", false
	}
	return real, true
}

func isRepository(path string) bool {
	if fi, err := os.Stat(filepath.Join(path, ".git")); err == nil && fi.IsDir() {
		return true
	}
	if fi, err := os.Stat(filepath.Join(path, "HEAD")); err != nil || fi.IsDir() {
		return false
	}
	fi, err := os.Stat(filepath.Join(path, "objects"))
	return err == nil && fi.IsDir()
}

func registered(projects []string, path string) bool {
	for _, p := range projects {
		if realPath(p) == path {
			return true
		}
	}
	return false
}

func realPath(p string) string {
	if real, err := filepath.EvalSymlinks(p); err == nil {
		return real
	}
	if abs, err := filepath.Abs(p); err == nil {
		return abs
	}
	return p
}

func gitPath(worktree, flag string) (string, error) {
	cmd := exec.Command("git", "-C", worktree, "rev-parse", "--path-format=absolute", flag)
	var stderr strings.Builder
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	p := strings.TrimSpace(string(out))
	if p == "" {
		return "", errors.New("git named no folder")
	}
	return p, nil
}

func sandboxDoc(p sandbox.Policy) *toon.Doc {
	var d toon.Doc
	d.Field("access", p.Mode)
	if p.Mode == sandbox.Open {
		d.Help("Nothing is confined. Give --access boxed to confine the command.")
		return &d
	}
	probe := sandbox.Probe()
	d.Field("backend", probe.Backend)
	d.Field("abi", fmt.Sprint(probe.ABI))
	d.Field("worktree", p.Worktree)
	d.Field("temp", p.Temp)
	rows := make([][]string, 0, len(p.Rules))
	for _, rule := range p.Rules {
		level := "ro"
		if rule.Write {
			level = "rw"
		}
		rows = append(rows, []string{rule.Path, level})
	}
	d.Rows("rules", []string{"path", "access"}, rows)
	d.List("dropped", p.Dropped)
	return &d
}

func cmdSandboxCheck(r *runner, args []string) error {
	if _, err := parse(flags("sandbox check"), args, 0); err != nil {
		return err
	}
	var d toon.Doc
	var c checks
	probe := sandbox.Probe()
	d.Field("backend", probe.Backend)
	if probe.ABI > 0 {
		d.Field("abi", fmt.Sprint(probe.ABI))
	}
	switch {
	case probe.Backend == "none":
		c.add("landlock", "fail", probe.Reason)
	case probe.UnixSockets:
		c.add("landlock", "ok", "")
	default:
		c.add("landlock", "fail", probe.Reason)
	}
	if probe.UnixSockets {
		c.add("unix sockets", "ok", "")
	} else {
		c.add("unix sockets", "fail", "boxed refuses to run here; run a kernel with Landlock ABI 9 or newer, or keep profiles open")
	}
	d.Rows("checks", []string{"check", "state", "fix"}, c.rows)
	d.Help(c.help...)
	if err := r.print(&d); err != nil {
		return err
	}
	if c.failed > 0 {
		return fmt.Errorf("sandbox: boxed cannot be enforced here; follow the fix lines")
	}
	return nil
}
