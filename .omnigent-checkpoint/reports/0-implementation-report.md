# bd-a5xi.3 revision 2 implementation report

Status: round 1 contract-break resolved in implementation. All requested checks completed successfully. Independent re-review remains pending. No delivery commit, delivery push or PR creation was performed.

## Preflight

Repository: /home/omnigent/workspace/b9s. Actual branch: omnigent/bd-a5xi.3. Assigned base/PR target: main.

HEAD and freshly fetched origin/main: c2fccef8b537ed034ebd360ea7cf31c02250e083. `timeout 60s git fetch origin main` exited 0. The exact remote delivery probe exited 2 (absent). Existing local delivery branch and all round 1 uncommitted work were preserved. Initial status was modified scripts/preview/Dockerfile plus untracked scripts/wt-image/. Required commits: none, required ancestry checks not applicable. Checkpoint inspect matched context, with no restore required. No conflicting saved context was found.

Read /tmp/bd-a5xi.3-review-1/review-report.md and destination-proof.json. The sole blocking finding was contract-break on bd-a5xi.3: bare --registry and --push-registry authorities normalize to Docker Hub. No external blocker or new issue was needed.

## Minimal change and test-first proof

Changed this round:

- scripts/wt-image/main.go: validate the authority before the first slash. Require localhost, a dotted hostname or an explicit port, in addition to the existing syntax check. Apply the same validator to both registry flags before Git access, source capture or builder invocation.
- scripts/wt-image/main_test.go: two rejection regressions, each covering bare localregistry and localregistry/prefix.with.dot. Add accepted-authority cases for both flags. Extend the existing command fixture to pass flag overrides and report early validation errors.
- scripts/wt-image/README.md: document explicit-authority syntax, examples and rejection timing.

scripts/preview/Dockerfile remains unchanged from round 1. Source capture, exclusions, mode normalization, hash algorithm, image tags and baked metadata semantics remain unchanged. This round adds no dependencies.

RED command, before the implementation fix:

```sh
timeout 60s go test ./scripts/wt-image -run '^(TestRegistryRejectsAmbiguousAuthority|TestPushRegistryRejectsAmbiguousAuthority)$' -count=1 -timeout=30s -json
```

Exit 1. Two top-level tests failed. All four argument cases reached the stubbed Docker build/push route. Including parent test results, six failure events were reported. No external push occurred. Evidence: red.jsonl, red.exit and red-summary.json.

GREEN command:

```sh
timeout 60s go test ./scripts/wt-image -count=1 -timeout=30s -json
```

Exit 0. 16 top-level tests passed, 0 failed, 0 skipped. Including eight subtests: 24 passes. Rejected arguments fail with the explicit-authority diagnostic before an evidence directory or builder command is created. The tests guard the entire silent-routing defect class for both flags and for dotted path prefixes.

## Completed checks and exact commands

All commands ran in /home/omnigent/workspace/b9s unless an absolute path or -C specifies otherwise. All final commands exited 0.

| Exact command | Measured result | Evidence |
| --- | --- | --- |
| `timeout 180s go build ./...` | One build command passed | go-build.exit, go-build-summary.txt |
| `timeout 180s go vet ./...` | One vet command passed | go-vet.exit, go-vet-summary.txt |
| `GOFLAGS=-v timeout 660s make test-stable` | 2140 top-level passed, 0 failed, 18 optional skipped | test-stable.log, test-stable.exit, stable-summary.json |
| `timeout 180s bash /home/omnigent/workspace/b9s/tests/preview_contract_test.sh` | 219 passed, 0 failed, 0 skipped | preview-contract.log, preview-contract.exit |
| `timeout 60s go build -o /tmp/bd-a5xi.3-revision-2/wt-image ./scripts/wt-image` | Standalone proof helper built | helper-build.log, helper-build.exit |
| `timeout 75s python3 /tmp/bd-a5xi.3-revision-2/check-destination.py` | 9 passed, 0 failed, 0 skipped | destination-proof.json, destination-proof.exit |
| `timeout 45s python3 /tmp/bd-a5xi.3-revision-2/check-source.py` | 10 passed, 0 failed, 0 skipped | source-proof.json, source-proof.exit |
| `git diff --check` | Exit 0 | Final worktree checked |
| `gofmt -l /home/omnigent/workspace/b9s/scripts/wt-image/main.go /home/omnigent/workspace/b9s/scripts/wt-image/main_test.go` | Exit 0, no paths listed | Final files checked |

