# Implementation report: bd-ac05.1.1

Readiness: BLOCKED on required external installed-inventory evidence. The documentation draft is ready for independent review. No delivery commit, delivery push, or pull request was made.

## Changed files

- `docs/ecosystem.md` (new): nine-row component/store table, required versus observed versions, executable transcripts, reader/writer boundaries, upstream links, missing access prerequisites.
- No source, tests, dependencies, generated assets, or task ownership/status/dependencies changed. No files deleted.

## Git preflight

- Shared repository: `/home/omnigent/workspace/b9s`.
- Origin: `https://github.com/vanderheijden86/b9s.git`.
- Initial branch: `main`, clean.
- Initial HEAD: `3a5ed086abe347af005c06efc3fe3f2c7661af12`.
- `timeout 60s git -C /home/omnigent/workspace/b9s fetch origin main`: exit 0.
- Fetched `origin/main`: `3a5ed086abe347af005c06efc3fe3f2c7661af12`.
- `timeout 60s git -C /home/omnigent/workspace/b9s ls-remote --exit-code --heads origin refs/heads/omnigent/bd-ac05.1.1`: exit 2, absent. No local delivery branch existed.
- `git -C /home/omnigent/workspace/b9s switch -c omnigent/bd-ac05.1.1 origin/main`: exit 0.
- Actual branch: `omnigent/bd-ac05.1.1`.
- Actual HEAD remains `3a5ed086abe347af005c06efc3fe3f2c7661af12`.
- Required prerequisite commits: none. Ancestry against fetched base and HEAD: N/A.
- PR target: `main`.
- `lane-checkpoint inspect --repo /home/omnigent/workspace/b9s --task bd-ac05.1.1 --base origin/main`: exit 0, checkpoint absent before implementation.
- Report: `preflight.txt`.

## Executable and documentation evidence

The Pod's installed bd is `1.3.0-rc.1 (9c6a69ec1)`. Downloaded release executables were run by absolute path from a neutral directory after archive checksum checks:

- `/tmp/bd-ac05.1.1-evidence/bd-release/bd --version`: exit 0, `bd version 1.3.1 (c1c4b642a)`.
- `/tmp/bd-ac05.1.1-evidence/bd-rc-release/bd --version`: exit 0, `bd version 1.3.2-rc.2 (53637a74a)`.
- `/tmp/bd-ac05.1.1-evidence/gas-city-release/gc version`: exit 0, `1.5.0`.
- `/tmp/bd-ac05.1.1-evidence/b9s-release/b9s --version`: exit 0, `b9s v1.3.3`.

Archive SHA-256 values match the upstream checksum files. Download argv, executable argv, outputs and digests are preserved in `*-executable.json`. These downloaded artifacts do not establish host-installed versions.

`versions --help` and `links --help` exit 1 as unknown commands for both checked bd releases. `memories --help` exits 0 without `records-json`. These expected negative capability observations are not failed required tests. Full commands and results are in `released-memory-probes.json`.

Bounded `gh api` calls checked gastownhall/beads issues 5877 and 6535, the requested release tags, Gas City's release requirement for bd, and the b9s v1.3.3 datasource tree. Each exited 0. The b9s release tree lacks this checkout's proxied-server adapter. Reports: `beads-5877.json`, `beads-6535.json`, `gas-city-release.json`, `bd-release.json`, `bd-rc-release.json`, `b9s-release.json`, `b9s-release-sources.json`, and corresponding command files.

Each external document link was checked with `curl --location --fail --silent --show-error --max-time 25 --output /dev/null --write-out '%{http_code}' URL`. Local links were resolved against the document directory. Result: 14 passed, 0 failed, 0 skipped, 0 interrupted, 0 unrun. Exact URLs and argv are in `doc-links.json`.

Numeric/version and capability evidence assertions: 11 passed, 0 failed, 0 skipped. Report: `version-claims.json`. Unknown installed versions are explicitly excluded from that pass count and remain required blockers below.

## Required Go checks

All commands ran in `/home/omnigent/workspace/b9s`. Runtime: `go version go1.25.5 linux/amd64`.

