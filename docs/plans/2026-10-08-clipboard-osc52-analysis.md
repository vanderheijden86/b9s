# Clipboard Fallback Through OSC 52: Analysis and Roadmap (draft)

Beads task: bd-saau. GitHub issue: [#23 Clipboard Fallbacking](https://github.com/vanderheijden86/b9s/issues/23).

Status: draft. This document feeds the reply to the contributor and a later ADR. It decides nothing yet.

## Contents

- [What the issue asks for](#what-the-issue-asks-for)
- [What b9s does today](#what-b9s-does-today)
- [Constraints the issue does not mention](#constraints-the-issue-does-not-mention)
- [Prior art: k9s](#prior-art-k9s)
- [Options](#options)
- [Comparison](#comparison)
- [Recommendation](#recommendation)
- [Roadmap](#roadmap)
- [Open questions for the owner](#open-questions-for-the-owner)

## What the issue asks for

The reporter runs b9s in a container, a Proxmox VM and over SSH. The copy keys (`y` for the ID, `c` for ID and title, the Markdown copy in the detail pane) fail with:

```
Clipboard error: No clipboard utilities available. Please install xsel, xclip, wl-clipboard or Termux:API add-on...
```

Installing `xclip` does not help: it needs an X or Wayland display, and a headless host has none.

The request is OSC 52: write `ESC ] 52 ; c ; <base64> BEL` to the terminal, and the terminal emulator on the user's own machine puts the payload on the local clipboard. The bytes travel through SSH, docker attach and tmux like any other output, so no utility on the remote side is involved.

The reporter proposes:

1. A small clipboard provider interface in front of the copy actions.
2. OSC 52 when `config` says so, or when `DISPLAY` and `WAYLAND_DISPLAY` are both empty.
3. The current utility-based path otherwise.

They offer to write the PR and ask for the preferred architecture first.

## What b9s does today

```
  key y / c / copy-issue ──▶ pkg/ui/model.go ──▶ atotto/clipboard.WriteAll
                                                       │
                                        ┌──────────────┼──────────────┐
                                        ▼              ▼              ▼
                                     pbcopy        xclip/xsel/      Windows
                                     (macOS)       wl-copy (Linux)  API
```

- `github.com/atotto/clipboard` v0.1.4 does the work. On Linux its `init()` picks `wl-copy`, `xclip`, `xsel` or `termux-clipboard-set` by `LookPath`, and `WriteAll` returns `missingCommands` when none exists. When `xclip` exists but `DISPLAY` is empty, `xclip` itself exits non-zero and that error comes back instead. Either way the failure is an error value at the call site, which is the natural hook for a fallback.
- Five call sites in `pkg/ui/model.go` write to the clipboard. Two (`copyIssueToClipboard`, `copyTreeIssueIDAndTitle`) go through the injectable `m.clipboardWrite` field that tests use. Three (`y` in board and list, the copy in the DB health popup) call `clipboard.WriteAll` directly. Any provider work starts by routing all five through one function.
- Each call site sets `statusMsg` to "Copied ... to clipboard" or "Clipboard error: ...". The message wording is per site, which a provider refactor should also fold into one place.
- The web UI (`cmd/b9s/web.go`) copies through the browser and is not affected.

Libraries already in the module:

| Library | Version | Relevance |
|---|---|---|
| `charmbracelet/bubbletea` | v1.3.10 | No clipboard command. `tea.SetClipboard` exists only in v2. |
| `muesli/termenv` | v0.16.0 (direct) | `termenv.Copy(str)` writes OSC 52 to its output, wraps for `screen` when `TERM` starts with `screen`. |
| `aymanbagabas/go-osc52/v2` | v2.0.1 (indirect, vendored) | Builds the sequence, with `Screen()`, `Tmux()`, `Primary()` and `Limit()` modes. |
| `charmbracelet/x/ansi` | vendored | `ansi.SetSystemClipboard(base64)` returns the bare sequence. |

So the sequence itself costs nothing: no new dependency, and three vendored ways to produce it.

## Constraints the issue does not mention

### The renderer owns stdout

Bubble Tea v1 renders frames from its own goroutine under a mutex. `Update` runs on the event loop goroutine. An OSC 52 write from `Update` straight to `os.Stdout` can land in the middle of a frame flush. A terminal parses the byte stream in order, so renderer bytes inside the OSC payload corrupt the base64 and the copy, and OSC bytes inside a frame can leave a stray glyph.

- For a short ID (under one write, far under `PIPE_BUF`) the race window is tiny and the write is one syscall.
- For the Markdown issue copy (kilobytes) the write can split across syscalls and the window is real.

Ways out, from cheap to thorough:

1. One `Write` call with the whole sequence, accept the small residual risk for short payloads, and measure it in the PTY E2E harness.
2. Route the write through the renderer. v1 exposes no public hook for raw output, so this means a fork or a `tea.Println`-style trick that adds a line, which is not acceptable.
3. Bubble Tea v2, where `tea.SetClipboard(text)` is a command the renderer serialises. That is a migration of bubbletea, bubbles, lipgloss and huh together, and far larger than this feature. Worth a separate ticket, not a precondition.

### OSC 52 is fire and forget

The terminal never answers a set-clipboard sequence. b9s cannot know whether the copy landed. A status line that says "Copied" after an OSC 52 write is a claim, not an observation. The honest message is "Sent to terminal clipboard (OSC 52)". The utility path keeps "Copied", because an exit status backs it.

### Terminal support is uneven

| Terminal | OSC 52 write | Note |
|---|---|---|
| Ghostty, WezTerm, Alacritty, foot | yes | on by default |
| iTerm2 | yes, off by default | "Applications in terminal may access clipboard" in Preferences |
| kitty | yes | default `clipboard_control` allows writes |
| Windows Terminal | yes | recent versions |
| tmux | passes through | default `set-clipboard external` forwards to the outer terminal; no wrapping needed on tmux 3.3+ |
| screen | needs DCS wrapping | `go-osc52` `Screen()` mode |
| macOS Terminal.app | no | `pbcopy` works there, so the utility path must stay first on macOS |
| GNOME Terminal and other VTE | no or recent only | needs a check per VTE version before claiming support |

Two consequences. A "OSC 52 first" policy silently breaks copy in Terminal.app and VTE, where the utility path works today. And payload limits differ: tmux and some emulators cap the base64 string, so the Markdown copy may be cut or dropped on some setups.

### The `DISPLAY` heuristic is weaker than the error

The proposal switches on `DISPLAY` and `WAYLAND_DISPLAY` being empty. The error from the utility path carries the same information with no guessing: no utility installed, or utility present but no display. Switching on the error also covers macOS (no `DISPLAY`, but `pbcopy` works) and Windows (no `DISPLAY`, native API works) without an OS check. One case the error does not cover: SSH with X forwarding sets `DISPLAY`, and `xclip` then copies to the remote X server instead of the user's machine. That is a reason for an explicit setting, not for a heuristic.

### Tests

- Unit tests already inject `m.clipboardWrite`. A provider interface makes that injection the normal path rather than a test-only field.
- The E2E harness runs b9s through the Unix `script` command in a PTY. CI has no `xclip`, so `y` fails there today. With the fallback, an E2E test can press `y` and assert that the captured output contains `\x1b]52;c;<base64 of the ID>\a`. That is a real end-to-end check of the feature with no clipboard involved, and it is also the measurement for the interleaving risk above.
- `B9S_TEST_MODE` must not change clipboard behaviour, or the E2E test checks nothing.

## Prior art: k9s

k9s had the same report (derailed/k9s#3740, #3646) and merged the same fix in [derailed/k9s#3902](https://github.com/derailed/k9s/pull/3902), shipped in v0.51.0. It is 120 lines in `internal/view/clipboard.go`:

- Same library, `atotto/clipboard` v0.1.4, same error.
- One `clipboardWrite(text)`. Mode from the env var `K9S_CLIPBOARD`: `auto` (default), `native`, `osc52`. `auto` tries native first and sends OSC 52 only when native returns an error. No `DISPLAY` heuristic.
- OSC 52 is refused when stdout is not a tty or `TERM=dumb`.
- Base64 longer than 74994 bytes is an error (`K9S_OSC52_MAX` overrides). That is the tmux limit.
- DCS wrapping when `$TMUX` is set (needs `allow-passthrough on`) and when `TERM` starts with `screen`. Bare sequence otherwise.
- One `os.Stdout.WriteString`. tview renders from another goroutine too, and they accepted the risk.
- A successful OSC 52 write reports as a normal copy.

Where b9s follows k9s: the `auto` rule, the tty and `TERM=dumb` check, the payload cap, one write. Where it differs: the mode lives in `config.yaml` rather than an env var, because b9s validates settings at load, and tmux gets the bare sequence, because tmux's default `set-clipboard external` forwards it and `allow-passthrough` is off by default.

## Options

### A. Utility first, OSC 52 on error (the issue's shape, without the env heuristic)

Try the utility path. When it returns an error, write OSC 52 and report "Sent to terminal clipboard". Nothing changes for anyone whose copy works today.

- Pro: zero behaviour change on working setups. One decision rule, derived from a fact (the error) rather than a guess. Smallest diff.
- Con: the SSH-with-X-forwarding case copies to the wrong machine. A user on a terminal without OSC 52 and without utilities gets a "Sent" message that did nothing, with no explanation.

### B. OSC 52 first, utility on nothing

Always write OSC 52. Some tools (Neovim with `SSH_TTY` set, yazi, zellij) lean this way.

- Pro: one path everywhere, works across every hop, no `exec`.
- Con: silent regression in Terminal.app and VTE, where today's copy works. Fire and forget means b9s cannot tell when to fall back, so "utility on nothing" cannot be implemented. This option is really "OSC 52 only".

### C. A `clipboard` config section with a provider choice

```yaml
clipboard:
  provider: auto   # auto | system | osc52 | command
  command: ["wl-copy"]   # only with provider: command
```

`auto` is option A. `system` and `osc52` force one path. `command` pipes the text to a user command, which covers every case the other two miss (a custom bridge, a remote clipboard daemon, `wl-copy` with flags).

- Pro: the SSH-with-X case and the Terminal.app case both have an explicit answer. `command` is the escape hatch every TUI with a clipboard eventually grows (lazygit's `os.copyToClipboardCmd` is the reference). Fits the existing `config.yaml` style, with validation at load like `ui.sort`.
- Con: more surface to document and test. `command` runs an arbitrary program from config, which needs the same care as the editor and attachment settings.

### D. Both at once

Write OSC 52 and run the utility on every copy. Report the utility's result when it ran, "Sent" otherwise.

- Pro: the clipboard is set on whichever side can hear.
- Con: on a laptop every copy also emits a sequence, and in a terminal that caps or logs OSC it is noise. Adds the interleaving risk to every copy, not only the fallback case. Hard to explain in the README.

### E. Bubble Tea v2 and `tea.SetClipboard`

The clean implementation of the write itself.

- Pro: no renderer race, framework-maintained.
- Con: a whole-stack migration (bubbletea, bubbles, lipgloss, huh, glamour). Not this feature. File it as its own epic and let the provider abstraction hide which writer is behind OSC 52, so the swap later is one function.

## Comparison

| | A utility-first | B OSC 52 first | C provider config | D both | E v2 |
|---|---|---|---|---|---|
| Fixes the reported case | yes | yes | yes | yes | yes (eventually) |
| Behaviour change for working setups | none | regression on Terminal.app, VTE | none by default | extra output | none |
| Handles SSH with X forwarding | no | yes | yes (`osc52`) | yes | n/a |
| Honest status message | yes | "Sent" only | yes | mixed | yes |
| Renderer race exposure | fallback case only | every copy | fallback case only | every copy | none |
| Diff size | small | small | medium | small | large |
| Testable in PTY E2E | yes | yes | yes | yes | yes |

## Recommendation

**Option C with option A as its `auto` default, shipped in two steps.** Step one is option A behind a `clipboard.Writer` abstraction and is a good first PR for the contributor. Step two adds the config section. Option E gets its own ticket and does not block either step.

Shape of the code:

```
  pkg/ui/model.go ──▶ m.clipboard.Write(text) ──▶ pkg/clipboard
                                                      │
                               ┌──────────────────────┼─────────────────┐
                               ▼                      ▼                 ▼
                         systemWriter           osc52Writer        commandWriter
                       (atotto, exit code)   (one Write to stdout)  (exec, stdin)
                               │
                               └── autoWriter: system, then osc52 on error
```

- `pkg/clipboard` is new and owns the three writers and `auto`. It returns a result that says which path ran, so the UI can word the status line ("Copied" or "Sent to terminal clipboard (OSC 52)").
- All five call sites in `pkg/ui/model.go` go through `m.clipboard` and one status helper. The test hook stays, as the same interface.
- The OSC 52 writer builds the sequence with `go-osc52/v2` (already vendored), applies `Screen()` when `TERM` starts with `screen`, does not wrap for tmux (default `set-clipboard external` forwards it), and writes with one call.
- Config keys follow the existing style: `clipboard.provider` validated in `UnmarshalYAML` with an error that lists the accepted values, like `ui.sort.field`.
- README key tables and `docs/USER_GUIDE.md` get one paragraph on the fallback and the iTerm2 setting, and the troubleshooting entry for "No clipboard utilities available" points at `provider: osc52`.

Why not the env heuristic: the error is the discriminator, and it already covers macOS and Windows. Why not OSC 52 first: it regresses terminals that work today and b9s cannot detect that. Why a `command` provider: it is the one setting that answers every future "my clipboard is special" issue without another release.

## Roadmap

| Step | Scope | Ticket | Owner |
|---|---|---|---|
| 1 | `pkg/clipboard` with `system`, `osc52`, `auto`; route all five call sites; one status helper; unit tests per writer; one PTY E2E test asserting the OSC 52 bytes after `y` with no utility on PATH | to file under a new feature | contributor (offered) |
| 2 | `clipboard:` config section with `provider` and `command`, validation, README and user guide | to file | contributor or owner |
| 3 | Measure the interleaving risk: a PTY test that copies the Markdown of a large issue repeatedly and checks the base64 decodes | to file | owner |
| 4 | Bubble Tea v2 migration, with `tea.SetClipboard` replacing the OSC 52 writer | separate epic | owner |

Step 1 alone closes the issue as reported. Steps 2 and 3 can follow in the same release or the next. ADR: write it when step 1 lands, with `status: active`, recording the `auto` rule and the honest status wording. This document is the draft for it.

## Open questions for the owner

1. Is "Sent to terminal clipboard (OSC 52)" acceptable as a status line, or should the fallback say "Copied" too? The analysis prefers the honest wording.
2. Should the `command` provider be in scope for the first PR, or wait for someone to ask for it?
3. Does any owner setup use tmux with `set-clipboard off`? That is the one tmux case that needs the `Tmux()` wrapping and `allow-passthrough on`.
4. Accept the contributor's PR for step 1, or implement in-house and keep the contributor on review? The issue reads as a competent proposal and the shape is agreed up to the env heuristic.
