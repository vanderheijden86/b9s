# bd-t8j5.11 implementation report

Verdict: PASS. All required implementer checks completed successfully. Independent reviewer and tester verdicts are still required. No unresolved required-check failures. No unrelated issues filed.

## Git preflight

- Shared repository: /home/omnigent/workspace/b9s, origin https://github.com/vanderheijden86/b9s.git.
- Initial checkout: main, clean. No existing local delivery branch or concurrent changes.
- `timeout 60s git -C /home/omnigent/workspace/b9s fetch origin main`: exit 0.
- Fetched base and PR target: origin/main, 793407bd86c74ba9985c9870c39392974ec8380e.
- `timeout 60s git -C /home/omnigent/workspace/b9s ls-remote --exit-code --heads origin refs/heads/omnigent/bd-t8j5.11`: exit 2, exact delivery ref absent. No remote delivery ref existed to fetch.
- `git -C /home/omnigent/workspace/b9s switch -c omnigent/bd-t8j5.11 origin/main`: exit 0.
- Actual branch: omnigent/bd-t8j5.11. HEAD remains 793407bd86c74ba9985c9870c39392974ec8380e.
- Required commits: none. Ancestry checks against base and HEAD: not applicable.
- Initial checkpoint inspect: exit 2, absent. Prior attempt files were absent in this Pod.
- Preflight evidence: /tmp/bd-t8j5.11-preflight-attempt2.txt.
- No delivery commit, delivery push or PR. Only the separate unapproved checkpoint ref was pushed.

## Changed files

- scripts/web/entrypoint.sh: optional B9S_WEB_CONFIG_FILE seeds writable generated config. Missing or unreadable configured file fails startup. Credentials remain inherited.
- scripts/web/entrypoint_test.go (new): bounded startup regression checks config seeding, original mount preservation, environment credentials, trusted-header arguments and missing-file refusal.
- scripts/web/README.md: deployment contract, attachment settings and startup check command.
- tests/e2e/attachments_e2e_test.go (new): real PTY attachment flow with stock bd, private embedded Dolt and local blobs.
- docs/testing.md: stock-bd attachment PTY command and isolation contract.

No dependency, vendor, Beads state, ADR or generated web asset changes. Existing work was preserved.

## Commands and results

All Go commands ran in /home/omnigent/workspace/b9s. Counts below count leaf tests, excluding parent containers. No retries or observed flaky outcomes.

| Exact command | Exit | Passed | Failed | Skipped | Evidence |
| --- | ---: | ---: | ---: | ---: | --- |
| `timeout 180s go mod download` | 0 | n/a | n/a | n/a | /tmp/bd-t8j5.11-dependencies.txt |
| `timeout 90s go test ./scripts/web -run '^TestWebEntrypointAttachmentSettings$' -count=1 -timeout=30s -json` (initial test syntax error) | 1 | no tests ran | no tests ran | no tests ran | /tmp/bd-t8j5.11-startup-red.jsonl |
| Same focused startup command, syntax fixed, before implementation | 1 | 1 | 2 | 0 | /tmp/bd-t8j5.11-startup-red2.jsonl |
| Same focused startup command, after implementation | 0 | 3 | 0 | 0 | /tmp/bd-t8j5.11-startup-green.jsonl |
| `B9S_TEST_STOCK_BD=/usr/local/libexec/beads/bd timeout 180s go test ./tests/e2e -run '^TestAttachmentsStockBDLocalBackendE2E$' -count=1 -timeout=120s -json` (first run) | 0 | 1 | 0 | 0 | /tmp/bd-t8j5.11-pty-first.jsonl |
| Same PTY command, explicit detail heading/author assertions and terminal evidence added | 0 | 1 | 0 | 0 | /tmp/bd-t8j5.11-pty-final.jsonl |
| `timeout 180s go build ./...` | 0 | n/a | n/a | n/a | /tmp/bd-t8j5.11-build.txt |
| `timeout 180s go vet ./...` | 0 | n/a | n/a | n/a | /tmp/bd-t8j5.11-vet.txt |
| `GOFLAGS=-v timeout 720s make test-stable` | 0 | 3171 | 0 | 20 | /tmp/bd-t8j5.11-stable.txt |
| `sh -n /home/omnigent/workspace/b9s/scripts/web/entrypoint.sh` | 0 | n/a | n/a | n/a | Observed shell syntax result |
| `git -C /home/omnigent/workspace/b9s diff --check` | 0 | n/a | n/a | n/a | Observed clean diff check |

