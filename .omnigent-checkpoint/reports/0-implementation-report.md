# bd-wd87.16 implementation report

Ready for independent review and testing. All required Linux checks passed. No unresolved required-check failures, blocker issue IDs, unrelated findings, new issues, retries or observed flaky results. Native macOS must still be checked by root after delivery. No delivery commit, delivery push or PR was authorized or performed.

## Context and preflight

- Repository: /home/omnigent/workspace/b9s
- Read task bd-wd87.16, parent bd-wd87 and dependency bd-wd87.4, including dependency's recent comments, before Git preparation. Task and parent had no comments at preflight. Parent covers DMG packaging. Dependency delivered the updater bundle guard through unmerged PR #45.
- Prior root evidence, not reproduced natively in this Linux Pod: copied b9s.app receives macOS SIGKILL before Go starts, including fresh CLI --version. Ordinary b9s runs. True bundle executable refuses --update with DMG guidance and unchanged bytes.
- Initial bootstrap main was clean. No local delivery branch existed.
- `timeout 60s git -C /home/omnigent/workspace/b9s fetch origin refs/heads/omnigent/bd-wd87.4:refs/remotes/origin/omnigent/bd-wd87.4` exited 0. Ref existence checked with `git show-ref --verify`.
- `timeout 60s git -C /home/omnigent/workspace/b9s ls-remote --exit-code --heads origin refs/heads/omnigent/bd-wd87.16` exited 2: remote delivery absent.
- Created branch with `git -C /home/omnigent/workspace/b9s switch -c omnigent/bd-wd87.16 origin/omnigent/bd-wd87.4`.
- Actual branch: omnigent/bd-wd87.16
- Actual HEAD and fetched base origin/omnigent/bd-wd87.4: 0fb809a91023ab20305e6e923a0bfb69d956509f
- Required commit: 0fb809a91023ab20305e6e923a0bfb69d956509f
- `git merge-base --is-ancestor` against fetched base: exit 0. Against HEAD: exit 0.
- `lane-checkpoint inspect --repo /home/omnigent/workspace/b9s --task bd-wd87.16 --base origin/omnigent/bd-wd87.4` exited 2 with explicit no-checkpoint message. No restore needed.
- Read global/project AGENTS, ADR 0028, docs/testing.md and executable-guards.md. No state machine, data contract or architecture decision changed.
- Evidence: preflight.txt. HEAD and fetched base remain unchanged at completion.

## Changed files and guard

- pkg/updater/fileops_test.go: pure table test covers nine ancestor-classification paths, including .app and .APP executable filenames outside a bundle (false), real enclosing bundles (true), nested bundles, misleading directory substrings and an executable suffix inside a real bundle (true). No filesystem or process execution is needed for these cases.
- pkg/updater/fileops_test.go: six real subprocess cases remain: direct bundle, symlink bundle, uppercase bundle, ordinary, symlink ordinary and misleading app-substring directory. Processes still require successful exit and correct DMG/asset-check behavior. No SIGKILL exception or skip exists.
- pkg/updater/fileops_test.go: cross-platform fixture guard rejects any subprocess filename ending in .app, case-insensitively, before copying or executing it. Its failure message names the pure test where filename-classification coverage belongs. This catches recurrence on Linux before a native macOS failure.
- pkg/updater/updater.go: extracted the existing ancestor-only scan into private pure function isInsideAppBundle. GetSelfUpdateBinaryPath calls it after the same symlink resolution. Guard error text, release checks and provenance verification remain unchanged.
- No new source files, file deletions, documentation changes or unrelated edits. User-visible behavior is unchanged, so docs require no change. Diff: two files, 38 insertions, 3 deletions.

## Test-first evidence

Added the portability guard while retaining the defective executable_suffix fixture. Then ran:

`timeout 180s go test ./pkg/updater -run '^TestPerformUpdateExecutableLocation$' -count=1 -timeout=60s -json`

RED exited 1: 6 passed, 1 failed, 0 skipped, 0 interrupted, 0 unrun leaf cases. Only TestPerformUpdateExecutableLocation/executable_suffix failed, at the explicit portability guard before process execution. This is a deterministic Linux reproduction of the invalid fixture configuration, not a claim to reproduce macOS SIGKILL in Linux. Evidence: red.jsonl, red.exit, red-report.json.

