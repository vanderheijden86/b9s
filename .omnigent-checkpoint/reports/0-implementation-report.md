# bd-cmjx implementation report

Implementation GREEN. Changed-source deployment acceptance remains pending under bd-cmjx until Polly authorizes the reviewed delivery commit. No delivery commit, delivery push, PR, namespace change or shared database test write was performed.

## Preflight

- Shared repository: /home/omnigent/workspace/b9s, origin https://github.com/vanderheijden86/b9s.git.
- `beads-connect` ran first and exited 0: connected b9s as bd_b9s.
- Initial branch main, clean tree. No local delivery branch existed.
- `timeout 60s git -C /home/omnigent/workspace/b9s fetch origin main`: exit 0.
- `git -C /home/omnigent/workspace/b9s show-ref --verify refs/remotes/origin/main`: origin/main exists at 873e74381482d2d7f73bccbc739d2c559daea050.
- `timeout 60s git -C /home/omnigent/workspace/b9s ls-remote --exit-code --heads origin refs/heads/omnigent/bd-cmjx`: exit 2, absent. No authentication or transport failure was treated as absence.
- `git -C /home/omnigent/workspace/b9s switch -c omnigent/bd-cmjx origin/main`: exit 0.
- Actual branch omnigent/bd-cmjx. HEAD and fetched base origin/main both 873e74381482d2d7f73bccbc739d2c559daea050. PR target main.
- Required commits: none. Prerequisite ancestry against fetched base and HEAD: not applicable.
- `lane-checkpoint inspect --repo /home/omnigent/workspace/b9s --task bd-cmjx --base origin/main`: no checkpoint existed.
- Evidence: /tmp/bd-cmjx/preflight.txt.

## Observed diagnosis and change

The historical report specified in the dispatch is absent in this Pod. The existing parent preview remains accessible through the mounted scoped kubeconfig. Its deployment uses the historical ttyd image at 4e3311ef70678f5f6b4e243447541e6f901ac027. Read-only entrypoint and manifest inspection prove that identity HTTP starts before ttyd and readiness checks identity alone. Parent logs record the eventual 7681 and 7683 listeners. These observations prove insufficient readiness coverage, but cannot establish the precise timing of the historical curl exit 7.

Current main follows active ADR 0058: b9s web on 7681 and the real TUI in tmux through kubectl exec. It retains the same identity-only readiness gap. The fix gates the actual current runtime rather than restoring retired ttyd. Both current full-ID headers already use complete issue IDs.

Files changed, exactly:

| File | Change |
| --- | --- |
| /home/omnigent/workspace/b9s/scripts/preview/lib.sh | Render an exec readiness probe requiring bounded local identity HTTP, authenticated web /api/session HTTP and a nonblank successful tmux capture. |
| /home/omnigent/workspace/b9s/scripts/preview/readiness_test.go | New regression in the owning package. It runs the rendered probe with controlled identity, web and TUI failures, plus recovery. Included automatically by make test-stable. |
| /home/omnigent/workspace/b9s/docs/preview-contract.md | Document the readiness conditions and deadlines. |

Each HTTP request has a one-second timeout. Kubernetes bounds the probe at four seconds, covering both requests and tmux. Existing rollout deadline remains in force. Baked commit identity and verifier identity comparisons are unchanged. The runtime probe uses the actual viewer environment and does not copy credentials into manifests.

Guard: the regression rejects identity-only readiness when web is down, viewer authentication fails, or the TUI is missing/blank. It also rejects readiness before web recovery. No new ADR was required because the existing ADR 0058 runtime is preserved.

## Exact checks and measured results

All paths below are absolute. Counts for Go regression/stable tests use leaf cases, avoiding double-counting parent tests. Build, vet, syntax and diff checks report command gates rather than invented test counts.

