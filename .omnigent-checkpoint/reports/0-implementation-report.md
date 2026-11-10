# bd-uv2t implementation report

Result: required implementation checks passed. Existing product code satisfies the requested Unicode sequence rendering. This change adds proof coverage and documents its commands. No product defect was reproduced and no product logic changed, so no artificial failing RED run was manufactured. The focused copy test passed on its first run. Independent reviewer and tester stages remain Polly's responsibility.

## Git preflight

- Shared repository: `/home/omnigent/workspace/b9s`.
- Assigned base and PR target: `main`.
- Actual branch: `omnigent/bd-uv2t`, created from fetched `origin/main` after checking the clean bootstrap checkout and finding no local delivery branch.
- HEAD and fetched base SHA: `c2fccef8b537ed034ebd360ea7cf31c02250e083`. Both remain unchanged after implementation.
- `timeout 60s git -C /home/omnigent/workspace/b9s fetch origin main`: exit 0.
- `timeout 60s git -C /home/omnigent/workspace/b9s ls-remote --exit-code --heads origin refs/heads/omnigent/bd-uv2t`: exit 2, remote delivery branch absent.
- Required commits: none. Required-commit ancestry checks against base and HEAD: not applicable.
- Initial `lane-checkpoint inspect --repo /home/omnigent/workspace/b9s --task bd-uv2t --base origin/main`: no checkpoint exists. No restore performed.
- Evidence: `/tmp/bd-uv2t-evidence/preflight.txt`.

## Changed files

- `pkg/ui/issue_detail_test.go`: `TestDetailCKeyKeepsMermaidSequenceSource` opens the exact source through Enter, checks rendered detail, presses `c`, and captures the clipboard writer. It requires the original Mermaid fence byte-for-byte and rejects rendered boxes in the copy.
- `tests/e2e/mermaid_e2e_test.go`: `TestMermaidSequenceProofE2E` opens an isolated JSONL issue with the exact task source in the real built TUI. It checks all 13 consecutive diagram rows, including both top and bottom participant boxes, continuous lifelines, solid forward and dashed reverse arrows, and both labels. It rejects retained source and records the terminal frame.
- `docs/testing.md`: documents the proof and focused commands.

All edits are additions in existing files. No new source file, deletion, vendor edit, dependency change, or unrelated work is included. ADR 0030 already decides the behavior. No new ADR is needed. The tests guard the display contract and preserve the original source across the copy action.

## Completed checks

All commands ran from the shared repository. Every command below exited 0. Counts are final outcomes, not console sequence numbers. Named-case counts include parent tests, subtests and fuzz seeds. Top-level counts are also provided to avoid treating these measures as interchangeable.

| Exact command | Passed | Failed | Skipped | Interrupted | Unrun | Evidence |
| --- | ---: | ---: | ---: | ---: | ---: | --- |
| `timeout 120s go test ./pkg/ui -run 'TestMarkdownRenderer.*Mermaid\|TestRenderIssueDetail_Mermaid' -count=1 -timeout=60s -json` | 22 | 0 | 0 | 0 | 0 | `focused-existing.jsonl` |
| `timeout 120s go test ./pkg/ui -run '^TestDetailCKeyKeepsMermaidSequenceSource$' -count=1 -timeout=60s -json` | 1 | 0 | 0 | 0 | 0 | `focused-copy.jsonl` |
| `timeout 180s go test ./tests/e2e -run '^TestMermaid(DetailRendering\|SequenceProof)E2E$' -count=1 -timeout=120s -json` | 2 | 0 | 0 | 0 | 0 | `tui.jsonl` |
| `GOFLAGS=-v timeout 720s make test-stable` | 3760 | 0 | 20 | 0 | 0 | `stable.txt`, `stable-summary.txt`, `stable-skips.txt` |

The backslashes before pipes in this Markdown table are table escaping. The actual regex arguments contain ordinary `|` characters. The captured command scripts and Go JSON reports retain the unescaped commands and results.

The stable suite ran alone with no base comparison. `GOFLAGS=-v` enables measured final counts without changing the Makefile's selected suite. Its top-level counts are 2125 passed, 0 failed, 18 skipped. Two additional skipped subtests require absent external JSONL fixtures. The remaining skips require optional real bd binaries, S3, Dolt, Memory preview/research inputs, or the explicit stress-test flag. `stable-skips.txt` records all 20 skip outcomes and their reasons. Makefile exclusions are outside the selected suite and are not counted as executed or skipped. No selected test was interrupted or left unrun. No retries were used and no flaky outcome was observed.

Other completed commands:

- `timeout 180s go mod download`: exit 0, pinned modules installed before checks. `dependencies.txt`.
- `timeout 180s go build ./...`: exit 0. This includes Go type checking. `build.txt`.
- `timeout 180s go vet ./...`: exit 0. `vet.txt`.
- `gofmt -w /home/omnigent/workspace/b9s/pkg/ui/issue_detail_test.go /home/omnigent/workspace/b9s/tests/e2e/mermaid_e2e_test.go`: completed. Final `gofmt -l` on these files returned no paths, exit 0.
- `git -C /home/omnigent/workspace/b9s diff --check`: exit 0. `final-review.txt`.
- `timeout 30s go run /tmp/bd-uv2t-evidence/render.go`: exit 0, inspected the existing renderer's exact sequence geometry before adding the terminal proof. `diagram.txt`.

The real terminal harness bounds each PTY run to 15 seconds and sets `B9S_TUI_AUTOCLOSE_MS=2500`. The Go suite has a 120-second cap and its outer command has a 180-second cap, including compilation. The stable Makefile bounds each package to ten minutes, with a 720-second outer command. No timeout was reached.

## Observed acceptance evidence

`/tmp/bd-uv2t-evidence/tui-frame.txt` contains the captured, ANSI-free terminal detail screen for `bd-uv2t` with:

- `User` and `b9s` in top and bottom boxes.
- Lifelines attached to the boxes through `┬` and `┴`.
- A solid right arrow labeled `Open detail`.
- A dashed left arrow labeled `Render diagram`.
- No visible `sequenceDiagram` declaration or request source.

`focused-copy.jsonl` records the successful copy-action test after rendering. The clipboard writer receives the original source, including its fence, participant declarations, indentation and both messages.

Runtime: Go 1.25.5, linux/amd64, util-linux script 2.41.5. Full non-secret fingerprint: `environment.txt`. Tests used isolated local JSONL fixtures and no shared database writes. No browser surface changed or browser check was required. Preview URL is not applicable to this terminal proof.

## Review, risks and delivery

Implementer self-review: diff and formatting passed, with scope limited to proof tests and their documentation. Independent review is pending. Required-check failures: none. Unrelated follow-up issues: none filed.

Residual limits: the complete sequence layout proof uses a 160-column terminal. Narrow panes retain the existing ADR 0030 source fallback. Clipboard contents are captured through the model's writer seam rather than a host clipboard service. Optional integrations and absent external JSONL fixtures were not exercised.

Delivery commits, delivery pushes and PR creation remain forbidden until Polly approves their stage. None occurred. The delivery HEAD is unchanged. Only the separate unapproved checkpoint ref `refs/heads/omnigent-checkpoints/bd-uv2t` was pushed successfully. The final checkpoint save is reported separately after completion. Its evidence is advisory and does not replace independent checks.

All evidence files named above are under `/tmp/bd-uv2t-evidence/`. `tui-evidence.json` and `stable-evidence.json` contain measured command/environment/count records. Lane progress and attributed Beads comments were published throughout the work.
