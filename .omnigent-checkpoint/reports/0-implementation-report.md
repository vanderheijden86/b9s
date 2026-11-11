# bd-i8ag implementation report: blocked

No repository files changed. No delivery commit, delivery push or PR was made.
The diagnostic checkpoint is unapproved and uses a separate checkpoint ref.

## Git preflight

- Shared repository: /home/omnigent/workspace/b9s.
- Assigned base and PR target: main.
- Actual branch: omnigent/bd-i8ag.
- HEAD: eb77cef9f8e68e9854f3c1b6ec6a6074b812830a.
- Fetched origin/main: eb77cef9f8e68e9854f3c1b6ec6a6074b812830a.
- `timeout 60s git -C /home/omnigent/workspace/b9s fetch origin main` exited 0.
- `git -C /home/omnigent/workspace/b9s rev-parse --verify refs/remotes/origin/main` found the assigned base.
- `timeout 60s git -C /home/omnigent/workspace/b9s ls-remote --exit-code --heads origin refs/heads/omnigent/bd-i8ag` exited 2: absent.
- Local delivery branch was absent. Created with `git switch -c omnigent/bd-i8ag origin/main`.
- Required commits and ancestry checks: none.
- Initial and final worktree: clean, no unrelated work.
- Initial `lane-checkpoint inspect` exited 2: no checkpoint exists.

## Diagnostic evidence

The repository's generated `.beads/hooks/post-checkout` delegates to the external
`bd hooks run post-checkout`. Its 300-second timeout is a limit, not an idle delay.
This repository does not implement that command or contain the repair diagnostic.

Upstream v0.63.3 resolves to `884cc1171e3eb3ee44c1762973dfb8da6b56cdcc`.
The source was read at that exact tag, not inferred from current documentation:

- [Hook handler](https://github.com/gastownhall/beads/blob/884cc1171e3eb3ee44c1762973dfb8da6b56cdcc/cmd/bd/hooks.go): `runPostCheckoutHook` only runs a chained `post-checkout.old` when present. `hooksRunCmd` exits through `os.Exit`, bypassing the root post-command auto-push path.
- [Startup](https://github.com/gastownhall/beads/blob/884cc1171e3eb3ee44c1762973dfb8da6b56cdcc/cmd/bd/main.go): the `hooks` parent skips full database initialization.
- Searching the complete extracted upstream release source found no `beads: repair:` diagnostic.
- The Pod has no configured `core.hooksPath` and no incident `post-checkout.old`.
- The shim version marker does not establish which executable or shell wrapper ran during the incident.

The Pod's installed bd is `1.3.0-rc.1 (9c6a69ec1: HEAD@9c6a69ec1235)`.
A direct hook probe against isolated server-mode metadata targeting unreachable
`127.0.0.1:1`, without shared credentials, exited 0 in 0.2446 seconds.
It printed a directory-permission warning and no Dolt-remote advice.
This different executable is not a reproduction or proof of an incident fix.

The official Linux v0.63.3 archive matches the release's SHA-256:
`5f4efd2e010209b3f381dbcd783b2a3a652f50ea72f40ef04c8ba434d408bf9e`.
Its embedded build revision matches the source tag. The binary cannot start in
this image because `libicui18n.so.74` is missing. Version, direct hook and shim
attempts all exited 127 at the dynamic loader. No CLI check ran in those attempts.
The three attempts each had a 15-second subprocess deadline. No system packages,
Dolt remotes or shared-server experiments were used.

Reports:

- `/tmp/bd-i8ag-reports/preflight.txt`
- `/tmp/bd-i8ag-reports/diagnosis.txt`
- `/tmp/bd-i8ag-reports/isolated-hook.json`: exact argv, isolated environment, loader errors.
- `/tmp/bd-i8ag-reports/current-hook.json`: exact current-binary probe and measured duration.
- `/tmp/bd-i8ag-reports/stock-runtime.txt`: loader dependencies and binary build revision.

## Required checks

Commands ran from `/home/omnigent/workspace/b9s` unless `-C` is explicit.
Dependencies were installed before any check using the committed go.mod/go.sum.
No lockfile changed. Go: go1.25.5 linux/amd64. Host: Linux 6.8.0-149-generic x86_64.
Container image digest is unavailable.

| Command | Exit | Observed result |
| --- | --- | --- |
| `timeout 180s go mod download` | 0 | Dependencies installed |
| `timeout 720s env GOFLAGS=-v make -C /home/omnigent/workspace/b9s test-stable` | 0 | 2125 top-level passed, 0 failed, 18 skipped, 0 interrupted, 0 unrun within the selected suite |
| `timeout 180s go build ./...` | 0 | Build completed, test counts not applicable |
| `timeout 180s go vet ./...` | 0 | Vet completed, test counts not applicable |
| Focused bug RED/GREEN | Not run | Required blocker bd-i8ag: no causal reproduction or owned fix |

The stable target ran alone, with its declared 10-minute Go package deadline
and a 720-second outer deadline. Compiler progress was observed during the cold
build. No timeout occurred. GOFLAGS=-v records test outcomes without changing the
selected suite. The selected suite includes 22 packages.

Including nested subtests, stable results are 3769 passed, 0 failed, 20 skipped.
These are alternative counting scopes, not additive totals. Property iterations
and console sequence numbers were not counted as Go tests. No retry ran, so no
flaky outcome was identified. A single run does not establish absence of flakes.

The 18 top-level skips are declared opt-in tests: real bd capability (1), S3 (3),
Dolt DSN (7), Memory preview/research fixtures (4), and PERF_TEST stress tests (3).
Makefile exclusions and tests/e2e are outside the selected stable suite and are
not represented as reported skips. No browser/UI changes or browser suite ran.

Raw completed output: `test-stable.txt`, `build.txt`, `vet.txt` in the report folder.
Measured stable summary: `/tmp/bd-i8ag-reports/test-stable-summary.json`.

## Unresolved required blockers

All unresolved required acceptance failures belong to **bd-i8ag**:

1. The actual cause of the two-minute wait remains unproved. The incident's resolved executable, shell wrapper, chained hooks and sanitized startup configuration are absent.
2. The reported repair hint's emitter is unidentified. Its absence from this repository and inspected stock source prevents an evidence-based server-mode fix.
3. The stock incident-version probe is incomplete: missing `libicui18n.so.74`, loader exit 127. This is a runtime prerequisite failure, not a successful hook check.
4. Focused RED/GREEN and a regression guard remain unrun because there is no causal reproduction or repository-owned fix to exercise.
5. Safe behavior for the incident path and an affected documentation change have not been established. No claim of acceptance success is made.

Polly needs the incident executable/wrapper provenance and matching isolated runtime
before resuming the diagnosis. No source workaround was made, as dispatched.
No separate issue was created because these are unresolved parts of bd-i8ag.

Unrelated follow-up work: none implemented or filed. The existing metadata-port
deprecation warning was observed during bd progress comments and left unchanged.

## Handoff

Return for review of this blocked diagnosis. The evidence narrows ownership and
records the unavailable prerequisites, but does not identify the delay's root cause.
No repository diff is ready for approval. Delivery remains unapproved.
