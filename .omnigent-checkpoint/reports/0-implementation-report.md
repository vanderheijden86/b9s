# bd-p8zj implementation report: BLOCKED

## Result and changed files

Recovered available provenance evidence and documented the missing prerequisites.
Causal reproduction remains impossible from the inspected artifacts. No product,
hook, or configuration fix was attempted. No delivery commit, push or PR.

Only changed file (new):
`docs/post-mortems/2026-09-23-post-checkout-artifact-recovery.md`.
It records observed recovery results, distinguishes previous reports from current
measurements, and enumerates all missing acceptance prerequisites.
No new regression test or RED/GREEN: documentation/investigation only, no logic
change and no responsible incident executable or hook identified.

## Git preflight

Repository: /home/omnigent/workspace/b9s.
Origin: https://github.com/vanderheijden86/b9s.git.
Actual branch: omnigent/bd-p8zj. Assigned base and PR target: main.
HEAD and fetched origin/main: eb77cef9f8e68e9854f3c1b6ec6a6074b812830a.
Required commits: none. Required-commit ancestry against base and HEAD: N/A.
Additional origin/main-to-HEAD ancestry check exited 0 (equal).
Initial worktree clean. Exact delivery remote probe exited 2 (absent), so local
delivery branch was created from fetched origin/main. No delivery ref existed to
fetch. Git network calls were bounded with timeout 60s.
Checkpoint inspect for this task exited 2 with explicit no-checkpoint message.
Final worktree has only the new recovery document. HEAD and base unchanged.
`git diff --check` exited 0. No staged files.
Detailed preflight: /tmp/bd-p8zj-attempt2-preflight.txt.

beads-connect ran once and succeeded. Progress comments are attributed to Codex,
session ede09d9b575945e8858d03136620a07e,
implementer-codex:bd-p8zj-implementation. No task state was changed.

## Recovery and causal evidence

The full bd-i8ag lane report and issue description were read. bd-i8ag depends on
bd-p8zj, and is provenance, not a prerequisite blocker.

The referenced /tmp/bd-i8ag-reports directory is absent. Fetched exact unapproved
checkpoint refs/heads/omnigent-checkpoints/bd-i8ag to a remote-tracking ref,
without switching delivery branch or restoring its worktree. Remote probe and
fetch exited 0, each under timeout 60s. Ref resolves to
 d2c8ce2de3b1445b5d3b9cdc62ceb5767d91eae6.
Only its manifest and final report were saved, not the earlier diagnostic JSON,
release binary, incident executable or runtime. Current extracted copies are
prior-manifest.json and prior-implementation-report.md in this report directory.

Historical tracked shim recovered from last pre-incident main commit
6e05818d5c4980949fb98c2008a8a8949785da10. It is byte-identical to current,
SHA-256 cfaf5be55306380ad70b2831dc64f218b6006b8b3f8bff6d425e0a96c5e22752.
Historical tracked config already has backup.enabled=false. Machine-specific
metadata was untracked before incident, and post-checkout.old is not tracked.
This cannot establish the incident machine's files or executable selection.
Current Pod wrapper and binary checksums and PATH are in artifact-inventory.json.
The binary is v1.3.0-rc.1, revision 9c6a69ec12350959ec8c495c74eeb02902d629b6.
The wrapper defaults to it, with no BD_REAL override present. Neither is proven
to be the incident executable. Current core.hooksPath is unset and active
post-checkout and post-checkout.old files are absent from .git/hooks.

Exact recovery argv, exits, elapsed seconds and outputs are in
recovery-commands.json. All recorded recovery subprocesses finished under their
30-second deadlines. Library-cache query exited 0 in 0.0027 seconds and lists
ICU 76, no ICU 74. Matching runtime cannot be selected without incident binary
identity, format and library requirements. The prior stock Linux loader failure
is prior evidence, not a current reproduction.

No hook replay was executed in this attempt because the incident artifacts are
unavailable. The earlier 0.2446-second Pod probe and stock loader exit 127 are
explicitly attributed to the recovered prior report. A replacement stock-only
probe would not meet acceptance. No causal duration, final incident exit or
responsible diagnostic process was invented.
No suggested Dolt remote/push command, shared-server experiment, replacement
hook, system-package download or deletion command was executed.

## Fresh required checks

All commands ran from /home/omnigent/workspace/b9s. Dependencies installed first
from committed go.mod/go.sum, with no lockfile changes. Stable suite ran alone
on delivery tree, with no base-branch comparison or parallel build/vet.
Its declared Go package timeout is 10m, with existing bounded test helpers and
an outer 720s deadline. No timeout or retry occurred. Compiler CPU progress and
output freshness were observed. No previous task's test evidence was reused.

| Exact command | Exit | Elapsed seconds | Measured result |
| --- | --- | --- | --- |
| `timeout 180s go mod download` | 0 | 8.047 | Dependencies installed |
| `timeout 720s env GOFLAGS=-v make -C /home/omnigent/workspace/b9s test-stable` | 0 | 138.8668 | 2125 passed, 0 failed, 18 skipped, 0 interrupted, 0 unrun (top-level) |
| `timeout 180s go build ./...` | 0 | 19.7255 | No output |
| `timeout 180s go vet ./...` | 0 | 21.6071 | No output |

Stable suite: 22 packages. Including nested subtests, 3769 passed, 0 failed,
20 skipped. These counts are an alternative scope, not additive totals.
All 18 top-level skips are declared optional prerequisites: real bd capability
(1), S3 (3), Dolt DSN (7), Memory fixtures (4), PERF_TEST (3).
The Makefile exclusions and tests/e2e are outside the selected suite and not
counted as skipped or unrun. No flaky retry outcomes. One run cannot prove
absence of flakes. Build and vet have no test counts.

Reports: dependencies.json, test-stable-summary.json, test-stable-evidence.json,
test-stable.txt, build.json and vet.json. Runtime: Go 1.25.5 linux/amd64.
Host: Linux 6.8.0-149-generic x86_64. Container image digest unavailable.
No browser/UI changes, so no browser checks were required or executed.

## Every unresolved required acceptance failure

1. **bd-p8zj:** Actual incident executable identity, checksum and build/install
   provenance unavailable. Neither version marker nor Pod identity satisfies it.
2. **bd-p8zj:** Incident PATH selection, shell wrappers/functions, relevant
   overrides and active Git hook configuration unavailable.
3. **bd-p8zj:** Complete incident hook chain unavailable, including optional
   post-checkout.old or contemporaneous evidence of its absence.
4. **bd-p8zj:** Sanitized incident machine metadata/config/environment and a
   compatible isolated runtime for the identified incident executable unavailable.
   Prior stock Linux probe had missing ICU 74, but that does not identify the
   incident runtime or justify using a substitute binary.
5. **bd-p8zj:** Required causal replay of delay AND repair hint unrun without
   items 1-4. Exact original duration, final exit and responsible emitter remain
   unproved. No incident-path safety or resolution claim is made.

All dispatched baseline commands passed. No required baseline check failure
remains. The failures above are acceptance prerequisites, not test failures.
No separate issue was filed for work already within bd-p8zj. bd-i8ag and bd-6qp2
are not prerequisite blockers for this dispatch. Unrelated follow-up work: none
implemented or filed. The metadata-port warning was observed and left unchanged.

## Durability and handoff

Unapproved checkpoint save succeeded before validation, only to
refs/heads/omnigent-checkpoints/bd-p8zj. Source HEAD/index/delivery branch
were not committed or pushed. Final checkpoint save adds these measured reports.
A successful final save must be observed separately before claiming durability.
Return BLOCKED for review of recovered evidence and external prerequisites.
