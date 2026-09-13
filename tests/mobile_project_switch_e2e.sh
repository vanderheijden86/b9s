#!/usr/bin/env bash
# Exercise project switches through the phone UI and prove each selected Dolt
# database supplies its real issue count to the shared terminal session.
set -uo pipefail

if [[ $# -ne 5 ]]; then
  printf 'usage: %s BASE_URL TASK_ID COMMIT_SHA NAMESPACE KUBECONFIG\n' "$0" >&2
  exit 2
fi

base_url="${1%/}"
task_id="$2"
commit_sha="$3"
namespace="$4"
kubeconfig="$5"
session="b9s-project-switch-$$"
pass=0
fail=0

export BW_NO_BROWSER=1
export BW_TEST_MODE=1

kc=(kubectl --kubeconfig "$kubeconfig" -n "$namespace")

ok() { pass=$((pass + 1)); printf 'PASS  %s\n' "$1"; }
bad() { fail=$((fail + 1)); printf 'FAIL  %s%s\n' "$1" "${2:+: $2}" >&2; }
contains() {
  local description="$1" needle="$2" value="$3"
  if grep -Fq -- "$needle" <<<"$value"; then ok "$description"; else bad "$description" "missing $needle"; fi
}
capture_pane() { "${kc[@]}" exec deploy/b9s -- tmux capture-pane -p -t b9s 2>/dev/null; }
close_browser() { playwright-cli -s="$session" close >/dev/null 2>&1 || true; }
trap close_browser EXIT

identity="$(curl -fsS --connect-timeout 3 --max-time 10 "$base_url/b9s/__preview" 2>/dev/null || true)"
contains "preview serves the requested commit" "\"commit\":\"$commit_sha\"" "$identity"
contains "preview serves the requested task" "\"taskId\":\"$task_id\"" "$identity"

if browser_output="$(playwright-cli -s="$session" open "$base_url/b9s/" 2>&1)"; then
  ok "phone UI opens in a headless browser"
else
  bad "phone UI opens in a headless browser" "$browser_output"
fi
if resize_output="$(playwright-cli -s="$session" resize 390 844 2>&1)"; then
  ok "phone viewport is applied"
else
  bad "phone viewport is applied" "$resize_output"
fi

all_pane="$(capture_pane)"
if ! grep -Fq "All projects:" <<<"$all_pane"; then
  reset_code="async (page) => {
    const responsePromise = page.waitForResponse(
      response => response.url().includes('/cgi-bin/b9s-key?key=0'),
      { timeout: 10000 }
    );
    await page.getByRole('button', { name: 'Show all projects', exact: true }).click();
    const response = await responsePromise;
    if (!response.ok()) throw new Error('navigation returned HTTP ' + response.status());
  }"
  playwright-cli -s="$session" run-code "$reset_code" >/dev/null 2>&1 || true
  for _ in {1..20}; do
    all_pane="$(capture_pane)"
    if grep -Fq "All projects:" <<<"$all_pane"; then
      break
    fi
    sleep 1
  done
fi
contains "shared terminal starts in All projects" "All projects:" "$all_pane"

reset_pager_code="async (page) => {
  const button = page.getByRole('button', { name: 'Previous projects', exact: true });
  for (let i = 0; i < 50; i += 1) {
    const responsePromise = page.waitForResponse(
      response => new URL(response.url()).searchParams.get('key') === '[',
      { timeout: 10000 }
    );
    await button.click();
    const response = await responsePromise;
    if (!response.ok()) throw new Error('navigation returned HTTP ' + response.status());
  }
}"
if pager_output="$(playwright-cli -s="$session" run-code "$reset_pager_code" 2>&1)"; then
  ok "project picker is reset to its first page through the phone UI"
else
  bad "project picker is reset to its first page through the phone UI" "$pager_output"
fi
all_pane="$(capture_pane)"

