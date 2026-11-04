# bd-969k implementation report

Triage complete. Delivery commit, delivery push and PR are NOT authorized and were not performed. Independent review and tester checks remain Polly's next gates.

## Outcome

- Baseline audit reproduced: npm audit exit 1, 1 critical and 2 low package entries, four distinct advisories.
- Supported narrow patch: installed transitive DOMPurify 3.4.16 -> 3.4.17 within Mermaid's ^3.3.3 constraint. Only three lockfile values changed. No package.json change, overrides, major upgrade or application logic change.
- Final full and production-only audits BOTH exit 1: 0 critical, 0 high, 0 moderate, 2 low, 2 total package entries. These are not clean audit checks. The unchanged residual entries are KaTeX and its propagated Mermaid entry.
- The prebuilt Mermaid runtime asset still contains DOMPurify 3.4.12 and KaTeX 0.16.47. A lockfile patch does NOT patch that asset. Runtime components are not development-only. Source inspection narrows exploit prerequisites but does not establish full safety.
- Four distinct advisories are explicitly dispositioned in docs/security/web-dependency-audit.md, including installed/bundled versions, ranges, patch versions, dependency paths, exposure, supported remediation and residual risks. Raw before/after audit JSON is committed-source material beside the record.
- Deferred discovered-from issue bd-w8yz: [w8yz] Assess patched Mermaid runtime bundle and KaTeX upgrade. This is out-of-scope runtime maintenance for Polly to decide, not a required zero-findings blocker of this triage. npm proposes Mermaid 12.3.0, a major migration. Published dependency metadata is not proof its prebuilt asset is patched. No newer Mermaid 11 release was returned.

## Full Git preflight

Repository /home/omnigent/workspace/b9s, origin https://github.com/vanderheijden86/b9s.git.
Assigned base and PR target: main. Delivery: omnigent/bd-969k.

- Initial worktree clean, bootstrap main (not used as authority).
- `timeout 60s git -C /home/omnigent/workspace/b9s fetch origin main`: exit 0.
- `git -C /home/omnigent/workspace/b9s rev-parse --verify refs/remotes/origin/main`: exit 0, fetched base 3945b45ab6f3932b0195f3482d6cd0d2025d5ecb.
- `timeout 60s git -C /home/omnigent/workspace/b9s ls-remote --exit-code --heads origin refs/heads/omnigent/bd-969k`: exit 2, exact remote delivery ref absent. There was no remote delivery branch to fetch/resume.
- `git -C /home/omnigent/workspace/b9s show-ref --verify refs/heads/omnigent/bd-969k`: exit 128, local delivery branch absent.
- `git -C /home/omnigent/workspace/b9s switch -c omnigent/bd-969k origin/main`: exit 0.
- Actual branch: omnigent/bd-969k. HEAD: 3945b45ab6f3932b0195f3482d6cd0d2025d5ecb.
- Required commits: none. Required-commit ancestry against fetched base and HEAD: not applicable.
- `lane-checkpoint inspect --repo /home/omnigent/workspace/b9s --task bd-969k --base origin/main`: exit 0, checkpoint absent. No restore/reuse.
- Preflight published and attributed task comment posted before dependency edits. Branch and HEAD have not changed during implementation.

## Changed files

1. web/package-lock.json: three values patch DOMPurify to 3.4.17.
2. pkg/web/dist/source.sha256: generated input hash after the lockfile change.
3. docs/security/web-dependency-audit.md: dated durable evidence, exposure, disposition, follow-up and validation record.
4. docs/security/bd-969k-audit-before.json: raw baseline audit response.
5. docs/security/bd-969k-audit-after.json: raw current audit response.

No source logic, dependency constraints, existing ADRs or prebuilt Mermaid bytes changed. No files deleted. The generated bundle hash is guarded by the existing passing TestEmbeddedBundleIsCurrent. Dependency/config-only work does not require a new test-first logic regression. The baseline audit was collected before the patch. ADR 0031's runtime design remains unchanged.

