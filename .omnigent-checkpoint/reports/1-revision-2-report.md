# bd-teri.13 implementation revision 2

Verdict: READY_FOR_REVIEW.
Reported: 2026-11-11 05:28 CET.

## Finding and change

Addressed required finding missing-docs, issue ID bd-teri.13, from reviewer-report.md. The previous ADR omitted a required interaction diagram. Its documentation checker incorrectly treated tables as an exemption. That exemption and the round-1 documentation-compliance conclusion are superseded by this report.

Only repository file changed: /home/omnigent/workspace/b9s/docs/adr/0066-open-specification-references-with-explicit-snapshot-import.md.
Added one six-participant Mermaid sequence diagram showing separate explicit opening and import actions, guarded source access, no upload or Beads write on open, bounded snapshot acquisition, upload before reference comment, upload failure without a comment, comment failure with manual retry, and orphan handling through GC grace.
The ADR remains proposed. No feature implementation, active-ADR edit, deletion, dependency-manifest change, delivery commit/push/PR or GitHub communication occurred.
Temporary evidence checker /tmp/bd-teri.13-evidence/check-docs.py now requires the interaction sequence, its policy rules, and successful render evidence with matching input/output SHA-256 values. The exact revised checker is archived as revision-2-check-docs.py. Round-1 checker and report are retained under round-1-* filenames.

## Git context

Actual branch: omnigent/bd-teri.13.
Preserved HEAD: c2fccef8b537ed034ebd360ea7cf31c02250e083.
Fetched origin/main: 873e74381482d2d7f73bccbc739d2c559daea050.
Remote delivery ref remains absent (bounded ls-remote exit 2). Required commits: none. Required-commit ancestry against both refs: not applicable.
HEAD ancestor of fetched main: exit 0. Fetched main ancestor of HEAD: exit 1.
Main advanced one unrelated commit, 873e7438, changing the edit-modal footer and its tests. No ADR or spec-reference path changed. The existing branch and untracked work were preserved without merge, rebase or reset.
Checkpoint inspection found the changed base SHA. The saved ADR was read from checkpoint commit 4649ae33cefad7b1d3d29bc9aa7c19ae6f5c6ba1 and matched the preserved pre-revision file exactly (cmp exit 0). No checkpoint restore occurred. A new unapproved checkpoint save succeeded after reviewing that context change.
Detailed commands and ancestry results: revision-2-preflight.txt.

## New checks

All commands ran from the shared repository unless --project or --prefix specifies otherwise. All deadlines were bounded. No new check was interrupted or left unrun.

1. Lockfile dependencies:
   timeout 180s env PATH=/tmp/bd-teri.13-evidence/bin:$PATH PLAYWRIGHT_SKIP_BROWSER_DOWNLOAD=1 PUPPETEER_SKIP_DOWNLOAD=1 npm --prefix /home/omnigent/workspace/b9s/web ci
   Exit 0. Project manifests unchanged. Temporary npm/npx launchers point to the image's installed npm JS entrypoints. No browser or system-package download.
2. Runtime fingerprint:
   lane-browser fingerprint --project /home/omnigent/workspace/b9s/web
   Exit 0. Node v22.23.4, Playwright 1.63.0, native runtime SHA-256 a32868da82a4a7bf3f0a736c570c8d28d79faba858fde32c43b6c2f9faa6b9a6. Pinned Chromium path is in revision-2-puppeteer.json. Browser sandbox boundary: existing Pod.
3. Browser smoke:
   timeout 100s lane-browser smoke --project /home/omnigent/workspace/b9s/web --report /tmp/bd-teri.13-evidence/revision-2-browser-smoke.json --seconds 90
   Exit 0. 3 passed, 0 failed, 0 skipped: Chromium, Firefox and WebKit.