`make test-stable` ran alone, without baseline comparison or another check. GOFLAGS=-v added verbose output without changing the Makefile suite selection. Its package timeout was 10m. All 22 packages passed. Top-level results: 2058 passed, 0 failed, 18 skipped. Including parent containers: 3349 pass events and 20 skip events. Leaf results: 3171 passed, 0 failed, 20 skipped, 0 interrupted, 0 unrun. The focused final startup check has one passed parent containing three passed leaves. Final PTY: one passed test. The focused startup test also passed inside the stable suite.

The 20 stable skips cover optional real-binary probes, S3 integration, Dolt integration, Memory research/preview, three opt-in stress tests and two absent fixture files. Full names and reasons are in /tmp/bd-t8j5.11-stable-counts.json and the verbose report. Filtered Makefile exclusions and tests/e2e were outside that command's selection, not counted as skipped. No shared Dolt test writes.

Go 1.25.5 linux/amd64 supplies type checking and compilation. go vet supplies the Go lint gate. No TypeScript changed.

## Acceptance evidence

- RED reproduced missing configuration propagation before the production edit. Both previously failed cases pass after the edit.
- Startup shell runs with only container filesystem paths remapped into a temporary directory. SQL, httpd, destructive home cleanup and b9s are stubbed. The test checks the exact copied settings with config.LoadFrom, preserves the read-only input and checks both attachment credential values at exec with trusted proxy arguments.
- PTY runs the built b9s, uses I to attach terminal-note.txt, checks `1 attached`, `ATTACHMENTS · 1`, the attachment author and R picker, then opens with Enter.
- `B9S_TEST_MODE=1` produces `test mode: opener not run` after the actual download. No browser or editor runs.
- Stock bd export contains exactly one valid attachment reference. CLI list returns the expected hash. CLI get bytes equal the original.
- Expected SHA256: 0c149767d10daaacf12d58d014ea2f913c7055fec86df295c1d1188e42b07b1b.
- Runtime bd: /usr/local/libexec/beads/bd, version 1.3.0-rc.1, build 9c6a69ec1. `go version -m` reports module github.com/steveyegge/beads v1.3.0-rc.1. No fork bd build was made.
- Isolation blanks inherited Beads/bd/b9s connection settings, uses a private home, disables lane connection loading, and requires embedded metadata before creating an issue. Local attachment backend only.
- Startup subcommands have a 5s deadline. PTY has the shared 15s deadline and 6500ms autoclose. Fixture subcommands have 30s limits within a 90s fixture deadline. Go and outer command timeouts are listed above.
- Regression test and fail-fast configured-file check guard against silent settings loss. No architecture change requiring an ADR.

## Limitations and handoff

- Docker is not installed. bwrap cannot create namespaces. No Docker image build, deployed proxy traffic, real S3 or image-specific Alpine runtime check was performed. The startup harness and real host PTY are the observed evidence.
- The Dockerfile pins stock bd v1.3.0. The installed stock PTY binary is v1.3.0-rc.1, recorded above.
- The web server has no attachment blob-serving endpoint. No endpoint was added, so the conditional task note about sniffed HTTP Content-Type/Content-Disposition does not apply to this change.
- No unresolved required blockers or deferred unrelated findings.
- No test evidence was reused. A comparison rejected evidence after the PTY assertions changed, and the final PTY command was run again. Checkpoint pushes succeeded. Stable evidence includes its optional skips and is advisory, not a reusable all-passing gate.
- Delivery is unapproved. Polly can dispatch independent review and testing against the shared dirty delivery branch. Do not commit, push the delivery branch or open a PR without explicit approval.

## Evidence files

- /tmp/bd-t8j5.11-verification.txt: combined scrubbed reports, runtime fingerprint and measured counts.
- /tmp/bd-t8j5.11-stable-evidence.json: stable command, runtime and counts.
- /tmp/bd-t8j5.11-pty-final-evidence.json: focused PTY command, runtime and counts.
- /tmp/bd-t8j5.11-runtime.json: Go/bd runtime and bd binary SHA256.
- /tmp/bd-t8j5.11-startup-green-counts.json, /tmp/bd-t8j5.11-pty-final-counts.json, /tmp/bd-t8j5.11-stable-counts.json: measured counts.
- /tmp/bd-t8j5.11-implementation-report.md: this implementation report.