## Commands and observed results

All cwd values below are /home/omnigent/workspace/b9s unless --prefix/--project selects web. All command deadlines were fixed before running and none expired.

| Exact command | Exit | Measured result |
| --- | --- | --- |
| `beads-connect` (bootstrap, run once) | 0 | bd connected to b9s as bd_b9s |
| `timeout 180s go mod download` | 0 | Committed Go dependencies downloaded |
| `timeout 180s npm --prefix /home/omnigent/workspace/b9s/web ci` (before) | 0 | 122 installed, 123 audited; 1 critical, 2 low |
| `timeout 120s npm --prefix /home/omnigent/workspace/b9s/web audit --json` (before) | 1 | 1 critical, 2 low, 3 total package entries |
| `timeout 60s npm --prefix /home/omnigent/workspace/b9s/web ls mermaid dompurify katex --all` (before) | 0 | mermaid 11.17.2 -> DOMPurify 3.4.16, KaTeX 0.16.47 |
| `timeout 60s npm view mermaid@11 version dependencies --json` | 0 | Latest returned 11.x is 11.17.2; KaTeX ^0.16.47 |
| `timeout 60s npm view mermaid@12.3.0 dependencies --json` | 0 | DOMPurify ^3.4.17, KaTeX ^0.18.0; candidate major upgrade |
| `timeout 60s npm view dompurify@3.4.17 version dist.integrity engines --json` | 0 | Patch metadata and integrity recorded |
| `timeout 60s npm --prefix /home/omnigent/workspace/b9s/web audit fix --dry-run --json` | 1 | Offered compatible DOMPurify patch; major Mermaid migration for KaTeX. Dry run is not clean audit evidence |
| `timeout 120s npm --prefix /home/omnigent/workspace/b9s/web update dompurify --package-lock-only --ignore-scripts` | 0 | Only DOMPurify version/URL/integrity changed |
| `timeout 180s npm --prefix /home/omnigent/workspace/b9s/web ci` (after) | 0 | Fresh install 122 packages, 123 audited; 2 low |
| `timeout 120s npm --prefix /home/omnigent/workspace/b9s/web audit --json` (after) | 1 | Critical 0/high 0/moderate 0/low 2/total 2 |
| `timeout 120s npm --prefix /home/omnigent/workspace/b9s/web audit --omit=dev --json` | 1 | Critical 0/high 0/moderate 0/low 2/total 2 |
| `timeout 60s npm --prefix /home/omnigent/workspace/b9s/web ls mermaid dompurify katex --all` (after) | 0 | mermaid 11.17.2 -> DOMPurify 3.4.17, KaTeX 0.16.47 |
| `timeout 120s npm --prefix /home/omnigent/workspace/b9s/web run typecheck` | 0 | Typecheck passed, test counts not applicable |
| `timeout 120s npm --prefix /home/omnigent/workspace/b9s/web run build` | 0 | Build passed, test counts not applicable |
| `timeout 60s npm --prefix /home/omnigent/workspace/b9s/web run test:guards` | 0 | 16 passed, 0 failed, 0 skipped, 0 cancelled |
| `timeout 660s env GOFLAGS=-v make -C /home/omnigent/workspace/b9s test-stable` | 0 | Alone, no comparison. Top-level 2085 passed/0 failed/18 skipped. Including subtests 3447 passed/0 failed/20 skipped. 22 passing packages, 0 failing packages |
| `timeout 300s go build ./...` | 0 | Build passed, test counts not applicable |
| `timeout 300s go vet ./...` | 0 | Vet passed, test counts not applicable |
| `lane-browser fingerprint --project /home/omnigent/workspace/b9s/web` | 0 | Fingerprint retained |
| `lane-browser smoke --project /home/omnigent/workspace/b9s/web --report /tmp/bd-969k/browser-smoke.json --seconds 90` | 0 | Chromium/Firefox/WebKit: 3 passed, 0 failed, 0 skipped |
| `PLAYWRIGHT_JSON_OUTPUT_FILE=/tmp/bd-969k/mermaid-results.json B9S_KEEP_TEST_FIXTURES=1 lane-browser run --project /home/omnigent/workspace/b9s/web --seconds 420 -- npm run test:e2e -- tests/mermaid.spec.ts --project pixel --project iphone --workers=1 --reporter=list,json --output=/tmp/bd-969k/mermaid-output` | 0 | Guard precheck 16 passed/0 failed/0 skipped. Playwright 10 passed/0 failed/0 skipped/0 flaky/0 interrupted/0 unrun |
| `git -C /home/omnigent/workspace/b9s diff --check` | 0 | No whitespace errors |