Moved the suffix case into pure path coverage, added the remaining classifier cases and extracted the unchanged classifier. GREEN then passed. There was no timeout increase, skip, accepted signal, retry or weakened bundle guard.

## Required completed commands

All commands ran from /home/omnigent/workspace/b9s unless a directory flag is shown. Counts below are completed leaf cases, excluding parent aggregates. Excluded tests outside the selected suite are not counted as unrun.

| Command | Exit | Passed | Failed | Skipped | Interrupted | Unrun |
| --- | --- | --- | --- | --- | --- | --- |
| `timeout 180s go mod download` | 0 | n/a | n/a | n/a | n/a | n/a |
| `timeout 180s go test ./pkg/updater -count=1 -timeout=60s -json` | 0 | 122 | 0 | 0 | 0 | 0 |
| `timeout 300s go build ./...` | 0 | n/a | n/a | n/a | n/a | n/a |
| `timeout 900s make -C /home/omnigent/workspace/b9s test-stable GOFLAGS='-v -count=1'` | 0 | 3183 | 0 | 20 | 0 | 0 |
| `timeout 300s go vet ./...` | 0 | n/a | n/a | n/a | n/a | n/a |
| `git -C /home/omnigent/workspace/b9s diff --check` | 0 | n/a | n/a | n/a | n/a | n/a |
| `gofmt -l /home/omnigent/workspace/b9s/pkg/updater/updater.go /home/omnigent/workspace/b9s/pkg/updater/fileops_test.go` | 0, no output | n/a | n/a | n/a | n/a | n/a |

The full updater GREEN includes all nine pure classifier cases and all six TestPerformUpdateExecutableLocation subprocess cases. Each subprocess has a 10-second context deadline and an 8-second Go test deadline. The updater package cap was 60 seconds. Stable used its unchanged declared 10-minute package cap and ran alone, without base comparison. Report modification times, process states and output tails were checked while commands ran. No hung test or deadline expiry occurred.

Stable: 21 passing packages. Existing optional skips: real bd binaries 1, S3 configuration 3, Dolt DSN 7, Memory preview/research 4, absent fixtures 2, opt-in stress 3. Exact skip names appear in stable-report.json and reasons in stable.txt. Makefile STABLE_SKIP and tests/e2e exclusions remain unchanged. No new skips.

Go build supplies compilation/type checking. go vet supplies the required Go static check. No UI or web code changed, and no browser suite was part of this dispatch.

## Evidence and environment

Evidence directory: /tmp/bd-wd87.16-evidence

- preflight.txt: task/dependency and Git evidence.
- dependencies.txt: dependency command exit.
- red.jsonl, red.exit, red-report.json: expected RED.
- green.jsonl, green.exit, green-report.json: full updater GREEN.
- stable.txt, stable.exit, stable-report.json: exact stable output, exit and measured leaf counts/skip names.
- build.txt, vet.txt: actual completed command exits.
- final-diff.txt: final status, formatting, diff check, HEAD and base.
- environment.json: observed Go/kernel/environment fingerprint.
- green-evidence.json: actual command argv, environment and measured counts for checkpoint evidence.
- summarize.py: local evidence parser, outside the repository.

Runtime: go version go1.25.5 linux/amd64, CGO_ENABLED=1. Dependencies installed from committed go.mod/go.sum versions before checks. No dependency file changed. No shared Dolt database was written by a test.

Checkpoint saves after edits and before long validation succeeded, on refs/heads/omnigent-checkpoints/bd-wd87.16 only. The latest observed source tree before final report save was 2a1360013b2a202ad7c29f47d7ec57138ed061d1. A final checkpoint save includes this scrubbed report and GREEN evidence. Its actual result is recorded in checkpoint-final.json.

## Residual limits and handoff

Native macOS process launch, signing and DMG installation cannot be exercised in this Linux Pod. Root can rerun the revised native suite after delivery. The pure classifier is covered explicitly, and Linux real subprocesses exercise runnable paths. Release installation success is unchanged and outside this fixture fix.

All required Linux gates are green. Independent reviewer/tester work remains Polly's next role sequence. No task status, assignee, dependencies or lane labels were changed. No PR was opened and the parent PR was not merged. No required blocker issues or unrelated follow-up issues exist from this implementation.
