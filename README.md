# b9s

![Go Version](https://img.shields.io/github/go-mod/go-version/vanderheijden86/b9s?style=for-the-badge&color=6272a4)
![License](https://img.shields.io/badge/License-MIT-50fa7b?style=for-the-badge)

**See what every agent is doing in your [Beads](https://github.com/steveyegge/beads) project, and change it, from your terminal, a browser or your phone.** b9s shows the issues as a tree, a board and a dependency graph, follows changes from `bd` or another agent as they happen, and makes every write through the `bd` CLI. No new tracker: your Beads project as it is.

**Try it in your browser, no install:** [demo.b9s.osen.co](https://demo.b9s.osen.co) opens a made-up web shop, with epics split into features and tasks, that anyone can edit. It resets every 30 minutes.

**Or watch the 90-second demo:** the tree and the board in a terminal, then the same project on a phone over Tailscale.

https://github.com/user-attachments/assets/ccc8ebc9-4daf-4867-8c41-3f91df364f42

| Terminal | Browser | Phone |
|:---:|:---:|:---:|
| ![b9s in a terminal, the issue tree of the sample web shop: epics, features and tasks](docs/screenshot.png) | ![b9s web on a desktop browser, the board of the sample web shop with one swimlane per epic](docs/screenshot-web-board.png) | <img src="docs/screenshot-phone.png" alt="b9s web on a phone, the issue tree of the sample web shop" width="200"> |

[Install](#install) · [First five minutes](#first-five-minutes) · [What you can do](#what-you-can-do) · [Phone and browser](#phone-and-browser) · [Documentation](#documentation) · [Contributing](#contributing)

## Who it is for

b9s is for people who keep their work in Beads and let coding agents work on it. An agent runs `bd create`, `bd update --claim` and `bd close` all day; b9s is the screen you keep open beside it to see what landed, what is blocked and what nobody picked up. It is a keyboard-driven terminal UI modelled on [k9s](https://k9scli.io/), so if you know k9s, you already know most of b9s.

What it is not: b9s is not a tracker and stores nothing of its own. It reads your `.beads` directory, or the Dolt server behind it, and every change is a `bd` command, so b9s, `bd` and your agents always agree on what is stored. The phone view is a page in the phone's browser, not a native app, and it needs the computer that serves it to stay awake. Windows is not supported.

## Install

With Homebrew on macOS or Linux:

```bash
brew install vanderheijden86/tap/b9s
```

On Linux or macOS without Homebrew, the install script downloads the release binary for your platform, checks it against the release checksums and puts it in `~/.local/bin`:

```bash
curl -fsSL https://raw.githubusercontent.com/vanderheijden86/b9s/main/install.sh | bash
```

Set `INSTALL_DIR` to choose another directory. On a platform without a release binary, the script builds from source instead. After that, `b9s --update` installs new releases.

On Debian, Ubuntu, Fedora or RHEL, each [release](https://github.com/vanderheijden86/b9s/releases/latest) also carries `.deb` and `.rpm` packages for amd64 and arm64:

```bash
sudo dpkg -i b9s_<version>_linux_amd64.deb   # Debian, Ubuntu
sudo rpm -i b9s_<version>_linux_amd64.rpm    # Fedora, RHEL
```

From source, with [Go 1.25 or later](https://go.dev/dl/): `git clone https://github.com/vanderheijden86/b9s.git && cd b9s && make install`.

b9s makes every write through the Beads `bd` command, and reads embedded Dolt projects through it too, so `bd` must be on your `PATH`. **b9s is tested with bd 1.2.2 through 1.3.0.** A newer bd usually works. [docs/testing.md](docs/testing.md#bd-releases) says how to check one.

**Tried it?** Tell me what worked and what did not in the [feedback thread](https://github.com/vanderheijden86/b9s/discussions/12). To get an email when a release ships, use Watch → Custom → Releases on the repository.

## First five minutes

Run `b9s` in a folder that has a `.beads` directory:

```bash
b9s
```

1. **Find an issue.** The tree shows every issue under its parent. Move with `j` and `k`, fold with `Tab`, and type `/` to search: `status:open label:backend` or just a word from a title. `o`, `r` and `a` show open, ready or all issues.
2. **Understand its status.** `Enter` opens the detail: status, priority, creator and assignee, the description as Markdown, its children with a progress bar, what blocks it and what it blocks, and the comments. `g` draws the dependency graph around it. `b` opens the board, with one lane per epic.
3. **Change it.** `S` sets the status, `e` edits it in a form, `K` closes it, `Ctrl-N` creates one. Each key runs the matching `bd` command in your checkout, and the row updates when `bd` has written. Run `bd show <id>` in another terminal and you see the same change.

`?` lists every key, and `Ctrl-C` quits.

No Beads project at hand? Download the [sample project](examples/sample-project/), the made-up web shop the [demo](https://demo.b9s.osen.co) runs, with epics split into features and tasks, two releases, blocked work and comments:

```bash
mkdir -p b9s-sample/.beads && cd b9s-sample
curl -fsSL -o .beads/issues.jsonl \
  https://raw.githubusercontent.com/vanderheijden86/b9s/main/examples/sample-project/.beads/issues.jsonl
b9s
```

To open the same project on your phone, run `b9s web` in the same folder and follow [Phone and browser](#phone-and-browser).

## What you can do

Each paragraph names the key to press and links to the [user guide](docs/USER_GUIDE.md), which has every key, gesture, query and setting.

**See the tree.** Epics hold features, features hold tasks, and every issue sits under its parent with its status, priority, assignee and age. `h` and `l` collapse and expand, `t` flattens the tree into a sortable list, `|` picks columns, and `s` sorts. Under Created and Updated a parent ranks by its newest descendant, so an epic moves up when a subtask changes. [Tree view](docs/USER_GUIDE.md#tree-view).

**Follow the agents.** b9s checks the Dolt database hash every 500 ms, so changes from `bd` or another agent appear by themselves, uncommitted ones included. `F` moves the cursor to whatever changed last. JSONL files reload when they change on disk. [Live reload](docs/USER_GUIDE.md#live-reload).

**Query, do not scroll.** `/` searches as you type. Plain words match IDs, titles and labels, and predicates narrow by field: `status:open label:backend`, `type:bug !assignee:andre`. `Tab` completes names, values and labels from the loaded issues. `--filter` starts b9s with a query, which suits shell scripts and tmux bindings. [Search](docs/USER_GUIDE.md#search).

**Work the board.** `b` groups the issues into status columns, one swimlane per epic, with the epic's completion, open P0 and P1 count and last change in a rail on the left. `h` and `l` change the column, `Tab` folds a lane, `z` folds a column into a rail, and `s` regroups by priority or type. [Board view](docs/USER_GUIDE.md#board-view).

**Change many at once.** `Space` marks an issue, `V` marks a range, and `S`, `K` and `Delete` then act on every marked issue, after a confirmation that lists them. [Marks and bulk actions](docs/USER_GUIDE.md#marks-and-bulk-actions).

**Switch projects.** Recent projects sit on `1`-`9`, like favourite namespaces in k9s, and `0` shows them all in one read-only view. `:project` lists every Beads database your Dolt user can read. A project that fails to open leaves the one on screen in place and says why. [Projects](docs/USER_GUIDE.md#projects).

**Know who did what.** Every issue shows its Creator, the `bd` actor that made it, and its Assignee, whoever holds it now. One person often appears as a git name, an email and a lane agent; list them once in `bd config` and b9s shows one name. [Creators, assignees and identities](docs/USER_GUIDE.md#creators-assignees-and-identities).

**Attach files.** `I` attaches screenshots, logs or documents to an issue, and `R` lists and opens them. The bytes go to a local folder or an S3-compatible bucket, and the issue holds a reference comment, so Beads stores no binary data. `b9s attach` does the same from a script. Off until you configure a store. [Attachments](docs/USER_GUIDE.md#attachments).

**Keep b9s on the agent's issue.** `b9s ctl branch <id>` steers the b9s in the same tmux window to an issue. A Claude Code hook calls it on `bd create` and `bd update --claim`, so the tree follows the agent's work. [Steering b9s from another program](docs/USER_GUIDE.md#steering-b9s-from-another-program).

**Make it yours.** `~/.config/b9s/config.yaml` sets the sort, the board design and the poll interval, and `hotkeys.yaml` binds keys to `:` commands in the shape of the k9s file. [Configuration](docs/USER_GUIDE.md#configuration).

**Preview Memory Beads.** With the Memory Beads preview build of `bd`, `M` opens Memories and their Links as wires in the terminal and as a graph in the browser. A released `bd` works as before and says in one line why the view is not available. [Memory Beads preview](docs/USER_GUIDE.md#memory-beads-preview).

### If you know k9s

b9s takes its interaction model from k9s. The header with the logo and the project shortcuts, `Ctrl-E` to hide it. `:` for a command prompt with aliases such as `:epic`, `:bug` and `:project`. `/` for a filter that `Enter` keeps and `Esc` clears. `Space` to mark, with bulk actions on the marks. `Ctrl-F` and `Ctrl-B` to page. A bright full-row cursor, one line of key hints at the bottom, and plain-text tokens instead of emoji. Where k9s lists the pods in a namespace, b9s lists the issues in a project.

## How it reads and writes

b9s reads the `.beads` directory of the current folder, or the one in `BEADS_DIR`, and uses the first source it finds:

| Source | Found through |
|--------|---------------|
| Dolt server | `.beads/metadata.json` with `"dolt_mode": "server"` |
| Embedded Dolt | `.beads/metadata.json` with `"dolt_mode": "embedded"`, the default of `bd init` |
| SQLite | `.beads/beads.db`, from older `bd` versions |
| JSONL | `.beads/issues.jsonl`, also in linked git worktrees |

Embedded Dolt is read by running `bd export`, so b9s never opens the store and your own `bd` writes never wait for it. A Dolt server is read over the MySQL protocol with the user and database from `metadata.json` and the password from `BEADS_DOLT_PASSWORD`, which b9s sends only to a loopback address or to a `host:port` you list in `B9S_TRUSTED_DOLT_ENDPOINTS`, so a cloned repository cannot choose where your password goes. `D` shows the active source and any connection error.

Every write is a `bd` command run in the project's checkout, as your `bd` actor. A project opened from its database alone, without a checkout, is read-only. [Data sources](docs/USER_GUIDE.md#data-sources), [Editing issues](docs/USER_GUIDE.md#editing-issues), [Architecture](docs/ARCHITECTURE.md).

## Phone and browser

`b9s web` serves the same project to a browser: the tree, the board, search, the detail sheet, the dependency graph, and every write the terminal makes, through the same `bd` calls.

```bash
cd ~/code/my-project       # a folder with .beads, as for b9s itself
b9s web                    # serves 127.0.0.1:7979 and prints a pairing link
tailscale serve --bg 7979  # publishes it to your tailnet over HTTPS
```

Open the printed link once on each device: it sets a cookie for 90 days, and the server stays on loopback, so nothing outside your tailnet can reach it. With `qrencode` installed, the link also prints as a QR code. On a phone, swipe a row right to start it and left to close it, long-press to mark, and drag a card between columns. A laptop or iPad gets the whole board at once, with the terminal's keys. A LAN address works too, over plain HTTP, for a network you trust. [Phone and browser](docs/USER_GUIDE.md#phone-and-browser) has the steps, the gestures and the risks.

## Documentation

- [User guide](docs/USER_GUIDE.md): every view, key, gesture, query, setting and command-line option.
- [Architecture](docs/ARCHITECTURE.md): the packages, the data flow and the write path through `bd`.
- [Testing](docs/testing.md): unit, integration and end-to-end tests, and how to check a `bd` release.
- [Migrating embedded Dolt to a server](docs/embedded-to-server-migration.md): for a project that several agents or machines write to at once.
- [Decision records](docs/adr/): why b9s works the way it does.
- [How b9s Works](docs/book/how-b9s-works.epub): a short book on the architecture and design, built from [docs/book](docs/book/) with `make book`.

## Development

```bash
make build                            # builds ./b9s
go test ./... -skip DoltIntegration   # everything that needs no Dolt server
make web                              # npm ci, typecheck, build pkg/web/dist
```

The web UI lives in `web/` (TypeScript, no framework) and builds into `pkg/web/dist`, which is committed so `go build` needs no Node. `make install` refuses to run in a git worktree, so a branch build never replaces the `b9s` on your `PATH`; use `make build` there, or pass `FORCE=1`. Tests named `DoltIntegration` need a Dolt server, and the ones that create databases run only against a disposable local one. [docs/testing.md](docs/testing.md) has the test layers and the Dolt rules.

## Contributing

Pull requests are welcome. To keep review quick:

1. **Open an [issue](https://github.com/vanderheijden86/b9s/issues) first** for anything beyond a typo or a small bug fix. Agree on the change there before you write code, so no work is wasted on a change that will not be merged.
2. **Keep one change per pull request.** A small, focused diff is reviewed sooner than a large one.
3. **Include tests.** A bug fix comes with a test that fails before the fix. `go test ./... -skip DoltIntegration` must pass, and code must be `gofmt`-formatted. Tests never write to a shared Dolt server (see [Development](#development)).
4. **Update the docs** that your change affects: the [user guide](docs/USER_GUIDE.md) for a new key or setting, [docs/](docs/) for new behaviour. The README stays a front page; a test keeps it under 300 lines. A change that goes against a [decision record](docs/adr/) needs a new decision record, not a quiet exception.
5. **Use [Conventional Commits](https://www.conventionalcommits.org/)** for the pull request title, for example `fix(tree): keep the selection after a refresh`.

The maintainer reviews every pull request and merges it as one squashed commit on `main`, with the pull request title as its subject. A pull request can be declined when it does not fit the k9s-style direction of the project, even if the code is good. Contributions are licensed under the project's [MIT licence](LICENSE).

## Acknowledgments

- Jeffrey Emanuel for [beads_viewer](https://github.com/Dicklesworthstone/beads_viewer), the project b9s started from. Parts of its code remain, under the MIT licence.
- Steve Yegge for [Beads](https://github.com/steveyegge/beads).
- The [k9s](https://k9scli.io/) project for the interaction model b9s follows.
- The [Charm](https://charm.sh) team for [Bubble Tea](https://github.com/charmbracelet/bubbletea), [Lip Gloss](https://github.com/charmbracelet/lipgloss), [Bubbles](https://github.com/charmbracelet/bubbles), [Huh](https://github.com/charmbracelet/huh) and [Glamour](https://github.com/charmbracelet/glamour).

## License

MIT. See [LICENSE](LICENSE).