targets="$(
  awk '$1 ~ /^<[1-9]>$/ && $2 !~ /\.\.\.$/ && (($3 + 0) + ($4 + 0) + ($5 + 0)) > 0 { print $1 "\t" $2 }' <<<"$all_pane" |
    head -2
)"
target_count="$(grep -c . <<<"$targets")"
if [[ $target_count -ge 2 ]]; then
  ok "two populated projects are available for switching"
else
  bad "two populated projects are available for switching" "found $target_count"
fi

while IFS= read -r target; do
  [[ -n $target ]] || continue
  key="${target%%$'\t'*}"
  key="${key#<}"
  key="${key%>}"
  project="${target#*$'\t'}"

  expected_count="$("${kc[@]}" exec deploy/b9s -- sh -c '
    MYSQL_PWD="$B9S_DOLT_READ_PASSWORD" mariadb \
      --protocol=TCP \
      --host="$B9S_DOLT_HOST" \
      --port="$B9S_DOLT_PORT" \
      --user="$B9S_DOLT_READ_USER" \
      --database="$1" \
      --batch --skip-column-names --raw \
      --execute="SELECT COUNT(*) FROM issues WHERE status != \"tombstone\""
  ' sh "$project" 2>/dev/null || true)"
  if [[ $expected_count =~ ^[1-9][0-9]*$ ]]; then
    ok "$project has a positive shared Dolt issue count"
  else
    bad "$project has a positive shared Dolt issue count" "got ${expected_count:-nothing}"
    continue
  fi

  click_code="async (page) => {
    const responsePromise = page.waitForResponse(
      response => response.url().includes('/cgi-bin/b9s-key?key=$key'),
      { timeout: 10000 }
    );
    await page.getByRole('button', { name: 'Open project $key', exact: true }).click();
    const response = await responsePromise;
    if (!response.ok()) throw new Error('navigation returned HTTP ' + response.status());
    return { status: response.status() };
  }"
  if click_output="$(playwright-cli -s="$session" run-code "$click_code" 2>&1)"; then
    ok "project $project is selected through its real phone button"
  else
    bad "project $project is selected through its real phone button" "$click_output"
    continue
  fi

  switched_pane=""
  for _ in {1..20}; do
    switched_pane="$(capture_pane)"
    if grep -Fq "Reloaded $expected_count issues" <<<"$switched_pane"; then
      break
    fi
    if grep -Eq 'No beads found|Reload error' <<<"$switched_pane"; then
      break
    fi
    sleep 1
  done

  if grep -Fq "Reloaded $expected_count issues" <<<"$switched_pane"; then
    ok "$project renders all $expected_count shared Dolt issues"
  else
    problem="$(grep -E 'No beads found|Reload error|Switched to|Reloaded [0-9]+ issues' <<<"$switched_pane" | tail -1)"
    bad "$project renders all $expected_count shared Dolt issues" "${problem:-no terminal result}"
  fi
done <<<"$targets"

all_code="async (page) => {
  const responsePromise = page.waitForResponse(
    response => response.url().includes('/cgi-bin/b9s-key?key=0'),
    { timeout: 10000 }
  );
  await page.getByRole('button', { name: 'Show all projects', exact: true }).click();
  const response = await responsePromise;
  if (!response.ok()) throw new Error('navigation returned HTTP ' + response.status());
}"
if all_output="$(playwright-cli -s="$session" run-code "$all_code" 2>&1)"; then
  ok "All projects is restored through the phone UI"
else
  bad "All projects is restored through the phone UI" "$all_output"
fi

restored_pane=""
for _ in {1..20}; do
  restored_pane="$(capture_pane)"
  if grep -Fq "All projects:" <<<"$restored_pane"; then
    break
  fi
  sleep 1
done
contains "shared terminal returns to All projects" "All projects:" "$restored_pane"

printf '=== mobile-project-switch-e2e DONE pass=%d fail=%d ===\n' "$pass" "$fail"
[[ $fail -eq 0 ]]