Stable skip reasons: optional real BD binaries (1), S3 (3), Dolt without B9S_TEST_DOLT_DSN (7), optional Memory preview/research (4), stress tests without PERF_TEST (3), absent real-project fixtures (2 subtests). No unexpected skips were found. Tests excluded by the stable Makefile contract are not run and are not counted as executed. Full terminal E2E/nightly browser matrix and production databases are outside this dispatch's checks.

Browser fingerprint: Node v22.23.4, Playwright 1.63.0, x86_64, kernel 6.8.0-147-generic, CPUs 0-3, 2 graphics threads, sandbox pod-boundary, native_runtime_sha256 a32868da82a4a7bf3f0a736c570c8d28d79faba858fde32c43b6c2f9faa6b9a6. An image digest was not supplied by this fingerprint, so none is invented. Go go1.25.5 linux/amd64, npm 10.9.9.

Browser suite covers sequence rendering, flowchart rendering, invalid-source fallback, malicious HTML labels and large diagrams. Existing malicious-label/large-diagram tests explicitly assert no page errors. No unexpected browser page errors were observed in these checks. Compatibility results do not prove every vulnerability configuration safe.

## Evidence paths

All files are under /tmp/bd-969k unless a repository path is named:

- preflight.txt
- dependencies.txt
- npm-ci-before.txt, npm-ci-after.txt, go-mod-download.txt
- audit-before.json, audit-after.json, audit-production-after.json (each actual audit exit 1)
- audit-fix-dry-run.json, npm-update-dompurify.txt
- dependency-tree-before.txt, dependency-tree-after.txt
- dompurify-metadata.json, mermaid-11-metadata.json, mermaid-12-metadata.json
- bundle-inspection.txt (source-map callsites and unchanged embedded bundle SHA-256)
- typecheck.txt, web-build.txt, web-guards.txt
- test-stable.txt (raw verbose log), test-stable-summary.json (measured counts and all skip contexts), stable-evidence.json
- go-build.txt, go-vet.txt
- browser-fingerprint.json, browser-smoke.json, browser-smoke.txt
- mermaid-results.json (fresh JSON for this completed run), mermaid-browser.txt, mermaid-exit.txt, browser-evidence.json
- diff-check.txt
- docs/security/bd-969k-audit-before.json and bd-969k-audit-after.json are the durable raw audit snapshots.

Lane progress published each check's start and actual completion. All npm audits were explicitly published as failed/nonzero observations with dispositions, not as passing audits. Attributed Beads milestones and the follow-up link are on bd-969k.

## Required blockers versus follow-up

Required implementation/build/test blockers: none observed. No required check timed out or remained incomplete.

Nonzero audit observations: full and production-only current audit exit 1, the two low package entries both assigned actionable disposition bd-w8yz. The original critical installed finding is patched in the lockfile. The embedded DOMPurify remains old and is also explicitly covered by bd-w8yz. Zero findings were not an acceptance requirement. The triage is complete without claiming these runtime dependencies are patched.

Unrelated follow-up: none filed. bd-w8yz is related deferred runtime maintenance outside this task's narrow triage change, and requires Polly's decision.

No delivery commit/push/PR. Checkpoint pushes use only refs/heads/omnigent-checkpoints/bd-969k. The checkpoint saves succeeded, including pre-validation saves. Final checkpoint save is reported separately after this report is written.
