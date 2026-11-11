# bd-rz2i implementation report

Result: implemented and checked. No unresolved required-check failures. Independent review and testing remain with Polly. No delivery commit, delivery push, or pull request.

## Git preflight

- Shared repository: /home/omnigent/workspace/b9s
- Actual branch: omnigent/bd-rz2i
- HEAD and fetched origin/main: eb77cef9f8e68e9854f3c1b6ec6a6074b812830a
- Assigned base and PR target: main
- Initial worktree clean, local delivery branch absent.
- `timeout 60s git -C /home/omnigent/workspace/b9s fetch origin main` exited 0.
- `git -C /home/omnigent/workspace/b9s show-ref --verify refs/remotes/origin/main` found the assigned base.
- `timeout 60s git -C /home/omnigent/workspace/b9s ls-remote --exit-code --heads origin refs/heads/omnigent/bd-rz2i` exited 2, remote delivery absent.
- `git -C /home/omnigent/workspace/b9s switch -c omnigent/bd-rz2i origin/main` created the branch from the fetched assigned base.
- Required commits: none. Required-commit ancestry against base and HEAD: not applicable.
- `lane-checkpoint inspect --repo /home/omnigent/workspace/b9s --task bd-rz2i --base origin/main` found no saved checkpoint.
- Final HEAD and base remain unchanged. Exactly two tracked files changed.

## Changes

- pkg/ui/model.go: join rendered key and description blocks horizontally at their top edge, so description continuations retain the key-column indent. Use text glyphs without emoji presentation selectors in the title, Hotkeys heading and Detail heading. These selectors had different cell widths under grapheme and terminal width methods.
- pkg/ui/help_search_test.go: regression for wrapped custom-command alignment and full continuation text. Regression for consistent terminal cell widths with the original Ctrl-T hotkey present.

The native baseline capture reproduced missing heading text and leftover marks from the prior screen. The heading-width test independently reproduced a 160 versus 159 cell discrepancy. The alignment test reproduced command continuation text entering the key column.

Documentation and ADR changes were unnecessary because this changes rendering only. Existing shortcut behavior and help contents remain the same. The regression tests guard both defect classes. No files were deleted.

## Verification

All commands below ran from /home/omnigent/workspace/b9s unless an absolute project path is included. Counts for Go suites are top-level test-function outcomes, with nested outcomes separately reported.

| Exact command | Exit | Passed | Failed | Skipped | Evidence |
| --- | --- | --- | --- | --- | --- |
| `timeout 180s go mod download` | 0 | N/A | N/A | N/A | dependencies.txt |
| `timeout 180s go build ./...` | 0 | N/A | N/A | N/A | build.txt |
| `timeout 180s go vet ./...` | 0 | N/A | N/A | N/A | vet.txt |
| `timeout 120s go test ./pkg/ui/ -run '^(TestHelpOverlay|TestHelpSearch|TestHotkeysAppearInFooterAndHelp|TestCommandHelp)' -count=1 -timeout=60s -json` | 0 | 15 | 0 | 0 | green.jsonl, green-summary.json |
| `GOFLAGS=-v timeout 660s make test-stable` | 0 | 2127 | 0 | 18 | stable.log, stable-summary.json |
| `timeout 120s go build -o /tmp/bd-rz2i/b9s-after ./cmd/b9s` | 0 | N/A | N/A | N/A | build-capture.txt |
| `timeout 30s python3 /tmp/bd-rz2i/capture.py /tmp/bd-rz2i/b9s-after original` | 0 | 4 | 0 | 0 | capture-original.txt |
| `B9S_CAPTURE_LONG=1 B9S_CAPTURE_CONFIG=/tmp/bd-rz2i/long-config timeout 30s python3 /tmp/bd-rz2i/capture.py /tmp/bd-rz2i/b9s-after long` | 0 | 4 | 0 | 0 | capture-long.txt |
| `git -C /home/omnigent/workspace/b9s diff --check` | 0 | N/A | N/A | N/A | final-checks.txt |
| `gofmt -l /home/omnigent/workspace/b9s/pkg/ui/model.go /home/omnigent/workspace/b9s/pkg/ui/help_search_test.go` | 0, empty output | N/A | N/A | N/A | final-checks.txt |