| Exact command and environment | Exit | Measured result | Evidence |
|---|---|---|---|
| `timeout 180s go mod download` | 0 | Dependency installation succeeded using committed go.mod/go.sum | `dependencies.txt` |
| `GOMAXPROCS=4 GOFLAGS=-p=2 timeout 300s go build ./...` | 0 | Build/typecheck command: 1 passed, 0 failed, 0 skipped. No individual test count applies. | `build-result.json`, `build.txt` |
| `GOMAXPROCS=4 GOFLAGS='-p=2 -v' timeout 900s make test-stable` | 0 | 3759 passed, 0 failed, 20 skipped emitted test/subtest outcomes, including parents. Top-level: 2124 passed, 0 failed, 18 skipped. Packages: 22 passed, 0 failed. | `test-stable-result.json`, `test-stable.txt`, `test-stable-skips.json` |
| `GOMAXPROCS=4 GOFLAGS=-p=2 timeout 300s go vet ./...` | 0 | Vet command: 1 passed, 0 failed, 0 skipped. No individual test count applies. | `vet-result.json`, `vet.txt` |

The stable suite ran alone, without a base comparison or concurrent verification commands. The Makefile supplied its existing 10-minute package deadline. The outer deadline did not fire. No retries were run. Flakiness was not independently assessed.

The 20 conditional skip outcomes were: real binary preview probe 1, opt-in S3 3, Dolt DSN integration 7, preview timing 1, opt-in research 3, opt-in stress 3, absent local fixture subtests 2. The suite's explicit `STABLE_SKIP` exclusions and `tests/e2e` are outside the target and are not runtime skip counts. No tests were changed, skipped, or weakened by this task. The conditional external skips do not establish the missing installed-inventory acceptance evidence.

Docs-only scope requires no new regression tests, browser tests, local service deployment, new guard, or ADR. Go build covers Go typechecking. Go vet is the requested lint/static check. No frontend source changed.

## Self-review

Implementer self-review: 6 passed, 0 failed, 0 skipped. Independent review has not occurred. See `self-review.json` for the final document digest and checks. `git diff --check` exited 0. The untracked document was separately checked with `git diff --no-index --check /dev/null /home/omnigent/workspace/b9s/docs/ecosystem.md`, which exited 1 for a new-file diff and emitted no whitespace warnings.

## Every unresolved required blocker

All belong to the dispatched issue **bd-ac05.1.1**. No extra issue was filed for an acceptance prerequisite already in scope.

- **bd-ac05.1.1: operator installed tool inventory.** Need read-only laptop shell evidence for installed released/candidate bd, bd-prev, and b9s. The Pod's only installed bd reports a different version. bd-prev, gc, b9s, dolt, ssh, the documented preview executable, and the operator shell configuration are absent here.
- **bd-ac05.1.1: preview identity and readiness.** Need the actual preview executable/version or revision, neutral-directory capability probes, and ready workspace metadata. Downloading an ordinary bd release cannot substitute for this.
- **bd-ac05.1.1: big-hetzner Dolt inventory.** Need a reachable host identity, executable `dolt version`, and SELECT-only running-engine version. Pod name resolution fails with `Name or service not known`, and SSH is absent.
- **bd-ac05.1.1: local fallback inventory.** Need the operator laptop's Dolt executable/engine version and actual fallback endpoint on 3307. Pod `127.0.0.1:3307` refuses TCP and does not represent the laptop.
- **bd-ac05.1.1: actual city/rig inventory.** Need installed `gc version`, selected `bd --version`, city/rig metadata, applicable proxy/client-info record, and running engine version. No actual city location or executable access was provided.

Evidence: `installed-tools.json`, `version-claims.json`, and the document's access section. There are no failed Go checks and no failed documentation-link checks. There is no unrelated follow-up work.

## Durability and handoff

The first checkpoint save succeeded on the separate **unapproved** `refs/heads/omnigent-checkpoints/bd-ac05.1.1` ref. A final checkpoint save accompanies this report. Checkpoint evidence is advisory and does not substitute for the independent tester.

Delivery remains uncommitted. No delivery push and no PR are authorized. Acceptance must remain blocked until the missing external evidence is supplied and reviewed. The required Go checks do not resolve inaccessible host facts.
