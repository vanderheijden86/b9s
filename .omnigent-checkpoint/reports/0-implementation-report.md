# bd-6e8h.7 implementation report

Status: implementation green, ready for independent review and tester. Delivery remains unapproved. No delivery commit, delivery push or PR was made.

## Git preflight

Repository: /home/omnigent/workspace/b9s
Origin: https://github.com/vanderheijden86/b9s.git
Task and context source: bd-6e8h.7
Assigned base and PR target: omnigent/bd-6e8h.12
Actual delivery branch: omnigent/bd-6e8h.7
Actual HEAD and fetched assigned base SHA: 44a92da087e2877691a824e6ad94d10313d67a5f
Required commit: 44a92da087e2877691a824e6ad94d10313d67a5f

- beads-connect ran first and exited 0, connected as bd_b9s.
- Initial checkout main was clean at 793407bd86c74ba9985c9870c39392974ec8380e. It was not used as the implementation base.
- `timeout 60s git -C /home/omnigent/workspace/b9s fetch origin refs/heads/omnigent/bd-6e8h.12:refs/remotes/origin/omnigent/bd-6e8h.12` exited 0. Assigned ref exists.
- `timeout 60s git -C /home/omnigent/workspace/b9s ls-remote --exit-code --heads origin refs/heads/omnigent/bd-6e8h.7` exited 2. Remote delivery branch absent, so no delivery ref existed to fetch.
- No existing local delivery branch or uncommitted changes. Created delivery branch from origin/omnigent/bd-6e8h.12.
- `git -C /home/omnigent/workspace/b9s merge-base --is-ancestor 44a92da087e2877691a824e6ad94d10313d67a5f origin/omnigent/bd-6e8h.12` exited 0.
- The same required ancestry check against HEAD exited 0.
- HEAD remains unchanged after implementation and checks.

## Changed-base checkpoint compatibility

`lane-checkpoint inspect --repo /home/omnigent/workspace/b9s --task bd-6e8h.7 --base origin/omnigent/bd-6e8h.12` found checkpoint 43fb24242069e8985d26652ed8495d4f540e2e35. Its source HEAD/base was old origin/main at 793407bd86c74ba9985c9870c39392974ec8380e. Helper mismatches were source_head, base_ref and base_sha.

Blind restore was not used. The full saved source patch and PR47 delta were reviewed. `git apply --check /tmp/bd-6e8h.7-retarget/checkpoint.patch` exited 0 against the assigned base. The patch does not replace PR47's SameProject path resolution or TouchRecent deduplication. The test file was applied first for fresh RED, then the rest of the reviewed patch was applied. A command-level active-alias test was added in the existing test file.

The native E2E file is byte-identical to the saved checkpoint. Final Git diff against that checkpoint is empty for tests/e2e/recent_projects_e2e_test.go. No old-base check evidence was reused. The first new checkpoint push succeeded and retains the old checkpoint as an ancestor, ancestry exit 0. Final checkpoint reports are saved separately alongside this report.

## Result and changed files

`:forget <visible header number>` forgets an inactive recent project without connecting to that project's server. Canonical command completion and help make it discoverable. It refuses a locked list, the active project's identity including path aliases, invalid numbers and unsuccessful saves. It persists through SaveRecentTo before changing the UI. Explicit exclusions prevent disk merge from restoring the target, while retaining another window's additions and unrelated YAML comments and settings.

- pkg/config/recent.go: ForgetRecent and targeted SaveRecentTo merge exclusions.
- pkg/config/recent_test.go: identity, order, locking and concurrent-window persistence regressions.
- pkg/ui/commands.go: closed command table and header-number argument.
- pkg/ui/command_prompt.go: command dispatch.
- pkg/ui/command_prompt_test.go: prompt-level regression, visible slots, lock, active identity, save failure and symlink/unclean aliases.
- pkg/ui/project_table.go: guarded forget action, persistence and header/picker rebuild.
- pkg/ui/model.go: in-app help entry.
- tests/e2e/recent_projects_e2e_test.go: unchanged checkpoint native forget cases for both lock values.
- docs/USER_GUIDE.md: command, behavior, lock and intentional concurrency limits.
- docs/ARCHITECTURE.md: explicit target exclusions and save-before-UI behavior.

10 tracked files changed. No new source files, deleted files, dependency changes, ADR edits or unrelated product changes.

## Commands and measured results

All Go checks ran from /home/omnigent/workspace/b9s. Counts below separate top-level tests from subtests. All reports are fresh for this combined tree.

| Command | Exit | Top-level pass/fail/skip | Subtest pass/fail/skip |
|---|---:|---:|---:|
| `timeout 180s go mod download` | 0 | N/A | N/A |
| `timeout 180s go test ./pkg/ui -run '^TestForgetRecentProjectRemovesOfflineEntryAndPreservesComments$' -count=1 -timeout 60s -json` (RED before feature application) | 1 | 0/1/0 | 0/0/0 |
| `timeout 180s go test ./pkg/config ./pkg/ui -run 'TestForgetRecent\|TestSaveRecentTo\|TestEveryCommandNameResolves\|TestRecentProject_SameProject\|TestTouchRecent\|TestHeaderProjects' -count=1 -timeout 60s -json` | 0 | 34/0/0 | 19/0/0 |
| `timeout 180s go test ./tests/e2e -run '^TestForgetRecentProjectE2E$' -count=1 -timeout 90s -json` | 0 | 1/0/0 | 2/0/0 |
| `GOFLAGS='-v -count=1' timeout 660s make test-stable` | 0 | 2078/0/18 | 1307/0/2 |
| `timeout 180s go build ./...` | 0 | N/A | N/A |
| `timeout 180s go vet ./...` | 0 | N/A | N/A |

