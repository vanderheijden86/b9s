# Independent review: bd-bezu, round 1

Verdict: CHANGES_REQUESTED

## Branch and diff

Repository: /home/omnigent/workspace/b9s
Assigned integration base: origin/main. PR target: main.
Selected branch: omnigent/bd-bezu.
HEAD and origin/main both resolve to eb77cef9f8e68e9854f3c1b6ec6a6074b812830a.
The assigned base is an ancestor of HEAD. Required commits: none.
Read implementer Git preflight and full diff against origin/main, including all test additions.
No untracked files were present. Exact changed-file paths:

- /home/omnigent/workspace/b9s/docs/testing.md
- /home/omnigent/workspace/b9s/pkg/ui/detail_footer_keys_test.go
- /home/omnigent/workspace/b9s/pkg/ui/model.go

## Blocking findings

FINDING: logic-error | pkg/ui/model.go:5018 | Query-forced flat rows still advertise unsupported Tab and Shift-Tab folding hints (bd-bezu).

Priority P2. The footer suppresses folding hints using IsFlatMode(), which checks only the explicit saved layout preference. ADR 0027 also requires positive and negated type predicates to force flat rows. The renderer uses IsListView(), which includes these predicates. Starting from tree mode and applying type:task (also reached through F3) or !type:epic leaves IsFlatMode false and IsListView true. Both fold hints remain advertised, yet Tab and Shift-Tab leave the three visible rows unchanged.

Use the effective list-layout predicate when deciding whether to advertise folding. Extend the guard at pkg/ui/detail_footer_keys_test.go:290 to cover positive and negated type queries with the explicit flat preference off. The current matrix only enables flat rows through ToggleFlatMode(), so the passing function-key assertions never inspect the resulting footer.

Independent reproduction used a temporary Go overlay. No repository source or test file was edited:

`timeout 120s go test -overlay /tmp/bd-bezu/reviewer-overlay.json ./pkg/ui -run '^TestReviewerFooterForcedFlatTypeQuery$' -count=1 -timeout=60s -v`

Final result: exit 1, 0 passed leaf subtests, 2 failed leaf subtests, 0 skipped. Each failing subtest exercises both Tab and Shift-Tab. Log: /tmp/bd-bezu/reviewer-probe-final.txt. Overlay source: /tmp/bd-bezu/reviewer-footer-probe_test.go. The initial diagnostic used unsupported -type syntax for its negative case. The final diagnostic uses valid !type syntax and supersedes it.

## Non-blocking findings

None. Removing unsupported hints from fallback split/list focus, advertising the existing Ctrl-N creation action, and matching numeric labels to picker mode are within scope. These edits preserve handlers and align the hints with existing contracts. No new architectural decision is introduced. Reviewed ADRs 0027, 0030, 0034, 0035 and 0057, plus the shared-query contract.

## Verification

Independent focused command:
`timeout 120s go test ./pkg/ui -run Footer -count=1 -timeout=60s -json`
Exit 0. 491 leaf checks passed, 0 failed, 0 skipped. 17 top-level tests passed. Evidence: /tmp/bd-bezu/reviewer-footer-summary.json and reviewer-footer.jsonl.

Independent git diff --check: exit 0.

Read implementer completed evidence: stable suite 4026 passed leaf checks, 0 failed, 20 declared skips, 22 passing packages. Build and vet each exited 0. These broader checks were not rerun by the reviewer. The independent tester stage remains separate.

## Residual risks

The guard uses isolated Model fixtures rather than a real terminal, project switch, clipboard or database write. These boundaries fit this task. The stable suite's declared external-service, fixture and opt-in stress skips remain. No additional security, error-handling or handler regressions were found in the production diff.

## Unresolved required-check failures

- bd-bezu: reviewer diagnostic TestReviewerFooterForcedFlatTypeQuery/type:task fails because unsupported folding hints remain.
- bd-bezu: reviewer diagnostic TestReviewerFooterForcedFlatTypeQuery/!type:epic fails for the same cause.

These are two reproductions of one required blocker on the assigned task. No unrelated follow-up issues were filed. No other unresolved required-check failures were observed.

Reviewer did not implement, commit, push, merge or open a pull request. Repository changes remain the implementer's three files.
