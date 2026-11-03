# bd-b6jw.4 implementation report

Implementation completed. Required checks passed. Delivery remains uncommitted and unpushed as dispatched. No required blockers or unrelated follow-up issues were found or filed.

## Git preflight

- Shared repository: /home/omnigent/workspace/b9s
- Assigned base and PR target: main
- Actual branch: omnigent/bd-b6jw.4
- HEAD: 793407bd86c74ba9985c9870c39392974ec8380e
- Fetched origin/main: 793407bd86c74ba9985c9870c39392974ec8380e
- Required prerequisite commits: none. Ancestry checks against base and HEAD: not applicable.
- Initial checkout: clean main. Existing local delivery branch: absent.
- `timeout 60s git -C /home/omnigent/workspace/b9s fetch origin main`: exit 0.
- `git -C /home/omnigent/workspace/b9s rev-parse --verify refs/remotes/origin/main`: exit 0, SHA above.
- `timeout 60s git -C /home/omnigent/workspace/b9s ls-remote --exit-code --heads origin refs/heads/omnigent/bd-b6jw.4`: exit 2, delivery ref absent.
- `git -C /home/omnigent/workspace/b9s switch -c omnigent/bd-b6jw.4 origin/main`: exit 0.
- `lane-checkpoint inspect --repo /home/omnigent/workspace/b9s --task bd-b6jw.4 --base origin/main`: exit 2, no checkpoint exists.
- `beads-connect`: exit 0, ran once before Git preflight.
- Final branch, HEAD and fetched base remain as above. Only the three scoped files are modified. No staged changes.

## Changed files

- scripts/preview/preview-verify: select ingress only when the initial Service identity request returns curl 6, 7 or 28. Keep both HTTP probes on the selected boundary. Preserve all immutable identity comparisons and Git artifact guards. Report malformed identity through the final counters. Include URL, curl exit and operator action in transport failures.
- tests/preview_contract_test.sh: add 28 bounded verifier scenarios executing the real script and Python parser. Record exact HTTP destinations and task namespace terminal access. Cover all three fallback codes, excluded errors, each identity field, invalid JSON/schema, unavailable ingress, and board failures on both boundaries.
- docs/preview-contract.md: describe Service-first selection, timeout limits, identity rejection, consistent board routing and simulated coverage.

ADR 0058 is the current contract. The board uses HTTP and the terminal uses kubectl exec in the task namespace. Historical browser-terminal assumptions do not apply. ADR 0058 and the existing immutable image identity design remain unchanged. No new ADR is needed for this routing fix.

## Verification

All counts below are measured after command exit. No retries were used.

| Exact command | Exit | Result | Evidence |
| --- | --- | --- | --- |
| `timeout 180s go -C /home/omnigent/workspace/b9s mod download` | 0 | Committed module versions/checksums installed before checks | dependencies.log |
| `timeout 120s /home/omnigent/workspace/b9s/tests/preview_contract_test.sh` before production edits | 1 | Expected RED: 176 passed, 42 failed, 0 skipped assertions | contract-red.log |
| `timeout 120s /home/omnigent/workspace/b9s/tests/preview_contract_test.sh` after production edits | 0 | GREEN: 218 passed, 0 failed, 0 skipped assertions | contract-green.log |
| `env GOFLAGS=-v timeout 660s make -C /home/omnigent/workspace/b9s test-stable` | 0 | Top-level tests: 2057 passed, 0 failed, 18 skipped. Subtests separately: 1288 passed, 0 failed, 2 skipped. 21 packages passed | test-stable.log, test-stable-summary.json |
| `timeout 240s go -C /home/omnigent/workspace/b9s build ./...` | 0 | 1 build command passed, 0 failed, 0 skipped | build.log |
| `timeout 240s go -C /home/omnigent/workspace/b9s vet ./...` | 0 | 1 vet command passed, 0 failed, 0 skipped | vet.log |
| `timeout 10s bash -n /home/omnigent/workspace/b9s/scripts/preview/preview-verify` | 0 | 1 syntax check passed, 0 failed, 0 skipped | syntax-whitespace.log |
| `timeout 10s bash -n /home/omnigent/workspace/b9s/tests/preview_contract_test.sh` | 0 | 1 syntax check passed, 0 failed, 0 skipped | syntax-whitespace.log |
| `timeout 10s git -C /home/omnigent/workspace/b9s diff --check` | 0 | 1 whitespace check passed, 0 failed, 0 skipped | syntax-whitespace.log |

Evidence directory: /tmp/bd-b6jw.4-evidence

Stable suite ran alone with no base comparison. GOFLAGS=-v enables measured per-test reporting without changing the Makefile filter. The stable suite excludes tests/e2e and the Makefile STABLE_SKIP names, which are not counted as runtime skips. The 18 runtime top-level skips comprise optional real Memory binary probes (1), S3 endpoints (3), Dolt DSN (7), Memory preview workspace (1), Memory research opt-ins (3), and performance stress opt-ins (3). Two subtests skip absent local JSONL fixture files. These existing opt-ins are unrelated to the verifier fix. No test was skipped to obtain GREEN.

Each added verifier invocation has a five-second deadline. HTTP stubs require the actual three-second connection and ten-second total timeout arguments. Production attempts at most two identity requests. Build supplies Go type checking and vet supplies Go static checks. No Go source changed, so gofmt changes were unnecessary. No TypeScript or browser UI changed. Browser suites and live deployment were not run or claimed. Network boundaries were simulated through the real verifier, without cluster writes or a shared database test write.

## Guard and limits

Executable regressions reject cluster-only laptop routing, cross-boundary approval, fallback on HTTP/identity failures, and malformed identity without final counts. The immutable SHA/task/namespace checks and baked image identity remain effective. Actual workstation ingress reachability still depends on the operator's configured DNS, port, VPN or port-forward. Failures identify that boundary and remain bounded.

## Checkpoint and handoff

An initial checkpoint save succeeded at refs/heads/omnigent-checkpoints/bd-b6jw.4 before Go validation. It is separate, unapproved retry storage. A final save records this report and stable-suite evidence, with its actual result in checkpoint-final.log. No completed check was repeated. Independent reviewer and tester checks remain Polly's next roles. No task status, assignee, dependency or lane label changes were made. No delivery commit, delivery push, PR or merge was performed.