Canonical parser command:

```sh
timeout 60s go run -C /tmp/bd-a5xi.3-review-1/reference -mod=mod . localregistry/b9s-worktree:example localregistry:5000/b9s-worktree:example localhost/b9s-worktree:example registry.internal/b9s-worktree:example localhost/prefix.with.dot/b9s-worktree:example
```

Exit 0. The same distribution/reference v0.6.0 parser used by the reviewer maps the bare authority to docker.io/localregistry and preserves all four approved explicit destinations. The destination proof asserts these five canonical results plus four early CLI rejections, for nine passing checks total. Docker was stubbed. Neither source capture nor a builder ran for rejected arguments.

Stable totals including nested results: 3783 passed, 0 failed, 20 skipped. Do not add these to top-level counts. The required stable suite ran alone, without baseline worktrees or comparison. Unchanged packages used Go's test cache where permitted by the Makefile. The changed helper suite executed fresh. Existing Makefile exclusions remain intact. Eighteen optional top-level skips require real bd builds, S3, Dolt, isolated Memory/research setup or opt-in stress runs. Two additional optional subtests require absent srps/cass fixture files. Exact skipped names/reasons are in stable-summary.json. No retries ran. Flakiness was not separately assessed.

Committed dependencies were installed during round 1 with go mod download. go.mod, go.sum and vendor are unchanged. Go build and vet provide the affected language's compiler/static checks. No web or UI code changed.

## Rebuilt image and source identity

The old source/image evidence was not reused as evidence for this changed tree. `lane-checkpoint compare` with round 1 evidence exited 2 and refused stale/skipped evidence as expected. New checks and a real image build followed.

Exact command:

```sh
timeout 620s go run ./scripts/wt-image --repo /home/omnigent/workspace/b9s --builder buildkit --delivery registry --buildkit-addr tcp://buildkitd.buildkit.svc.cluster.local:1234 --registry registry.registry.svc.cluster.local:5000 --insecure-registry --output /tmp/bd-a5xi.3-image-2
```

Exit 0. The revised dirty worktree image built and pushed to the local registry. Baked metadata matched the frozen context.

- Commit: c2fccef8b537ed034ebd360ea7cf31c02250e083
- Dirty: true
- Tree SHA-256: b5abad156f52ae9ba3435b7886c7c9d4b07407cf9b996f59ad50f0adbf199607
- Files: 6042
- Image: registry.registry.svc.cluster.local:5000/b9s-worktree:wt-c2fccef8b537-true-b5abad156f52ae9ba3435b7886c7c9d4b07407cf9b996f59ad50f0adbf199607
- Image digest: sha256:d19eddd9ccfb4ac81c7adb7e09d2f10441a4b2614d1b9b3dec5e6ae008f93988

The reviewer source-proof script was copied to this evidence directory, with only its image/evidence output paths changed. All ten assertions passed against the new image: selected path enumeration, all frozen/current bytes and normalized modes, recomputed tree hash, both baked identity files, live registry digest/labels, exported binary and all four cumulative task files. The exported binary command and output are in source-proof.json. It exited 0 with `b9s dev`.

## Review state, risks and handoff

The revision resolves the sole required failure, issue bd-a5xi.3, with RED/GREEN tests and direct CLI/canonical-destination proof. The previous failed implementation milestone is superseded by the evidence-backed revision 2 implementation milestone. The review stage has been moved to running for independent re-review. This implementer does not claim a reviewer approval or alter the original review evidence.

Required implementation/check blockers: none. New discovered issue IDs: none. Unrelated follow-up work: none filed. Existing epic siblings retain namespace deployment, sample data, bd support, shell access and running-pod comparison.

Residual limits remain: Docker/k3d delivery alternatives use stubs in this Pod. Real delivery used BuildKit and the local registry. Runtime checks used the exported image filesystem. No cluster namespace or pod was deployed, and no shared database was written.

Evidence: /tmp/bd-a5xi.3-revision-2/. New frozen input/runtime/image metadata: /tmp/bd-a5xi.3-image-2/. Initial revision checkpoint push succeeded to refs/heads/omnigent-checkpoints/bd-a5xi.3. Final checkpoint publication is recorded separately. All delivery actions still require Polly's explicit approval.