| Command | Exit and measured result | Evidence |
| --- | --- | --- |
| `timeout 180s go -C /home/omnigent/workspace/b9s mod download` | 0, dependencies installed from committed module files | /tmp/bd-cmjx/dependencies.txt |
| `timeout 180s npm --prefix /home/omnigent/workspace/b9s/web ci` | 0, dependencies installed from committed package-lock.json | /tmp/bd-cmjx/npm-ci.txt |
| `timeout 180s go -C /home/omnigent/workspace/b9s test ./scripts/preview -run '^TestPreviewReadiness$' -count=1 -timeout=30s -json` | RED exit 1: 2 passed, 5 failed, 0 skipped. Parent test also failed. All failures demonstrate identity-only false readiness. | /tmp/bd-cmjx/readiness-red.jsonl |
| `timeout 60s go -C /home/omnigent/workspace/b9s test ./scripts/preview -run '^TestPreviewReadiness$' -count=1 -timeout=30s -json` | GREEN exit 0: 7 passed, 0 failed, 0 skipped. Parent test also passed. | /tmp/bd-cmjx/readiness-green.jsonl |
| `timeout 120s bash /home/omnigent/workspace/b9s/tests/preview_contract_test.sh` | 0: 219 passed, 0 failed, 0 skipped | /tmp/bd-cmjx/preview-contract.txt |
| `timeout 180s go -C /home/omnigent/workspace/b9s build ./...` | 0: build gate passed | /tmp/bd-cmjx/build.txt |
| `timeout 180s go -C /home/omnigent/workspace/b9s vet ./...` | 0: vet gate passed, including Go type checks | /tmp/bd-cmjx/vet.txt |
| `GOFLAGS=-v timeout 660s make -C /home/omnigent/workspace/b9s test-stable` | 0: 23 packages passed, 0 failed. 3568 leaf tests passed, 0 failed, 20 skipped. 3777 PASS records including parent tests. Ran alone, without a base comparison. | /tmp/bd-cmjx/stable.txt and /tmp/bd-cmjx/stable-summary.json |
| `lane-browser smoke --project /home/omnigent/workspace/b9s/web --report /tmp/bd-cmjx/browser-smoke.json --seconds 90` | 0: 3 passed, 0 failed, 0 skipped | /tmp/bd-cmjx/browser-smoke.json |
| `lane-browser fingerprint --project /home/omnigent/workspace/b9s/web` | 0: Playwright 1.63.0, Node v22.23.4, x86_64, CPU affinity 0-3, native runtime SHA256 a32868da82a4a7bf3f0a736c570c8d28d79faba858fde32c43b6c2f9faa6b9a6 | /tmp/bd-cmjx/browser-environment.json |
| `lane-browser run --project /home/omnigent/workspace/b9s/web --seconds 90 -- /home/omnigent/workspace/b9s/web/node_modules/.bin/playwright test --config /tmp/bd-cmjx/browser-proof.config.ts` | Final run 0: 1 passed, 0 failed, 0 skipped, 0 flaky, 0 interrupted, 0 unrun. Existing parent exact commit/task/namespace checked over HTTP. Browser terminal connected and received real output. No unexpected page errors. | /tmp/bd-cmjx/browser-proof-r2.json and /tmp/bd-cmjx/browser-proof-r2.txt |
| `timeout 120s go -C /home/omnigent/workspace/b9s build -o /tmp/bd-cmjx/b9s ./cmd/b9s` | 0: current-source behavior binary built | /tmp/bd-cmjx/b9s |
| `B9S_WEB_BIN=/tmp/bd-cmjx/b9s B9S_KEEP_TEST_FIXTURES=1 lane-browser run --project /home/omnigent/workspace/b9s/web --seconds 120 -- /home/omnigent/workspace/b9s/web/node_modules/.bin/playwright test --config /tmp/bd-cmjx/local-proof.config.ts` | Final run 0: 2 passed, 0 failed, 0 skipped, 0 flaky, 0 interrupted, 0 unrun. Real header-auth web and real tmux TUI both show bd-readiness-proof.1. Web refuses missing viewer header and reports current HEAD with modified=true. No unexpected page errors. Each proof test has a 30-second timeout and each tmux operation a 3-second timeout. | /tmp/bd-cmjx/local-proof-r2.json and /tmp/bd-cmjx/local-proof-r2.txt |
| `gofmt -w /home/omnigent/workspace/b9s/scripts/preview/readiness_test.go` | 0 | Formatted source |
| `bash -n /home/omnigent/workspace/b9s/scripts/preview/lib.sh /home/omnigent/workspace/b9s/tests/preview_contract_test.sh` | 0: syntax gate passed | Source files |
| `git -C /home/omnigent/workspace/b9s diff --check` | 0: diff gate passed | Current diff |

