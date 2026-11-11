# bd-teri.6 implementation report

Prepared: 2026-11-11 03:40 CET

Implementation is green and ready for independent review. No delivery commit, delivery push or pull request was made. Only the separate unapproved checkpoint ref was pushed.

## Scope and behavior

Read GitHub #23 with `timeout 60s gh issue view 23 --repo vanderheijden86/b9s --comments` and `--json title,body,comments`. Neither the body nor the owner comment contains a screenshot or image link. Read the cited derailed/k9s#3902 body and diff. The owner comment controls the implementation.

The unchanged fetched main used `atotto/clipboard.WriteAll` directly and had no clipboard configuration. The new `pkg/clipboard.Write(text, mode)` entry point supports:

- `auto`, including the zero mode: system utility first, OSC52 only after a utility error. No DISPLAY test selects the fallback.
- `system`: system utility only, preserving its error.
- `osc52`: terminal output only, without a system utility call.
- A real stdout TTY check through `golang.org/x/term`, and refusal for TERM=dumb.
- A 74,994-byte base64 payload cap checked before writing. Oversized copies fail without truncation.
- GNU screen DCS wrapping in 76-byte encoded chunks. The complete escape sequence uses one stdout Write call. Write errors and short writes fail the operation.
- Both system and fallback failure causes remain available to callers.

`clipboard: auto|system|osc52` is a scalar in config.yaml. Missing keys default to auto, existing configurations need no manual migration, saved modes survive round trips, and invalid modes fail config loading.

All former five `WriteAll` references in model.go, including the initializer and four copy operations, were replaced by the shared model writer and the single package entry point. Health information, board ID, full Markdown, and tree ID/title copies all use it. Existing injected test writers remain supported.

## Files

- `pkg/clipboard/clipboard.go`: transport modes, config decoding and terminal writer (new).
- `pkg/clipboard/clipboard_test.go`: mode selection, gating, cap, wrapping, errors and single-write checks (new).
- `pkg/config/config.go`: mode field and default.
- `pkg/config/config_test.go`: default and explicit mode round trips, invalid mode refusal.
- `pkg/ui/model.go`: migrate every copy path to the shared writer.
- `pkg/ui/update_test.go`: board and health copy routing regression.
- `tests/e2e/clipboard_e2e_test.go`: real PTY, isolated config, b then y, exact OSC52 payload and success status (new).
- `docs/USER_GUIDE.md`: config key, default, fallback and terminal limits.
- `docs/testing.md`: focused clipboard unit and PTY commands.

Read docs/testing.md, ADR 0010, ADR 0055, the executable-guards guide and variable-placement guide. No data-read/write architecture was changed. No active ADR changed. The transport follows the owner's prescribed design and needs no separate decision record.

## Preflight and current Git context

- Repository: `/home/omnigent/workspace/b9s`
- Origin: `https://github.com/vanderheijden86/b9s.git`
- Assigned base and PR target: `main`
- Actual branch: `omnigent/bd-teri.6`
- Actual HEAD: `c2fccef8b537ed034ebd360ea7cf31c02250e083`
- Fetched base: `c2fccef8b537ed034ebd360ea7cf31c02250e083`
- Required commits: none. Prerequisite ancestry against base and HEAD: not applicable.
- Additional `git merge-base --is-ancestor origin/main HEAD`: exit 0.
- Initial remote delivery probe: exit 2, absent. Explicit delivery fetch returned 128, missing remote ref. Branch created from fetched origin/main.
- Initial worktree clean. Checkpoint initially absent. No reset, stash, file deletion, commit, delivery push or PR.
- Current worktree: task changes only, six modified tracked files and three new files.
- Preflight report: `/tmp/bd-teri.6-preflight-BbcFvS.txt`.

```
M docs/USER_GUIDE.md
 M docs/testing.md
 M pkg/config/config.go
 M pkg/config/config_test.go
 M pkg/ui/model.go
 M pkg/ui/update_test.go
?? pkg/clipboard/clipboard.go
?? pkg/clipboard/clipboard_test.go
?? tests/e2e/clipboard_e2e_test.go
```

## Red evidence

Tests were written before the required behavior. All commands ran from the shared repository and exited 1 for the expected assertions:

1. `timeout 150s go test ./pkg/ui -run '^TestClipboardCopyPathsUseModelWriter$' -count=1 -timeout=60s -json`
   - Unchanged main product code bypassed the injected writer for board and health copies.
   - Leaf cases: 0 passed, 2 failed, 0 skipped. Top-level: 0 passed, 1 failed, 0 skipped.
   - `/tmp/bd-teri.6-red-ui-report.json` and `/tmp/bd-teri.6-red-ui.jsonl`.
2. `timeout 120s go test ./pkg/config -run '^TestClipboardConfig' -count=1 -timeout=60s -json`
   - Unchanged config code lost all four modes/default round trips and accepted an unknown mode.
   - Leaf cases: 0 passed, 5 failed, 0 skipped. Top-level: 0 passed, 2 failed, 0 skipped.
   - `/tmp/bd-teri.6-red-config-report.json` and `/tmp/bd-teri.6-red-config.jsonl`.
