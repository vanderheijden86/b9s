Task: bd-80q4

The project-switch persistence defect is already fixed on `main` by `3a5ed086abe347af005c06efc3fe3f2c7661af12`, merged through https://github.com/vanderheijden86/b9s/pull/59. Successful project switches clear the previous tree and select the destination persistence directory. Expansion, bookmarks and layout use that project's `.beads/tree-state.json`, while session sort remains independent of persisted project state.

The approved delivery branch is `omnigent/bd-80q4`, targeting `main`. Its head is `873e74381482d2d7f73bccbc739d2c559daea050`, identical to the tested base. This lane made no source or documentation changes and created no artificial commit. PR #59 is the existing upstream fix, not a PR for this delivery branch.

## Verification

Independent reviewer verdict: **APPROVED**. Independent tester verdict: **TESTS_PASSED**.

- `timeout --kill-after=10s 180s go build ./...`: exit 0, no diagnostics.
- `timeout --kill-after=10s 660s env GOFLAGS="-v -count=1" make test-stable`: exit 0, run alone on the delivery tree. 3,561 passed leaf cases, 0 failed, 20 optional skipped, 0 interrupted and 0 unrun. All 22 tested packages passed. Top-level counts: 2,125 passed, 0 failed, 18 skipped.
- Focused persistence/sort subset: 48 passed leaf cases, 0 failed, 0 skipped, 0 interrupted and 0 unrun. Selection: `TreePersistence|TreeState|Tree.*Sort|TreeBuild.*Expansion|WithConfig.*Sort|ConfigSortFieldNames`. The tester measured this subset within the stable suite. The reviewer also ran it independently.
- `timeout --kill-after=10s 180s go vet ./...`: exit 0, no diagnostics.
- Controlled negative control: omitting only the project switch's directory assignment through an isolated Go overlay caused all three persistence cases to fail. The delivery source remained unchanged.

## Preview

http://b9s-bd-80q4.previews.osenco.test:8081

Preview checks established the exact tested SHA `873e74381482d2d7f73bccbc739d2c559daea050`: 21 passed, 0 failed, 0 skipped, comprising build 4, deploy 5, status 4 and verify 8. The preview did not exercise interactive project switching. The 48 persistence/sort regression cases provide that behavior's automated coverage.

## Residual risks

The stable target retains its declared `tests/e2e` and host-sensitive `STABLE_SKIP` exclusions. Its 20 optional skips cover real bd probes, S3, an unset Dolt DSN, isolated Memory preview/research, absent external JSONL fixtures and opt-in stress tests. No required acceptance case was skipped. No race-detector, browser or full real Dolt suite ran. The preview does not establish interactive switching, and repeated-run flakiness was not assessed. No unresolved required-check failures remain.