Stable suite ran alone on the delivery working tree, with no base comparison. Make sets `CGO_CFLAGS=-DSQLITE_ENABLE_FTS5` and `go test -timeout 10m` with its committed exclusion list. Including nested subtests, the measured results were 3771 passed, 0 failed, 20 skipped. The 18 top-level skips are optional real-binary probes, S3, Dolt, opt-in Memory research and stress tests. Two additional nested skips need absent external real-data fixtures. Every skip and its reason is in stable-summary.json. Interrupted and unrun tests within the selected, completed suites: 0. Makefile-excluded tests and tests/e2e are outside the selected stable suite. No flaky outcomes observed or retried suite tests.

## Red runs and resolved check failures

- Initial `timeout 120s go test ./pkg/ui/ -run '^TestHelpOverlayWrappedDescriptionsStayInColumn$' -count=1 -timeout=60s -json` exited 124 during cold dependency compilation. No tests started. Evidence incomplete, never treated as passing: red.jsonl, red.exit.
- The identical focused command after the compilation cache populated exited 1, 0 passed, 1 failed, 0 skipped. It failed because `(:epic sta` appeared in the key column: red-retry.jsonl, red-retry.exit.
- `timeout 120s go test ./pkg/ui/ -run '^TestHelpOverlayTerminalCellWidthsAgree$' -count=1 -timeout=60s -json` exited 1, 0 passed, 1 failed, 0 skipped. It failed for differing heading cell widths: red-width.jsonl, red-width.exit.
- Baseline binary command `timeout 180s go build -o /tmp/bd-rz2i/b9s-before ./cmd/b9s` exited 0.
- `timeout 30s python3 /tmp/bd-rz2i/capture.py /tmp/bd-rz2i/b9s-before before` exited 1 on the missing title assertion. The real screen reproduced stray marks. Evidence: capture-before.txt, before-screen.txt, before-screen.png.
- Updated-binary capture attempts with labels `after`, `final` and `checked` each exited 1. The first harness indexed Unicode code points as terminal cells. Two later attempts waited for exit without draining the PTY, using exit keys unsuitable for the active help state. Their evidence is capture-after.txt, capture-final.txt and capture-checked.txt. Harness changes use terminal cells, the project's declared 3500ms auto-close, and a five-second drain deadline. No product source changed after the stable run.
- Capture label `proof` then exited 0: capture-proof.txt. Final measured captures `original` and `long` each exited 0 with four explicit checks. No unresolved required-check failures or issue IDs to hand off.

## Real TUI evidence

Both final capture runs used the built binary in a real PTY, 160 columns by 40 rows, against an isolated copy of tests/testdata/minimal.jsonl. Config and data stayed under /tmp/bd-rz2i. No shared database writes and no browser/editor launches.

Each final run checked top borders and hotkey visibility, scrolling to the last content, filtered hotkey containment/alignment, and bounded normal exit. Original description: `Open epics`. Longer description: `Open epics and related issues`, with command `epic status:open`. The wrapped command remains entirely below the description, with a blank key column.

Visually inspected original top, long filtered hotkey, and original bottom captures. Headings and original command are complete, borders align, and the right-side marks are absent. Vertical scrolling remains intentional for help taller than the viewport.

Evidence paths:

- /tmp/bd-rz2i/original-top.png
- /tmp/bd-rz2i/original-bottom.png
- /tmp/bd-rz2i/original-hotkey.png
- /tmp/bd-rz2i/long-top.png
- /tmp/bd-rz2i/long-bottom.png
- /tmp/bd-rz2i/long-hotkey.png
- Corresponding .txt files preserve terminal screen contents.
- /tmp/bd-rz2i/capture.py contains the bounded harness.
- /tmp/bd-rz2i/final-checks.txt records Go, Python, pyte, wcwidth and module hashes.

## Risks and scope limits

- The laptop screenshot and its VHS/font environment are unavailable in the Pod. Evidence uses the Pod's real PTY and a pyte screen. PNG captures use FreeMono at 16 pixels.
- Current main has no Filters & Sort panel or F1-F4 help rows. Existing panels were checked. The shared row renderer is fixed without adding shortcut content.
- Independent reviewer and tester verdicts remain pending. Optional external-service checks were skipped as listed, not counted as passes.

## Delivery and checkpoint

No delivery commits, delivery branch pushes, merges, or PRs were authorized or performed. The first checkpoint push succeeded to the separate, unapproved ref refs/heads/omnigent-checkpoints/bd-rz2i. A final checkpoint contains this report and selected scrubbed evidence. Raw terminal session logs and environment files are excluded.