The RED test failed for the intended behavioral reason: unknown command forget. GREEN supersedes that expected historical failure. Focused GREEN passed two packages. Stable passed all 21 selected packages and ran alone without a base comparison. No selected cases interrupted or unrun. No retries or observed flakes. Build supplies the Go typecheck. Vet supplies the Go lint gate.

Additional checks: `git -C /home/omnigent/workspace/b9s diff --check` exited 0. `gofmt -l` with all eight changed Go file absolute paths exited 0 with no files listed. `gofmt -w /home/omnigent/workspace/b9s/pkg/ui/command_prompt_test.go` ran after adding the alias test. The checkpoint patch was reviewed and applied with `git apply --include=pkg/ui/command_prompt_test.go`, then `git apply --exclude=pkg/ui/command_prompt_test.go`. No source changed after focused GREEN.

## Native TUI proof and environment

Runtime: go1.25.5 linux/amd64, CGO_ENABLED=1, util-linux script 2.41.5. Stable Makefile sets CGO_CFLAGS=-DSQLITE_ENABLE_FTS5. Other Go commands used the inherited default CGO_CFLAGS=-O2 -g. No web/browser UI changed or browser suite required.

Both unchanged native leaves passed:

- TestForgetRecentProjectE2E/lock_recent=false: final header alpha then beta, offline absent, named success, persisted names alpha/beta and YAML comments retained.
- TestForgetRecentProjectE2E/lock_recent=true: final header alpha/offline/beta, lock refusal, saved names and comments retained.

Each PTY has a 15-second deadline and a 3500ms autoclose. Package timeout is 90s, outer timeout 180s. JSONL fixture projects and closed loopback port 127.0.0.1:1 were used. Tests did not write to shared Dolt. The existing native test assertions were not loosened. This run is Linux native PTY evidence. macOS was not rerun in this Pod. The earlier macOS experiment remains historical evidence only.

## Skips and selection

Stable's 18 top-level skips: optional real bd binaries (1), optional S3 endpoint cases (3), optional live Dolt reader/watcher cases without DSN (7), optional Memory preview/research cases without isolated fixtures (4), opt-in 10-minute stress tests (3). Two subtests skipped absent cass/srps JSONL datasets. Exact names and emitted reasons are in stable-skips.txt and stable-summary.json.

Makefile's stable selection excludes tests/e2e and its declared host-sensitive regex. Those exclusions are selection, not passed tests. The required native forget regression was run separately and skipped neither leaf.

## Guards, documentation and residual limits

Regression tests guard wrong-number removal, locked membership, active-project aliases, save-failure atomicity, YAML comment preservation and explicit-target disk merge. PR47's shared SameProject contract remains the identity boundary. ADRs 0008, 0009, 0012 and 0015, docs/testing.md and docs/ARCHITECTURE.md were read. The existing action/persistence patterns require no new architectural decision.

Intentional semantics are preserved and documented: an in-flight count refresh may finish, a stale window can save its old entry again, and opening the project again adds it again. Active projects require switching first.

Unresolved required-check failures and blocker issue IDs: none on this assigned stacked base. Historical main-based native failures are superseded by fresh unchanged cases on the assigned PR47 tree. bd-6e8h.9 and bd-6e8h.12 are closed. bd-qwck remains deferred main-integration follow-up and is not a blocker for this stacked delivery. No new unrelated follow-up issue was filed.

## Evidence files

All under /tmp/bd-6e8h.7-retarget:
preflight.txt, checkpoint-compatibility.txt, checkpoint-implementation.txt,
dependencies.txt, red.jsonl, red-summary.json, green.jsonl, green-summary.json,
green-evidence.json, tui.jsonl, tui-summary.json, tui-evidence.json,
tui-lock-false.txt, tui-lock-true.txt, native-assertions-final.diff,
stable.log, stable-summary.json, stable-evidence.json, stable-skips.txt,
build.txt, vet.txt, source-check.txt, gofmt.txt, runtime.txt, tree.txt.

## Final checkpoint advisory

The final checkpoint push succeeded at refs/heads/omnigent-checkpoints/bd-6e8h.7, source_tree 66799ae5c3a8adffd26df68329883fca814994c0. Both earlier checkpoints (43fb242 and original 528ffdf) remain ancestors, each ancestry check exit 0.

`timeout 60s lane-checkpoint compare --repo /home/omnigent/workspace/b9s --task bd-6e8h.7 --base origin/omnigent/bd-6e8h.12 --evidence /tmp/bd-6e8h.7-retarget/stable-evidence.json` exited 2. It reports compatible=true, complete=false and “saved evidence is stale, failed, skipped or incomplete”. The saved stable evidence has the measured 18 top-level skips, which the helper does not certify for reuse. This advisory comparison failure is reported to Polly. It does not invalidate the fresh stable command's exit 0 or its explicit skips. No check was repeated or reused through checkpoint evidence. No unresolved required-check failure results from this optional comparison.