4. Required Mermaid rendering:
   timeout 240s env PATH=/tmp/bd-teri.13-evidence/bin:$PATH PUPPETEER_SKIP_DOWNLOAD=1 lane-browser run --project /home/omnigent/workspace/b9s/web --seconds 220 -- npx -y @mermaid-js/mermaid-cli -i /tmp/bd-teri.13-evidence/revision-2-sequence.mmd -o /tmp/bd-teri.13-evidence/revision-2-sequence.svg -p /tmp/bd-teri.13-evidence/revision-2-puppeteer.json -b transparent -t dark
   Exit 0. 1 render passed, 0 failed, 0 skipped. CLI 12.2.0, Mermaid 12.3.0, Puppeteer 25.17.0. Output is a valid 41445-byte SVG. The render used the exact diagram extracted from the ADR. Actual exit code and SHA-256 values are retained in revision-2-mermaid-render.json.
5. Affected Go documentation tests:
   timeout 90s go test ./tests/docs -count=1 -timeout=60s -json
   Exit 0. 3 passed, 0 failed, 0 skipped. All three start events have completed outcomes. JSON output is revision-2-go-docs.jsonl.
6. Revised documentation checks:
   timeout 30s python3 /tmp/bd-teri.13-evidence/revision-2-check-docs.py
   Exit 0. 11 passed, 0 failed, 0 skipped. Includes YAML/frontmatter, monotonic numbering, required sections, TOC threshold, local links, original issue evidence, existing follow-up links, text/whitespace rules, six-participant interaction contracts, diagram label rules, and matching successful render evidence.

Resolved tooling problems: fingerprint discovery at the Go repository root could not find Playwright before selecting the web project. The image's npm shell entrypoint searched an incorrect module path and exited 1. Using its installed JS entrypoint through temporary launchers resolved this. The successful lockfile install and three-browser smoke supersede those setup attempts. No required environment failure remains.

## Retained results and limits

The dispatch explicitly retained existing build/vet/stable results. No full-suite rerun occurred because this revision changes documentation only. These results apply to the preserved delivery HEAD and original run, not to the newer fetched main:

- timeout 300s go build ./...: exit 0.
- timeout 300s go vet ./...: exit 0.
- timeout 720s env GOFLAGS=-v make test-stable: exit 0, 22 uncached packages, 2124 top-level passed, 0 failed, 18 optional skipped.
- Including nested outcomes: 3759 passed, 0 failed, 20 skipped. All 3779 starts completed. No retries were run. Skip details remain in test-stable-detailed.json.

Independent tester must execute the required checks next. No claim is made that the full suite ran against the final revised ADR or newly fetched main.

## Follow-ups and risk

Existing tickets bd-cbs3, bd-psgc, bd-0alf and bd-6oqw were read for evidence only. Their hierarchy, statuses, dependencies and assignees were not changed in this revision. Their original discovered-from:bd-teri.13 links remain.

Required blockers: none remaining from implementation checks. The reviewer must independently assess closure of missing-docs on bd-teri.13.
No unrelated follow-up issue or new task was created.
Residual risk: the diagram documents proposed interactions, not implemented behavior. Cross-VM reachability and executable security/import coverage remain future feature work in the existing tickets. The branch remains at its preserved HEAD while main is one commit ahead. Delivery authorization remains absent.

## Evidence

All evidence is under /tmp/bd-teri.13-evidence/:

- reviewer-report.md, revision-2-preflight.txt, revision-2-adr.diff.
- revision-2-sequence.mmd, revision-2-sequence.svg, revision-2-puppeteer.json.
- revision-2-mermaid-render.txt, revision-2-mermaid-render.exit, revision-2-mermaid-render.json.
- revision-2-browser-fingerprint.json, revision-2-browser-smoke.json, revision-2-browser-smoke.txt.
- revision-2-go-docs.jsonl, revision-2-go-docs.exit, revision-2-go-docs-summary.json.
- revision-2-check-docs.py, revision-2-docs-check.json, revision-2-follow-ups.json.
- revision-2-npm-ci.txt (failed launcher), revision-2-npm-ci-retry.txt (successful install).
- Original build.txt, vet.txt, test-stable.txt, test-stable-detailed.json retained unchanged.

Checkpoint refs are unapproved durability only. They never replace independent review/testing or update the delivery branch.
