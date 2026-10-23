# Safe use

Hand runs its agents with their permission prompts skipped, as your own OS user. A profile may opt in to `access: boxed` on Linux, which confines a worker's files and Unix sockets; every other profile runs with no sandbox. This page says what that allows, what to do about it, and what no setup of today prevents. Read it before you start a supervisor.

## What runs without asking

The supervisor and every worker start with the flag that skips permission prompts. No Hand setting turns it off. `hand attempt start` takes `--harness`, `--model`, `--effort` and `--profile`, and no permission option.

| Harness | Flags Hand passes (worker; the supervisor passes the same) |
|---|---|
| Claude Code | `--dangerously-skip-permissions` |
| Codex | `--dangerously-bypass-approvals-and-sandbox --disable hooks --cd DIR` |
| opencode | `--standalone --auto` |
| Antigravity (`agy`) | `--dangerously-skip-permissions` |
| GitHub Copilot (`copilot`; workers only) | `--allow-all-tools --allow-all-paths` |

These come from `internal/harness/harness.go` and `internal/harness/supervisor.go`. The model and effort flags are left out above.

Codex asks whether to trust a folder, and the flag above does not skip that. Hand presses it when the screen names the attempt's own worktree and, if the screen names a repository root, the project's repository: any other folder or repository is left alone and the attempt is noted blocked. Accepting it makes Codex save the trust in `$CODEX_HOME/config.toml` (`~/.codex/config.toml` by default), for good, and for all of your Codex use and not only Hand's workers. For a worker it saves the project's repository, because a worktree is trusted through its main repository. For a supervisor Codex asks only when the fleet home is in a repository, which Hand does not accept, or is itself a repository root (a fleet home Hand did not create), and then it saves the fleet home. Codex then loads that repository's project config, hooks and exec policies whenever you run it there. `hand attempt start` prints what was accepted as `trusted:`, and the attempt records an `attempt.trusted` event with the folder and the repository (`supervisor.trusted` for a supervisor). Hand never edits `config.toml`. To withdraw the trust, delete the `[projects."PATH"]` entry for the path in the event from that file yourself. To decide it yourself instead, run `codex` once in the repository root and choose Trust: Hand then has nothing to press.

