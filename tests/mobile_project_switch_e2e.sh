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
playwright_checked() {
  local output status
  output="$(playwright-cli "$@" 2>&1)"
  status=$?
  printf '%s\n' "$output"
  [[ $status -eq 0 ]] && ! grep -Fq '### Error' <<<"$output"
}
capture_pane() { "${kc[@]}" exec deploy/b9s -- tmux capture-pane -p -t b9s 2>/dev/null; }
close_browser() { playwright-cli -s="$session" close >/dev/null 2>&1 || true; }
trap close_browser EXIT

identity="$(curl -fsS --connect-timeout 3 --max-time 10 "$base_url/b9s/__preview" 2>/dev/null || true)"
contains "preview serves the requested commit" "\"commit\":\"$commit_sha\"" "$identity"
contains "preview serves the requested task" "\"taskId\":\"$task_id\"" "$identity"

if browser_output="$(playwright_checked -s="$session" open "$base_url/b9s/")"; then
  ok "phone UI opens in a headless browser"
else
  bad "phone UI opens in a headless browser" "$browser_output"
fi
if resize_output="$(playwright_checked -s="$session" resize 390 844)"; then
  ok "phone viewport is applied"
else
  bad "phone viewport is applied" "$resize_output"
fi

reconnect_code="async (page) => {
  let terminal = page.frames().find(frame => frame.url().includes('/b9s/terminal/'));
  if (!terminal) throw new Error('terminal frame is missing');
  await terminal.evaluate(() => {
    document.documentElement.dataset.reconnectProbe = 'stale';
  });
  await page.waitForTimeout(500);
  const markerBefore = await terminal.evaluate(
    () => document.documentElement.dataset.reconnectProbe || ''
  );
  if (markerBefore !== 'stale') throw new Error('terminal frame was not stable before reconnect');
  await page.getByRole('button', { name: 'Reconnect terminal', exact: true }).click();
  await page.waitForTimeout(2000);
  terminal = page.frames().find(frame => frame.url().includes('/b9s/terminal/'));
  if (!terminal) throw new Error('terminal frame did not reload');
  const marker = await terminal.evaluate(
    () => document.documentElement.dataset.reconnectProbe || ''
  );
  if (marker === 'stale') throw new Error('touch reconnect left the stale terminal frame intact');
}"
if reconnect_output="$(playwright_checked -s="$session" run-code "$reconnect_code")"; then
  ok "phone UI reloads the terminal through touch reconnect"
else
  bad "phone UI reloads the terminal through touch reconnect" "$reconnect_output"
fi

all_code="async (page) => {
  const responsePromise = page.waitForResponse(
    response => response.url().includes('/cgi-bin/b9s-key?key=0'),
    { timeout: 10000 }
  );
  await page.getByRole('button', { name: 'Show all projects', exact: true }).click();
  const response = await responsePromise;
  if (!response.ok()) throw new Error('navigation returned HTTP ' + response.status());
}"

all_pane="$(capture_pane)"
if ! grep -Fq "All projects:" <<<"$all_pane"; then
  for _ in {1..20}; do
    all_pane="$(capture_pane)"
    if grep -Fq "All projects:" <<<"$all_pane"; then
      break
    fi
    sleep 1
  done
fi
if ! grep -Fq "All projects:" <<<"$all_pane"; then
  playwright_checked -s="$session" run-code "$all_code" >/dev/null 2>&1 || true
  for _ in {1..45}; do
    all_pane="$(capture_pane)"
    if grep -Fq "All projects:" <<<"$all_pane"; then
      break
    fi
    sleep 1
  done
fi
if ! grep -Fq "All projects:" <<<"$all_pane"; then
  playwright_checked -s="$session" run-code "$all_code" >/dev/null 2>&1 || true
  for _ in {1..45}; do
    all_pane="$(capture_pane)"
    if grep -Fq "All projects:" <<<"$all_pane"; then
      break
    fi
    sleep 1
  done
fi
contains "shared terminal starts in All projects" "All projects:" "$all_pane"

tested_projects="|"
for iteration in 1 2; do
  all_pane="$(capture_pane)"
  target="$(awk -v tested="$tested_projects" '$1 ~ /^<[1-9]>$/ && $2 !~ /\.\.\.$/ && (($3 + 0) + ($4 + 0) + ($5 + 0)) > 0 && index(tested, "|" $2 "|") == 0 { print $1 "\t" $2; exit }' <<<"$all_pane")"
  if [[ -z $target ]]; then
    bad "project switch case $iteration has a populated visible project"
    continue
  fi
  ok "project switch case $iteration has a populated visible project"
  key="${target%%$'\t'*}"
  key="${key#<}"
  key="${key%>}"
  project="${target#*$'\t'}"
  tested_projects="$tested_projects$project|"

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
  if click_output="$(playwright_checked -s="$session" run-code "$click_code")"; then
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

  if all_output="$(playwright_checked -s="$session" run-code "$all_code")"; then
    ok "$project returns to All through the phone UI"
  else
    bad "$project returns to All through the phone UI" "$all_output"
  fi
  restored_pane=""
  for _ in {1..45}; do
    restored_pane="$(capture_pane)"
    if grep -Fq "All projects:" <<<"$restored_pane"; then
      break
    fi
    sleep 1
  done
  contains "$project returns to the shared All-project view" "All projects:" "$restored_pane"

done

restored_pane="$(capture_pane)"
contains "shared terminal returns to All projects" "All projects:" "$restored_pane"

printf '=== mobile-project-switch-e2e DONE pass=%d fail=%d ===\n' "$pass" "$fail"
[[ $fail -eq 0 ]]