Read-only parent runtime check also executed successfully:

```sh
timeout 15s kubectl --kubeconfig /mnt/secrets/preview/preview.kubeconfig --context preview --request-timeout=10s -n b9s-bd-iiem exec deploy/b9s -- sh -ec 'busybox | head -1; cat /etc/alpine-release; tmux -V; wget -q -T 1 -O /dev/null --header="X-Preview-Viewer: viewer@example.invalid" http://127.0.0.1:7682/__preview; printf "BusyBox bounded HTTP syntax: passed\n"'
```

Exit 0. Parent runtime: BusyBox 1.37.0, Alpine 3.24.1, tmux 3.7c. This confirms the request syntax in that runtime, not the deployment of changed current source. Report /tmp/bd-cmjx/parent-runtime.txt.

The 20 stable skips are declared optional host/service/data checks: real bd binary probing (1), S3 endpoint (3), Dolt DSN (7), isolated memory research/preview environments (4), unavailable optional real JSONL datasets (2), and PERF_TEST stress tests (3). Exact test names are in stable-summary.json. No required changed-code check skipped. Go stable uses its committed per-package 10-minute bound plus the outer 660-second bound. No Playwright retry was enabled. Web sources and bundle were unchanged, so no additional web lint/typecheck/build was required.

## Evidence corrections and limits

- Initial parent browser proof: exit 1, 0 passed, 1 failed, 0 skipped. It used a nonexistent .xterm-rows locator against a canvas terminal. The corrected harness observes real received WebSocket output. Initial evidence remains /tmp/bd-cmjx/browser-proof.json and browser-proof.txt. This was an evidence harness error, not a product failure or flaky result.
- Initial local proof could not import the repository TypeScript harness from a temporary CommonJS context. Exit 1, no tests executed, and lane-progress correctly rejected the incomplete JSON report. Adding type=module only to /tmp/bd-cmjx/package.json resolved it. Initial local-proof.json and local-proof.txt remain available. The fresh completed report is local-proof-r2.json.
- Parent evidence is read-only against the existing historical preview. It proves current reachability of that image, not reliable startup of the changed manifest.
- Current-source local behavior uses isolated JSONL fixtures and modified=true provenance. It does not claim a clean deployed image or database-backed startup proof.
- No historical report was reconstructed or fabricated. Parent entrypoint and logs are /tmp/bd-cmjx/parent-entrypoint.txt and parent-startup.log.
- Optional Docker-only preview-tui wrapper tests were not applicable: neither wrapper nor entrypoint changed. The actual local tmux TUI was exercised directly.
- Direct session messaging to Polly was refused because session orchestration is disabled. Attributed Beads comments and lane-progress carry the handoff.

## Checkpoint and handoff

The first unapproved checkpoint save succeeded on refs/heads/omnigent-checkpoints/bd-cmjx. A comparison against that initial checkpoint returned compatible=false, complete=false because it contained only RED reports and no passing evidence. No saved result was reused. Current results above were all executed in this attempt.

A final checkpoint save includes the new test source explicitly, this report, the completed focused evidence, stable summary and completed browser reports. Its result is recorded in /tmp/bd-cmjx/checkpoint-final.json. This is an unapproved checkpoint only, never the delivery ref.

Required pending acceptance: bd-cmjx, a clean reviewed changed-source installed lane-preview run must prove real deployment readiness and exact served identity after Polly's review/delivery authorization. The helper requires clean HEAD and the repository rejects uncommitted source. Running it against this dirty implementation would not test the changed source. No required local check remains failed.

Parent task bd-iiem is not delivered by this task. Full-ID behavior on the assigned current base was proved locally in browser and terminal. Unrelated deferred work bd-ky03 concerns terminal session exit after interaction and remains outside this change. No new follow-up issue was filed.

Ready for Polly's independent reviewer and tester. No delivery authorization was assumed.