One prompt still appears. Claude Code asks before a recognized `rm` or `rmdir` of a critical path, such as `/`, your home folder or the working directory, even with prompts skipped. Its docs say no allow rule or hook approves one, and that the check can miss some commands, so it is not a safety control ([Claude Code permission modes, Critical paths](https://code.claude.com/docs/en/permission-modes.md)). In `auto` and `bypassPermissions` modes the prompt shows a two-minute countdown. If it runs out, Claude Code denies the command. After three unanswered prompts in a session it denies the next ones at once.

What you see in Hand for that prompt:

- Luvus reads the pane as blocked and the watcher records `attempt.blocked`. The supervisor consumes that persisted wake when it is ready and may finish its current turn first. Its skill tells it to answer a permission screen the task needs with `hand attempt keys` and to send anything else to you.
- The Board shows a worker's blocked screen under Needs only when no supervisor runs, or after 10 minutes. It shows the countdown as a badge, then `denying…` at zero. The countdown is Claude Code's. The Board denies nothing.
- A paired phone has the same buttons.

What Luvus reads as blocked for Codex, opencode and agy permission screens, and whether the Board parses them: not verified here. The Board's parser is written for Claude's screens. For Copilot, Luvus 0.14.3 read its trust and permission screens as blocked in one check on Linux; the footer of a worker started with the flags above still says "Manual Approval", although tools and paths run without asking.

Copilot asks whether to trust a folder, and the flags above do not skip that. Hand presses `1. Yes` when the screen names the attempt's own worktree and never `2. Yes, and remember this folder`, which would save the folder in your Copilot config for all your Copilot use. After the trust, Copilot loads the repository's own configuration: hooks (`.github/hooks`, and the hooks in `.claude/settings.json`), MCP servers and skills. It has no flag to turn hooks off, only the `disableAllHooks` setting, so a repository's hooks run in a Copilot worker, unlike `--disable hooks` for Codex. Copilot also does not refuse a model its plan lacks: it prints `Model "X" from --model flag is not available. Using "auto" instead.` and runs on Auto, so a worker meant for another model runs on Auto, which bills differently. For a few seconds after the launch Hand reads the screen above the first prompt line, and when it finds that line it notes the attempt blocked with the model's name; it does not stop the attempt. A line that scrolled off the pane before Hand read it is missed, and a long brief scrolls it off a fraction of a second after Copilot prints it. A signed-out Copilot (`Please use /login to sign in to use Copilot`) is noted the same way.

## What that means

For a profile whose access is `open`, which is the default, a worker can:

- read every file your OS account can read: `~/.ssh`, `~/.config/gh`, `~/.gnupg`, other fleets and each fleet's `board.token`;
- run any command as that account;
- push with any credential in reach, and call any API with any token in its environment;
- be steered by text it reads: an issue, a PR, a review comment, a web page or a file.

A git worktree is not a fence. It is a folder.

## What to do about it

In order of value for cost. The first two matter most.

### 1. Run Hand as its own OS user, or in a VM

Then a worker cannot read your account's private files: your own `~/.ssh`, `gh` login and other folders stay out of reach. A dedicated user is not a VM. World-readable files, shared paths such as `/tmp`, network services and anything else that user may use stay reachable. Use a VM you can throw away if you want stronger isolation: share no folder with the host.

On Linux, as an administrator:

```sh
ls -ld ~
chmod go-rwx ~
sudo useradd --create-home --shell /bin/bash hand-agent
sudo -iu hand-agent
```

`ls -ld ~` shows whether other users can enter your home folder. Skip the `chmod` if it already reads `drwx------`. Otherwise `chmod go-rwx ~` closes it, so the new user cannot read your files. It also locks out anything else that reads your home folder as another user or group, such as a web server or a shared group, so skip it if you rely on that and close the folders that matter instead. As `hand-agent`, install Hand and a harness, and log in to the harness there. The account uses its own harness login, so a worker spends that login's quota.

```sh
s=$(curl -fsSL https://raw.githubusercontent.com/atqamz/hand/main/install.sh) && sh -c "$s"
```

To reach the Board from your own account, print its login link as that user and open it in your browser. Name the fleet folder with `--home` (replace `fleets/demo` with yours): otherwise Hand looks in the current directory. `.local/bin` is the installer's default folder (see [Install](install.md#install-a-release)); adjust the path if you installed elsewhere. The link carries the fleet's token, so keep it out of chats:

```sh
sudo -iu hand-agent .local/bin/hand --home /home/hand-agent/fleets/demo open --print
```

To keep the watcher and the Board running without a login, run `sudo loginctl enable-linger hand-agent`, then see [Autostart](autostart.md).

The commands above are Linux-specific and were not run here. Do not run them on macOS or Windows. On those systems a standard (non-administrator) user can be created in the system settings, but this page gives no verified steps for the account, fleet or autostart setup: not verified here.

### 2. Keep secrets out of reach

Give the account only what a hijacked worker could use without harm to you.

- **No keys with push rights to repositories that matter.** The account's `~/.ssh` and `gh` login are the ones a worker uses. Make new ones for it.
- **A narrow GitHub token.** Log the account in with a fine-grained personal access token limited to the repositories in use, with the Contents and Pull requests permissions and an expiry date. GitHub recommends an expiry but does not require one, so set it. GitHub advises the minimal repositories and permissions ([managing personal access tokens](https://docs.github.com/en/authentication/keeping-your-account-and-data-secure/managing-your-personal-access-tokens)). Pass the token on standard input, so it stays out of your shell history:

  ```sh
  gh auth login --with-token < token.txt
  ```

  Delete `token.txt` afterwards. Do not use a broad `gh auth login` from a browser.
- **A protected default branch.** Require a pull request and approving reviews, and refuse force-pushes, so a hijacked worker can propose a change and cannot land one. GitHub blocks force-pushes on protected branches by default. It exempts people with admin permission unless "Do not allow bypassing the above settings" is on, so give the account write access, not admin ([about protected branches](https://docs.github.com/en/repositories/configuring-branches-and-merges-in-your-repository/managing-protected-branches/about-protected-branches)). Keep the account, and any role, team or app it belongs to, off every bypass list in branch protection and rulesets.
- **Signed commits**, if the repository requires them: GitHub then accepts only verified, signed commits on the branch (same page). Sign with a key used only by that account. A key on a FIDO device (`ssh-keygen -t ed25519-sk`) needs a touch for each signature, and an unlocked signing agent signs whatever a process that can reach it asks. To sign with an SSH key:

  ```sh
  ssh-keygen -t ed25519 -f ~/.ssh/id_ed25519
  git config --global gpg.format ssh
  git config --global user.signingkey ~/.ssh/id_ed25519.pub
  git config --global commit.gpgsign true
  ```

  Run in a throw-away home, these made `git log --show-signature` report a good signature. That GitHub shows the commit as Verified once you add the public key to the account as a signing key: not verified here.
- **A clean environment.** Start fleets from a shell with no cloud or API secrets. When it starts the Luvus server, Hand removes `HAND_HOME`, `CLAUDECODE`, `CLAUDE_CODE_*`, `CODEX_*` except `CODEX_HOME`, and `LUVUS_*` except `LUVUS_HOME`. Its panes inherit the remaining environment.

### 3. Do not register repositories that hold production credentials

A worktree holds a full checkout. A `.env` file, a deploy key or a credentials file in the repository is in the worker's hands. Hand has no allowlist of paths. If a repository has them, keep it out of the fleet.

### 4. Review what workers push

Read a worker's PR before you merge it, including its workflow files, scripts and dependency changes. Rotate any token after a run that looked wrong.

### 5. Isolate fleets from each other

Workers of one fleet can read the other fleets of the same OS user, with their `board.token` and databases. Use one OS user, or one VM, for each group of fleets that must not see each other. `~/.secondhand` is per user, so a second user gets its own fleets.

### 6. Board and phone

- **Private board.** Keep the board on loopback and reach it with `ssh -L` or `tailscale serve` instead of exposing it. [Reach the board from a phone without Tailscale](phone.md) lists the other paths and who can reach the board through each. A LAN address is read-only and sends the token in the clear, and a public hostname needs an access policy in front of it.
- **Pair a phone, do not send the token.** The Board's Phone page signs a phone in with a code that shows only on the laptop, works once and lasts 5 minutes. `hand open --print` puts the fleet's token in a chat or on a clipboard for good. A paired phone has its own session for 90 days and can do everything the board allows over loopback, including answering agents and typing into them, so treat an unlocked paired phone like your laptop. One code can pair a phone with up to 8 fleets, and then a stolen phone reaches all of them: the Phone page ticks only the fleet you are on, so tick another only when the phone needs it.
- **Revoke a lost phone.** Press **Revoke everywhere** beside it on the Phone page, then remove it from your tailnet in the admin console, which cuts its network path. **Revoke everywhere** covers the fleets that browser is signed in to. For a fleet it is not signed in to, open that fleet and revoke there. When the page says a fleet could not be reached, that fleet's record is still active: press the button again when it is back, and do not rely on the revoke alone. Changing `board.token` signs out every phone and every browser of that fleet: do it for a fleet you cannot reach, or when you cannot tell that the revoke arrived.
- **A `copilot` worker's attempt page shows its commands.** The page lists the worker's messages and the first line of each command it ran, so a secret typed into a command line reaches every browser that may open the page, a paired phone included. Never put a credential in a brief that asks for a command line; tool output is not shown.
- **Never Funnel.** Do not publish the board with `tailscale funnel`, a quick tunnel or any public proxy: one session reaches agents with permission prompts skipped. The board refuses requests that Tailscale marks as Funnel traffic, but it cannot tell other public proxies from a tunnel. With `tailscale serve`, allow port 443 on the laptop only to your own user in the tailnet's access rules.
- **Machine names are public.** Turning on HTTPS certificates for `tailscale serve` writes the machine and tailnet names (`laptop.tailnet.ts.net`) to the public certificate transparency logs. Rename a machine whose name reveals anything before you enable it.

### Boxed access (Linux, opt-in)

A routing profile may say `"access": "boxed"`. `hand attempt start` then launches the harness through `hand sandbox exec`, which applies a Landlock policy to the worker and replaces itself with the harness, so the confined harness is the pane's root process. The launch line recorded on the attempt shows the exact wrapper. A profile without the field, or with `"access": "open"`, launches exactly as before.

Boxed needs a Linux kernel whose Landlock can deny Unix-socket connects (ABI 9). Where it cannot, `hand sandbox exec` and `hand attempt start` exit 2 and run nothing; they never fall back to an unconfined launch. macOS and Windows are unavailable, and `hand doctor` grows a `sandbox` row only when a profile asks for boxed. The supervisor is never boxed, because it needs the Luvus socket.

What a boxed worker can reach:

| Reachable | Why |
|---|---|
| the attempt's worktree, read and write | the worker's code |
| a private temporary folder for the attempt, `TMPDIR`, `TMP` and `TEMP` | build scratch space, set to `tmp/<worktree>` under the fleet home |
| the project's git directory, read-only, except `objects`, `refs`, `logs`, the attempt's own admin folder under `worktrees` and the lock files git writes | commit, rebase, fetch and push work; `config`, `hooks` and `info` cannot be rewritten. The folders come from the fleet's project record and its admin folder, not from the worktree's `.git` file, which a worker can rewrite |
| system folders, `/proc`, `/sys`, device files including `/dev/ptmx`, and named tool caches, read and write | running commands and fetching dependencies. The caches are `~/.cache/{mise,uv,go-build,pip,node-gyp}`, `~/.local/state/uv`, `~/.npm`, `~/.bun/install/cache`, `~/.cargo/{registry,git}`, `~/.rustup/{downloads,tmp,update-hashes}`, `~/go/pkg` and `~/.local/share/pnpm/store`; mise's state folder is read-only except `hints`, `incomplete-installs` and `tool-purgatory.json`, so a worker cannot mark its own `mise.toml` as trusted, and `~/.local/share/pnpm` and `~/.cargo/{config.toml,config}` are readable. `~/.cargo` is not granted whole, so `~/.cargo/credentials.toml` stays out. These caches are shared with your builds outside the box, so a worker can leave content there that a later build runs; per-attempt caches would change that, and are not built. mise installs, mise shims, `~/go/bin`, `~/.cargo/bin`, `~/.bun/bin`, `~/.rustup/toolchains` and `~/.local/bin` stay read-only, so installing a new toolchain binary inside the box usually fails |
| the harness's own login and config folders, read and write | Claude Code's `~/.claude` and `~/.claude.json`, Codex's `$CODEX_HOME` or `~/.codex`, opencode's config/data/cache/state folders, agy's `~/.gemini` and Antigravity config. opencode's data and state folders are granted only when the harness is opencode |
| local git remotes, read-only, with `objects`, `refs` and `logs` writable, when the remote is a bare repository or a registered project | pushing to a bare or project repository on the same machine; the list comes from the project's git folder, and the remote's `hooks` and `config` cannot be rewritten, so other folders under your home stay out |
| pathname Unix sockets inside that policy | and no others: abstract Unix sockets and signals are scoped to the sandbox |

Folders that do not exist yet are created (mode 0700) before the policy names them, so a first boxed run can write its harness folder or cache.

mise trusts a linked worktree once the project's main checkout is trusted, so a worker's `mise.toml` can be loaded by a later `mise` run outside the box even though the worker cannot write mise's trust records: read that file before running mise in the worktree.

The environment is rebuilt by a deny list. `LUVUS_*`, `HAND_LUVUS_*`, `SSH_AUTH_SOCK`, `SSH_AGENT_PID`, `DBUS_SESSION_BUS_ADDRESS`, `DBUS_SYSTEM_BUS_ADDRESS`, `GH_TOKEN`, `GITHUB_TOKEN` and `GPG_AGENT_INFO` are dropped, so the worker does not see the Luvus socket, the ssh-agent or the desktop bus, and a token passed that way does not leak. File descriptors above 2 are marked close-on-exec before the harness starts; when one cannot be marked, the launch exits 2 and runs nothing.

What it does not protect:

| Not protected | Detail |
|---|---|
| the network | TCP and UDP are open. A worker can reach the internet, any service on localhost, and any credential that remains in its environment or in a granted folder |
| what the policy grants | a hijacked worker can spend the harness login's quota, poison the toolchain caches and config it may write, and rewrite the code in its worktree. Use a separate harness login for Hand |
| the supervisor | it needs the Luvus socket to drive its workers, so boxing it is a separate problem |
| prompt injection | it still works through everything the policy grants. Boxed narrows the account and the fleet; it does not make a hostile page or issue safe |
| egress control, an audit log | Hand has neither |

`~/.ssh`, `~/.config/gh`, `~/.gnupg`, other fleets, other attempts' worktrees and the Luvus session folder are outside the policy, which is the point. `hand sandbox exec --print --access boxed --worktree DIR --harness NAME` prints the resolved rules and `network: open` without running anything, and `hand sandbox check` reports the backend and what it can enforce on this machine. Running a command needs `--access` and a `--` before it; without either, `hand sandbox exec` refuses rather than running unconfined.

The fleet database and `board.token` are outside the policy too, so a boxed worker cannot open them. That also means it cannot run `hand report add` yet: the report outbox (#965) gives a boxed attempt a private folder to write instead. Until then, have a boxed worker leave its report in the worktree and let the unboxed supervisor file it.

## What Hand checks before running git

Hand runs git in your projects: `git fetch` and `git worktree add` to start an attempt, and `git status` and `git worktree remove` to clean one. Some settings in a project's git configuration name programs that git runs, for example `core.sshCommand`, a filter's `smudge`, a credential helper, a config-defined hook or an included file. Hand records the values of those settings when a project is registered (a project registered by an earlier version records them the first time Hand runs git in it) and refuses to run git in the project if they have changed since: it runs nothing, exits 2 and names the settings that changed. Read them with `git -C REPO config --list --show-origin`, then accept them with `hand project trust PROJECT`. `hand doctor` has a row for each project.

The check covers the repository's own configuration (with the files it includes and a worktree's own `config.worktree`) and `.git/info/attributes`. It does not cover the global or system git configuration, the environment, a submodule's own configuration or the programs on your `PATH`. One command runs git without the check: `hand report add` from a worktree without `--attempt` runs `git rev-parse --show-toplevel`. It is not a sandbox, so the account separation above stays the boundary. Reading the settings and running the command are two steps, so a change made between them is caught by the next command. A project registered by an earlier version accepts the settings it has when Hand first runs git in it: read `git -C REPO config --list --show-origin` once after you upgrade.

### What Hand runs to list models

Hand lists the models of each harness with one command each, with no prompt and no model call, so it spends no tokens:

| Harness | Command | What Hand keeps |
|---|---|---|
| Claude Code | `claude -p --input-format stream-json --output-format stream-json --verbose --no-session-persistence`, with one `initialize` request on stdin | the `models` array of the answer: name, display name, supported efforts. The answer also holds `account` data and the command list; Hand's decoder declares neither and drops them |
| Codex | `codex debug models --bundled`, else a read of `$CODEX_HOME/models_cache.json` | slug, display name, visibility, effort levels. Not `identity`, `etag` or any instruction text |
| opencode | `opencode models --standalone` | model names, one per line. Never without `--standalone`, which leaves a server running; never `api`, `auth export` or `debug config`, which can print a key |
| Antigravity (`agy`) | `agy models` | id and label |
| GitHub Copilot (`copilot`) | `copilot help config` | the quoted names under `model`, and `auto`. The page also describes every setting; Hand reads nothing else from it |

Each is also run as `<harness> --version`, whose first line is kept. Every run has a timeout (5 to 30 seconds), a 4 MiB output limit, no terminal, and its process group is killed when it ends. It gets only `PATH`, the home folders and the variables its own harness reads (`ANTHROPIC_*`, `CLAUDE_*`, `CLOUD_ML_*` and the AWS region and profile for Claude Code, `CODEX_*`, `OPENCODE_*`, `COPILOT_*`), not the rest of your environment. Claude Code runs in the fleet home, or the current folder, so its project settings apply. Its stderr is read only for agy's sign-in line and is never printed or stored. Hand reads no `settings.json`, `config.toml`, `opencode.json` or `auth.json`: it only compares the size and modification time of a few of them to know when a list is stale. Nothing is cached but names, labels, efforts and sources, in `~/.secondhand/models/` (folder mode 0700, files 0600): a token, an API key, an account email or an organisation reaches neither that folder, an error, nor a log. If a harness prints one in the part Hand decodes, the decoder keeps it out; tests feed each reader an invented secret and check that it appears nowhere.

Claude Code may reach the network on its own while it answers (an authenticated `GET /v1/models` when you turned on gateway model discovery, and its fast-mode check): that is what every Claude Code start does, with the variables it gets here, and not verified here for a subscription login.

## What no setup of today prevents

- **Prompt injection that uses access you gave.** A worker that reads hostile text in an issue, a PR or a page can be persuaded to use the repositories, tokens and network it already has. Steps 1 to 4 limit what it can reach. None of them stops it from writing bad code inside that reach, and a prompt would not stop it either: Claude Code's own docs say `bypassPermissions` "offers no protection against prompt injection or unintended actions".
- **One agent typing into another.** Every pane's environment carries `LUVUS_SOCKET_PATH` and `LUVUS_API_ADDRESS`. A worker, or the supervisor, can list the fleet's panes through the Luvus socket and send keys or prompts to the others, including the one that shows a permission prompt. On the same host and account, a permission prompt is friction for a hijacked agent, not a boundary. Hand does not prevent this. Only a separate OS user or VM does.
- **Programs git runs for you.** Hand now refuses to run git in a project whose program-naming settings changed. The check has the gaps listed under [What Hand checks before running git](#what-hand-checks-before-running-git), and it does not stop a worker that already runs as your account.
- **A compromised worker on your own account.** Without steps 1 and 2, it has all of your account's access.
- **Isolation between open workers of one OS user**, egress control, protection of `board.token` from an open worker, and an audit log. Hand has none of these. Boxed confines a worker's files and Unix sockets; it leaves the network open.

## Planned, not built

None of this exists yet. The operator's safe-mode study describes them. It is not in this repository. Which of them get built is decided after this page has been read.

- An opt-in permission mode per routing profile, so a worker can start in a Claude Code mode that is not bypass.
- An approval flow in the Board: approve once, approve always or deny, with a timeout.
- Read-only roles for review and research workers.
- A macOS or Windows backend for boxed, network filtering, and boxing the supervisor (which needs the Luvus socket).

Until then, the setup above is the only protection.

## What each harness offers instead

Hand passes the flags above and none of these. Each is the harness's own feature, from its documentation as fetched on 2026-10-22 (UTC). Not verified here: that any of them works under Hand.

| Harness | What Hand passes | What the harness offers instead |
|---|---|---|
| Claude Code | `--dangerously-skip-permissions` | Modes `default`, `acceptEdits`, `plan`, `auto` (a classifier reviews each action), `dontAsk` and `bypassPermissions`. Allow, ask and deny rules. A Bash sandbox on Linux and macOS, none on native Windows. [Modes](https://code.claude.com/docs/en/permission-modes.md), [rules](https://code.claude.com/docs/en/permissions.md), [sandbox](https://code.claude.com/docs/en/sandboxing.md) |
| Codex | `--dangerously-bypass-approvals-and-sandbox --disable hooks --cd DIR` | `--sandbox` with `read-only`, `workspace-write` or `danger-full-access`, and approval policies. The Auto preset uses workspace-write and approval on request, with network off by default. Codex recommends it for version-controlled folders and read-only for others. [Agent approvals and security](https://developers.openai.com/codex/agent-approvals-security) |
| opencode | `--standalone --auto` | Per-tool `allow`, `ask` and `deny` with patterns, per agent. Without `--auto` the defaults already allow most tools. `--auto` approves whatever is not denied. [Permissions](https://opencode.ai/docs/permissions/) |
| Antigravity (`agy`) | `--dangerously-skip-permissions` | `permissions.allow`, `ask` and `deny` in its settings file, where deny wins over ask and ask over allow. Commands ask by default. `agy --help` also lists `--mode` and `--sandbox`: the docs page does not describe them, not verified here. [Permissions](https://antigravity.google/docs/permissions) |
