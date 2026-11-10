# bd-a5xi.3 implementation report

Status: implementation and requested verification complete, awaiting independent review. No delivery commit, delivery push or PR creation was performed. No required-check blockers or new discovered issues.

## Git preflight

Repository: /home/omnigent/workspace/b9s

- Initial branch: main, clean worktree.
- Initial HEAD, final HEAD and fetched origin/main: c2fccef8b537ed034ebd360ea7cf31c02250e083.
- Actual delivery branch: omnigent/bd-a5xi.3, created from fetched origin/main.
- `timeout 60s git fetch origin main`: exit 0.
- `git rev-parse --verify refs/remotes/origin/main`: ref exists.
- `timeout 60s git ls-remote --exit-code --heads origin refs/heads/omnigent/bd-a5xi.3`: exit 2, remote delivery absent. No remote branch existed to fetch/resume.
- Local delivery branch was absent. `git switch -c omnigent/bd-a5xi.3 origin/main` succeeded.
- Required commits: none. Required ancestry checks: not applicable.
- Dispatch metadata and saved comments agree with this base and branch. Initial checkpoint lookup reported absent.
- Final changes are limited to the four files below. HEAD and index remain unchanged.

## Changed files and decisions

- scripts/preview/Dockerfile: optional WORKTREE_DIRTY and WORKTREE_HASH build arguments, validated baked /app/.source.json, OCI revision and worktree labels. Existing committed-preview interface and entrypoint remain supported.
- scripts/wt-image/main.go: runnable Go build helper. Freeze current tracked and nonignored untracked source, apply literal preview exclusions, preserve current bytes and executable modes, reject escaping symlinks, and compute a deterministic SHA-256 tree hash. Build from that frozen context. Push a local registry through Docker or BuildKit, or import into one explicit k3d cluster. Compare baked metadata before success. Retain evidence and use bounded process-group execution.
- scripts/wt-image/main_test.go: test-first snapshot regressions and command-routing/failure tests using disposable Git repositories and tool stubs.
- scripts/wt-image/README.md: runnable commands, metadata/hash contract, registry mirror requirements, limits and sibling-task integration boundaries.

The helper uses the committed vendor tree and preview Dockerfile without adding dependencies. Frozen source prevents edits during a long build from changing the bytes consumed by the builder. Image tags contain commit prefix, dirty flag and full source hash. Source manifests permit independent recomputation. No new ADR is needed for this build helper. The epic's writable sample deployment decision belongs to bd-a5xi.7.

## Commands and measured results

All final checks exited 0. Commands ran in /home/omnigent/workspace/b9s unless paths specify otherwise.

| Exact command | Result | Evidence |
| --- | --- | --- |
| `timeout 180s go mod download` | Exit 0 | dependencies.txt |
| `timeout 60s go test ./scripts/wt-image -count=1 -timeout=30s -json` | 13 passed, 0 failed, 0 skipped | focused-2.jsonl, focused-summary.json |
| `timeout 180s go build ./...` | 1 build passed, 0 failed | go-build.exit, build-summary.txt |
| `timeout 180s go vet ./...` | 1 vet command passed, 0 failed | go-vet.exit, vet-summary.txt |
| `GOFLAGS=-v timeout 660s make test-stable` | 2137 top-level passed, 0 failed, 18 skipped | test-stable.log, test-stable.exit, stable-final-summary.json |
| `timeout 180s bash /home/omnigent/workspace/b9s/tests/preview_contract_test.sh` | 219 passed, 0 failed, 0 skipped | preview-contract.log, preview-contract.exit |
| `git diff --check` | Exit 0 | final working tree inspected |
| `gofmt -l /home/omnigent/workspace/b9s/scripts/wt-image/main.go /home/omnigent/workspace/b9s/scripts/wt-image/main_test.go` | Exit 0, no files listed | final files inspected |

Stable counts including nested tests and fuzz seeds: 3772 passed, 0 failed, 20 skipped. These counts include parent and child results and must not be added to the top-level totals. The selected suite ran alone, without base-branch comparisons. Makefile exclusions remained intact. The 18 optional top-level skips require real bd builds, S3, Dolt, isolated Memory workspaces/research setup or opt-in ten-minute stress tests. Two additional subtest skips concern absent external srps/cass fixture files. Exact names and reasons are recorded in stable-final-summary.json. No retries ran. Flakiness was not separately assessed.

