#!/usr/bin/env bash
# The repository-owned preview interface is the deployer's complete contract.
set -uo pipefail

ROOT="$(git -C "$(dirname "$0")/.." rev-parse --show-toplevel)"
PREVIEW="$ROOT/scripts/preview"
pass=0
fail=0

ok() { pass=$((pass + 1)); printf 'PASS  %s\n' "$1"; }
bad() { fail=$((fail + 1)); printf 'FAIL  %s%s\n' "$1" "${2:+: $2}"; }
contains() {
  local desc="$1" needle="$2" haystack="$3"
  if grep -Fq -- "$needle" <<<"$haystack"; then ok "$desc"; else bad "$desc" "missing $needle"; fi
}

for command in build deploy status verify destroy; do
  file="$PREVIEW/preview-$command"
  if [[ -x $file ]]; then ok "preview-$command is executable"; else bad "preview-$command is executable" "$file"; fi
done

if [[ -f $PREVIEW/Dockerfile ]]; then ok "preview image has a Dockerfile"; else bad "preview image has a Dockerfile"; fi
if [[ -f $PREVIEW/lib.sh ]]; then ok "preview commands share a contract library"; else bad "preview commands share a contract library"; fi
if [[ -f $PREVIEW/mobile.html ]]; then ok "mobile preview has a touch shell"; else bad "mobile preview has a touch shell"; fi
if [[ -x $PREVIEW/mobile-key.cgi ]]; then ok "mobile preview has an executable key endpoint"; else bad "mobile preview has an executable key endpoint"; fi
if [[ -x $ROOT/tests/mobile_preview_e2e.sh ]]; then ok "mobile preview has an executable E2E check"; else bad "mobile preview has an executable E2E check"; fi

entrypoint="$(<"$PREVIEW/entrypoint.sh")"
contains "browser terminal uses the darker preview background" 'theme={"background":"#18181b"}' "$entrypoint"
contains "mobile and desktop terminals share one session" "tmux new-session -d -s b9s" "$entrypoint"
contains "mobile terminal has a stable base path" "--base-path /b9s/terminal" "$entrypoint"
contains "mobile key endpoint is installed in the CGI root" "/www/cgi-bin/b9s-key" "$entrypoint"

contract_library="$(<"$PREVIEW/lib.sh")"
contains "mobile terminal is allowed through the network policy" 'port: 7683' "$contract_library"
contains "mobile terminal is exposed by the service" 'name: mobile-terminal' "$contract_library"
contains "mobile shell has a dedicated ingress path" 'path: /b9s/terminal' "$contract_library"
contains "mobile key endpoint has a dedicated ingress path" 'path: /cgi-bin/b9s-key' "$contract_library"

if [[ -f $PREVIEW/mobile.html ]]; then
  mobile="$(<"$PREVIEW/mobile.html")"
  contains "mobile viewport is declared" 'name="viewport"' "$mobile"
  contains "mobile shell avoids a favicon request" 'rel="icon"' "$mobile"
  contains "mobile projects are directly selectable" 'data-key="0"' "$mobile"
  contains "mobile list can move upward" 'data-key="Up"' "$mobile"
  contains "mobile list can move downward" 'data-key="Down"' "$mobile"
  contains "mobile pages can move left" 'data-key="Left"' "$mobile"
  contains "mobile pages can move right" 'data-key="Right"' "$mobile"
  contains "mobile selection can open" 'data-key="Enter"' "$mobile"
  contains "mobile selection can go back" 'data-key="Escape"' "$mobile"
  contains "mobile controls call the executable CGI path" '/cgi-bin/b9s-key?key=' "$mobile"
fi

if [[ $fail -eq 0 ]]; then
  sha="$(git -C "$ROOT" rev-parse HEAD)"
  kubeconfig="$(mktemp)"
  trap 'rm -f "$kubeconfig"' EXIT
  rendered="$(
    PREVIEW_CMD=contract-test \
    B9S_PREVIEW_KUBECONFIG="$kubeconfig" \
    B9S_PREVIEW_CONTEXT=preview \
    B9S_PREVIEW_HOST_SUFFIX=previews.osenco.test \
    bash -c 'source "$1"; preview_parse_args bd-b6jw "$2"; preview_render_manifests /dev/stdout' \
      bash "$PREVIEW/lib.sh" "$sha"
  )"
  render_rc=$?
  if [[ $render_rc -eq 0 ]]; then ok "preview manifest renders without cluster writes"; else bad "preview manifest renders without cluster writes"; fi

  contains "namespace is project-prefixed" "name: b9s-bd-b6jw" "$rendered"
  contains "namespace is labelled for b9s" "omnigent.osenco.dev/project: b9s" "$rendered"
  contains "namespace records its task" "omnigent.osenco.dev/task-id: bd-b6jw" "$rendered"
  contains "deployer binds the shared workload role" "name: omnigent-preview-workloads" "$rendered"
  contains "binding names the b9s deployer" "name: omnigent-preview-deployer-b9s" "$rendered"
  contains "image is immutable by full commit" "b9s-preview:$sha" "$rendered"
  contains "browser terminal is exposed" "name: terminal" "$rendered"
  contains "identity endpoint is exposed" "name: identity" "$rendered"
  contains "preview has a clickable hostname" "host: b9s-bd-b6jw.previews.osenco.test" "$rendered"

  set +e
  invalid="$(B9S_PREVIEW_KUBECONFIG="$kubeconfig" B9S_PREVIEW_CONTEXT=preview \
    "$PREVIEW/preview-status" 'bd-b6jw;touch-pwned' "$sha" 2>&1)"
  invalid_rc=$?
  set -e
  if [[ $invalid_rc -eq 2 ]]; then ok "unsafe task identifiers stop before cluster access"; else bad "unsafe task identifiers stop before cluster access" "exit $invalid_rc"; fi
  contains "unsafe task identifier reports the boundary" "TASK_ID is not a plain identifier" "$invalid"
fi

printf '=== preview-contract DONE pass=%d fail=%d ===\n' "$pass" "$fail"
[[ $fail -eq 0 ]]