3. `timeout 60s go test ./pkg/clipboard -run '^TestWriteModes$' -count=1 -timeout=30s -json`
   - Initial extraction retained the system-only behavior. Auto fallback, zero-mode fallback, forced OSC52 and invalid-mode refusal failed before implementing the transport logic.
   - Leaf cases: 3 passed, 4 failed, 0 skipped. Top-level: 0 passed, 1 failed, 0 skipped.
   - `/tmp/bd-teri.6-red-modes-report.json` and `/tmp/bd-teri.6-red-modes.jsonl`.

The first E2E red result is inconclusive and is not used as regression proof. Its fixture did not enter the board. Two subsequent fixture runs failed, each 0 passed / 1 failed / 0 skipped. Explicit b fixed the view setup. The remaining cause was an xterm TERM override, which triggered unanswered termenv color probes that consumed test input. The fixture uses the existing harness screen terminal. The assertions and deadlines were retained. These are resolved setup failures, not flaky tests or current required-check failures. Both the original claim and its correction were recorded as attributed task comments.

## Final commands and observed results

Counts below use leaf tests/subtests without counting parents twice. Top-level counts are also recorded for comparison. No test retries or flaky outcomes were reported.

| Exact command | Exit | Passed | Failed | Skipped |
|---|---:|---:|---:|---:|
| `timeout 120s go test ./pkg/clipboard ./pkg/config ./pkg/ui -run 'Test(WriteModes|OSC52|AutoPreserves|Clipboard|DetailCKeyCopiesSelectedIssue|TreeLowercaseCCopiesSelectedIDAndTitle)' -count=1 -timeout=60s -json` | 0 | 24 | 0 | 0 |
| `timeout 180s go test ./tests/e2e -run '^TestClipboardOSC52AfterY$' -count=1 -timeout=120s -json` | 0 | 1 | 0 | 0 |
| `timeout 660s env GOFLAGS=-v make -C /home/omnigent/workspace/b9s test-stable` | 0 | 3574 | 0 | 20 |
| `timeout 180s go build ./...` | 0 | N/A | N/A | N/A |
| `timeout 180s go vet ./...` | 0 | N/A | N/A | N/A |

Dependencies: `timeout 180s go mod download`, exit 0, using committed go.mod/go.sum. No dependency files changed.

The stable suite ran alone, without a baseline worktree or comparison. Its unchanged Makefile exclusions remain in force. Top-level counts: 2,133 passed, 0 failed, 18 skipped. The 20 leaf skips comprise seven Dolt DSN tests, three opt-in S3 tests, one real-bd probe, four Memory preview/research tests, two absent real-issue fixture subtests, and three opt-in stress tests. Their reasons and identities appear in `/tmp/bd-teri.6-stable-report.json`. No clipboard check skipped. No shared Dolt writes were used by the clipboard tests.

`git -C /home/omnigent/workspace/b9s diff --check` exited 0. `gofmt -l` on all seven changed/new Go files returned no paths. Go build supplies the type check. Go vet supplies the Go lint gate. No web source changed, so browser tests and browser smoke are outside this terminal task.

Reports:

- `/tmp/bd-teri.6-focused-green-report.json`
- `/tmp/bd-teri.6-stable-report.json`, with complete raw output in `/tmp/bd-teri.6-stable.txt`
- `/tmp/bd-teri.6-e2e-evidence.json`, with complete final output in `/tmp/bd-teri.6-green-e2e-3.jsonl`
- `/tmp/bd-teri.6-build.txt`
- `/tmp/bd-teri.6-vet.txt`

Environment: Go 1.25.5, linux/amd64, CGO_ENABLED=1, GOTOOLCHAIN=auto, script from util-linux 2.41.5. E2E TERM is screen-256color. make test-stable sets CGO_CFLAGS=-DSQLITE_ENABLE_FTS5. The E2E harness bounds its PTY runs with the existing 15-second context and auto-exit timer. Every test command also had an outer deadline.

## Guards, limits and handoff

The regression tests guard routing, fallback selection, invalid configuration, redirected/dumb terminals, encoded size, screen framing, output failures and atomic output. The PTY test checks the real binary emits ESC ]52;c; with the selected ID after y.

Required unresolved blockers: none. Unrelated issues filed: none. Optional integration skips are listed above, separately from the required clipboard checks.

Terminal clipboard permission and emulator support cannot be acknowledged through OSC52. Successful terminal output proves byte delivery, not the external emulator's clipboard policy. A native utility that reports success remains authoritative in auto mode. These are transport limits, not test failures.

Stop here for independent review and testing. Delivery is not authorized. When Polly authorizes a PR, its body must include `Refs #23`, target main, and reuse an existing matching PR before creating one.
