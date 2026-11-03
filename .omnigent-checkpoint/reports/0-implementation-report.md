# bd-wd87.4 implementation report

## Result

The CLI checks the resolved executable before fetching release metadata or prompting. The updater library checks the same path before checking release assets or writing files. An executable whose ancestor directory ends in `.app` (case insensitive) is refused with a code-signature explanation and the latest-release DMG download link. Ordinary executable paths retain the existing update flow and provenance checks.

Delivery commits, delivery pushes and PR creation were not authorized and were not performed. Only the separate unapproved checkpoint ref was pushed.

## Git preflight

- Repository: `/home/omnigent/workspace/b9s`
- Initial branch: `main`, clean bootstrap checkout.
- Actual implementation branch: `omnigent/bd-wd87.4`.
- HEAD: `793407bd86c74ba9985c9870c39392974ec8380e`.
- Fetched `origin/main`: `793407bd86c74ba9985c9870c39392974ec8380e`.
- `timeout 60s git -C /home/omnigent/workspace/b9s fetch origin main`: exit 0.
- `timeout 60s git -C /home/omnigent/workspace/b9s ls-remote --exit-code --heads origin refs/heads/omnigent/bd-wd87.4`: exit 2, remote delivery branch absent, SHA not applicable.
- Local delivery branch was absent. Created with `git -C /home/omnigent/workspace/b9s switch -c omnigent/bd-wd87.4 origin/main`.
- Required commits: none. Required-commit ancestry against base and HEAD is not applicable.
- Initial checkpoint inspection: no checkpoint existed.
- Final HEAD and fetched base remain unchanged.

## Changed files

- `cmd/b9s/main.go`: refuse bundled self-update before any network request or prompt.
- `pkg/updater/updater.go`: resolve the executable through existing symlink resolution, guard `.app` ancestors, and protect `PerformUpdate` callers.
- `pkg/updater/fileops_test.go`: subprocess regression cases for direct, symlinked and uppercase bundles, ordinary and symlinked ordinary binaries, a directory containing `.app` as a substring, and an executable with an `.app` filename suffix. Subprocesses have 10-second context deadlines and 8-second Go test deadlines.
- `docs/USER_GUIDE.md`: document DMG updates and the symlink case.

## Commands and measured results

All commands ran from `/home/omnigent/workspace/b9s`. Test counts below count leaf cases, excluding parent test aggregates. Rapid-generated samples are not counted as separate cases. Suite exclusions in the Makefile are outside the selected stable suite.

| Command | Exit | Passed | Failed | Skipped |
| --- | --- | --- | --- | --- |
| `timeout 180s go mod download` | 0 | n/a | n/a | n/a |
| `timeout 180s go test ./pkg/updater -run '^TestPerformUpdateExecutableLocation$' -count=1 -timeout=60s -json` | 1, expected RED | 4 | 3 | 0 |
| `timeout 180s go test ./pkg/updater -count=1 -timeout=60s -json` | 0, GREEN | 114 | 0 | 0 |
| `timeout 300s go build ./...` | 0 | n/a | n/a | n/a |
| `timeout 900s make test-stable GOFLAGS=-v` | 0 | 3175 | 0 | 20 |
| `timeout 300s go vet ./...` | 0 | n/a | n/a | n/a |
| `timeout 300s go build -o /tmp/bd-wd87.4-evidence/b9s ./cmd/b9s` | 0 | n/a | n/a | n/a |
| Offline CLI smoke checks below | expected 1 each | 2 | 0 | 0 |
| `git -C /home/omnigent/workspace/b9s diff --check` | 0 | n/a | n/a | n/a |
| `gofmt -l /home/omnigent/workspace/b9s/pkg/updater/updater.go /home/omnigent/workspace/b9s/pkg/updater/fileops_test.go /home/omnigent/workspace/b9s/cmd/b9s/main.go` | 0, empty output | n/a | n/a | n/a |

The focused RED failed for the intended reason in all three bundle cases: asset checks were reached without DMG guidance. All seven focused cases passed within the GREEN package run. The stable suite ran alone, without baseline comparisons, and passed all 21 packages. Completed test runs had zero interrupted and zero unrun cases within their selected suites. No retries were used or flaky outcomes observed. An early progress summary incorrectly stated 208 updater passes. The next milestone corrected it to the measured 114 passes.

The 20 runtime skips cover optional real bd probes (1), S3 services (3), Dolt DSN (7), Memory preview/research environments (4), absent optional fixture files (2), and opt-in stress tests (3). These were existing suite conditions. No test was changed to skip.

Offline CLI commands:

```sh
timeout 10s env HTTPS_PROXY=http://127.0.0.1:1 HTTP_PROXY=http://127.0.0.1:1 /tmp/bd-wd87.4-evidence/cli-smoke/B9s.app/Contents/Resources/bin/b9s --update --yes
timeout 10s env HTTPS_PROXY=http://127.0.0.1:1 HTTP_PROXY=http://127.0.0.1:1 /tmp/bd-wd87.4-evidence/cli-smoke/usr/local/bin/b9s --update --yes
```

Both CLI invocations exited 1 with DMG instructions and the latest-release link, before the deliberately unavailable proxy could cause a network error. The bundle binary SHA-256 was unchanged after both commands: `e25e093d08044e65436527ad56b50f6b8fd04eca6a8336ebc499574b1e610367`. No additional files appeared beside the executable.

## Evidence and limitations

Evidence directory: `/tmp/bd-wd87.4-evidence/`.

- `preflight.txt`, `dependencies.txt`.
- `red.jsonl`, `red.exit`, `red-report.json`.
- `green.jsonl`, `green.exit`, `green-report.json`.
- `build.txt`, `stable.txt`, `stable.exit`, `stable-report.json`.
- `vet.txt`, `cli-build.txt`, `cli-smoke-report.json`, `final-diff.txt`.
- This report: `implementation-report.md`.

Runtime: `go version go1.25.5 linux/amd64`. Native macOS codesign and DMG installation were not exercised in this Linux container. Real executables and symlinks exercised the path guard. Browser tests do not apply to this CLI updater change. No web UI files changed.

Guard against recurrence: runtime rejection plus seven subprocess regression cases, with existing ordinary updater tests left intact. No new ADR was needed for this direct implementation of the assigned bundle policy. Independent reviewer and tester verdicts remain pending.

Unresolved required-check failures: none. The expected RED failures are resolved by GREEN. New issue IDs: none. Unrelated follow-up work: none filed. Delivery remains gated by Polly's authorization.