RED evidence: red.jsonl exits 1 because capture was not implemented. The isolated escaping-parent regression in red-parent-isolated.jsonl exits 1 with `tracked file copied through an escaping symlink parent`. GREEN focused-2.jsonl includes the fix and all 13 passing tests. An earlier parent test did not isolate the behavior, so its passing result was not used as RED proof.

Go build supplies type checking and go vet supplies static checking. No TypeScript or UI behavior changed. Browser tests are outside this task's required scope.

## Real image and runtime proof

Exact build command:

```sh
timeout 620s go run ./scripts/wt-image --repo /home/omnigent/workspace/b9s --builder buildkit --delivery registry --buildkit-addr tcp://buildkitd.buildkit.svc.cluster.local:1234 --registry registry.registry.svc.cluster.local:5000 --insecure-registry --output /tmp/bd-a5xi.3-image-1
```

Actual exit: 0. Local registry push completed. The helper also exported the runtime filesystem and checked its source metadata.

- Image: registry.registry.svc.cluster.local:5000/b9s-worktree:wt-c2fccef8b537-true-63f38b902a78b29d149c0ba84471177b4cfd3e24d452f1a0563e805247c1e245
- Image digest: sha256:fff3d652f87bb4e6cc43b019dd1d0df542fce8479836f4f459038bf5ec548bb2
- Baked commit: c2fccef8b537ed034ebd360ea7cf31c02250e083
- Baked dirty flag: true
- Tree SHA-256: 63f38b902a78b29d149c0ba84471177b4cfd3e24d452f1a0563e805247c1e245
- Captured files: 6042.
- BuildKit: v0.17.2 a3d734228b860a215f8a336e3983d35cbfaf08d8.
- Worker: zmxt9zpcpp8nk4vt2j78l2sw1, linux/amd64 supported.
- Go: go1.25.5 linux/amd64.

A bounded Python proof (`timeout 45s python3 -`, script supplied through stdin) completed 9 assertions, 0 failed, 0 skipped. It matched HEAD/dirty state, recomputed every manifest file hash and mode against the frozen context and current source, recomputed the tree hash, compared both baked files, fetched the pushed registry manifest/config, compared digest and labels, and executed the exported image binary. The baked source file is absent from the committed Dockerfile and present in the runtime, proving that the uncommitted Dockerfile changes entered the real build.

Exact runtime binary command:

```sh
/tmp/bd-a5xi.3-image-1/runtime/lib/ld-musl-x86_64.so.1 --library-path /tmp/bd-a5xi.3-image-1/runtime/lib:/tmp/bd-a5xi.3-image-1/runtime/usr/lib /tmp/bd-a5xi.3-image-1/runtime/usr/local/bin/b9s --version
```

Exit 0, output `b9s dev`. Source identity is recorded in /app/.source.json and OCI labels. The registry manifest/config and all nine assertion names are retained in runtime-proof.json, registry-manifest.json and registry-config.json.

## Scope and residual risks

- Registry delivery was exercised with real BuildKit and the in-cluster local registry. Docker and k3d alternatives were checked with tool stubs because this Pod has neither Docker nor k3d.
- Runtime proof used the exported image filesystem. No pod deployment or namespace resources were created. The adjacent tasks own stack deployment, disposable Dolt, shell access and running-pod/worktree comparison.
- The reused image does not contain bd. Sample write support remains with the adjacent tasks, as documented by the helper.
- No shared database writes or production/acceptance resources were used.
- Required-check failures: none. Newly discovered issue IDs: none. Existing sibling work remains bd-a5xi.1, .2, .4, .5, .6 and .7.
- Independent reviewer and tester checks remain pending. No delivery authorization has been assumed.

## Evidence and checkpoint

Primary evidence: /tmp/bd-a5xi.3-evidence/
Image evidence, full frozen context, file manifest and exported runtime: /tmp/bd-a5xi.3-image-1/
Git preflight: /tmp/bd-a5xi.3-evidence/preflight.txt
Unapproved checkpoint ref: refs/heads/omnigent-checkpoints/bd-a5xi.3. The initial checkpoint save succeeded. Final save publication is recorded separately after this report is written. This ref is separate from the unmodified delivery branch.
