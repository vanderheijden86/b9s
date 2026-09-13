#!/usr/bin/env bash
# Verify the deployed mobile shell through the same URL a phone uses.
set -uo pipefail

if [[ $# -ne 3 ]]; then
  printf 'usage: %s BASE_URL TASK_ID COMMIT_SHA\n' "$0" >&2
  exit 2
fi

base_url="${1%/}"
task_id="$2"
commit_sha="$3"
pass=0
fail=0

ok() { pass=$((pass + 1)); printf 'PASS  %s\n' "$1"; }
bad() { fail=$((fail + 1)); printf 'FAIL  %s%s\n' "$1" "${2:+: $2}" >&2; }
contains() {
  local description="$1" needle="$2" value="$3"
  if grep -Fq -- "$needle" <<<"$value"; then ok "$description"; else bad "$description" "missing $needle"; fi
}
fetch() { curl -fsS --connect-timeout 3 --max-time 10 "$@"; }

mobile="$(fetch "$base_url/b9s/" 2>/dev/null || true)"
contains "mobile shell loads" "B9s mobile preview" "$mobile"
contains "project selector is present" 'aria-label="Projects"' "$mobile"
contains "issue navigation is present" 'aria-label="Issue navigation"' "$mobile"

terminal="$(fetch "$base_url/b9s/terminal/" 2>/dev/null || true)"
contains "mobile ttyd loads" "<title>ttyd" "$terminal"

identity="$(fetch "$base_url/b9s/__preview" 2>/dev/null || true)"
contains "mobile preview serves the requested commit" "\"commit\":\"$commit_sha\"" "$identity"
contains "mobile preview serves the requested task" "\"taskId\":\"$task_id\"" "$identity"

down="$(fetch -d '' "$base_url/cgi-bin/b9s-key?key=Down" 2>/dev/null || true)"
contains "Down control reaches the shared session" '"ok":true,"key":"Down"' "$down"

up="$(fetch -d '' "$base_url/cgi-bin/b9s-key?key=Up" 2>/dev/null || true)"
contains "Up control reaches the shared session" '"ok":true,"key":"Up"' "$up"

invalid_status="$(curl -sS --connect-timeout 3 --max-time 10 -o /dev/null -w '%{http_code}' \
  -d '' "$base_url/cgi-bin/b9s-key?key=Delete" 2>/dev/null || true)"
if [[ $invalid_status == 400 ]]; then
  ok "unsupported keys are rejected"
else
  bad "unsupported keys are rejected" "HTTP ${invalid_status:-unreachable}"
fi

printf '=== mobile-preview-e2e DONE pass=%d fail=%d ===\n' "$pass" "$fail"
[[ $fail -eq 0 ]]
