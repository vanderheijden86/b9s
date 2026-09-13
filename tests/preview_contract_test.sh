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
if [[ -x $PREVIEW/preview-provision-reader ]]; then ok "preview has an executable read-only credential provisioner"; else bad "preview has an executable read-only credential provisioner"; fi

entrypoint="$(<"$PREVIEW/entrypoint.sh")"
contains "browser terminal uses the darker preview background" 'theme={"background":"#18181b"}' "$entrypoint"
contains "mobile and desktop terminals share one session" "tmux new-session -d -s b9s" "$entrypoint"
contains "mobile terminal has a stable base path" "--base-path /b9s/terminal" "$entrypoint"
contains "mobile key endpoint is installed in the CGI root" "/www/cgi-bin/b9s-key" "$entrypoint"
contains "preview requires a read-only Dolt user" 'B9S_DOLT_READ_USER:?required' "$entrypoint"
contains "preview requires a read-only Dolt password" 'B9S_DOLT_READ_PASSWORD:?required' "$entrypoint"
contains "preview discovers only accessible databases" 'SHOW DATABASES' "$entrypoint"
contains "preview verifies the Beads issues table" 'SELECT 1 FROM' "$entrypoint"
contains "preview starts in the all-projects view" 'send-keys -t b9s 0' "$entrypoint"
if grep -Fq '"id":"demo"' <<<"$entrypoint"; then
  bad "preview does not seed demo tickets"
else
  ok "preview does not seed demo tickets"
fi

contract_library="$(<"$PREVIEW/lib.sh")"
contains "mobile terminal is allowed through the network policy" 'port: 7683' "$contract_library"
contains "mobile terminal is exposed by the service" 'name: mobile-terminal' "$contract_library"
contains "mobile shell has a dedicated ingress path" 'path: /b9s/terminal' "$contract_library"
contains "mobile key endpoint has a dedicated ingress path" 'path: /cgi-bin/b9s-key' "$contract_library"
contains "database traffic is limited to the MySQL port" 'port: 3306' "$contract_library"
contains "deployment reads the dedicated Dolt secret" 'secretRef:' "$contract_library"
contains "deployment names the dedicated Dolt secret" 'name: b9s-shared-dolt-reader' "$contract_library"

if [[ -f $PREVIEW/preview-provision-reader ]]; then
  provisioner="$(<"$PREVIEW/preview-provision-reader")"
  contains "provisioner grants only SELECT" 'GRANT SELECT ON' "$provisioner"
  contains "provisioner enumerates Beads databases" "table_name = 'issues'" "$provisioner"
  contains "provisioner proves writes are refused" 'UPDATE issues SET title = title WHERE 1 = 0' "$provisioner"
  if grep -Eq 'GRANT (ALL|INSERT|UPDATE|DELETE|CREATE|DROP|ALTER)' <<<"$provisioner"; then
    bad "provisioner never grants a write privilege"
  else
    ok "provisioner never grants a write privilege"
  fi
fi

if [[ -f $PREVIEW/mobile.html ]]; then
  mobile="$(<"$PREVIEW/mobile.html")"
  contains "mobile viewport is declared" 'name="viewport"' "$mobile"
  contains "mobile shell avoids a favicon request" 'rel="icon"' "$mobile"
  contains "mobile projects are directly selectable" 'data-key="0"' "$mobile"
  contains "mobile project slots scroll between pinned paging buttons" 'class="project-slots"' "$mobile"
  contains "mobile projects can scroll backward" 'data-key="["' "$mobile"
  contains "mobile projects can scroll forward" 'data-key="]"' "$mobile"
  contains "mobile list can move upward" 'data-key="Up"' "$mobile"
  contains "mobile list can move downward" 'data-key="Down"' "$mobile"
  contains "mobile pages can move left" 'data-key="Left"' "$mobile"
  contains "mobile pages can move right" 'data-key="Right"' "$mobile"
  contains "mobile selection can open" 'data-key="Enter"' "$mobile"
  contains "mobile selection can go back" 'data-key="Escape"' "$mobile"
  contains "mobile terminal has a reconnect control" 'aria-label="Reconnect terminal"' "$mobile"
  contains "mobile terminal reconnects when the network returns" "window.addEventListener('online', reconnectTerminal)" "$mobile"
  contains "mobile controls call the executable CGI path" '/cgi-bin/b9s-key?key=' "$mobile"
fi

if [[ $fail -eq 0 ]]; then
  sha="$(git -C "$ROOT" rev-parse HEAD)"
  kubeconfig="$ROOT/go.mod"
  rendered="$(
    PREVIEW_CMD=contract-test \
    B9S_PREVIEW_KUBECONFIG="$kubeconfig" \
    B9S_PREVIEW_CONTEXT=preview \
    B9S_PREVIEW_HOST_SUFFIX=previews.osenco.test \
    B9S_PREVIEW_TAILSCALE_HOST=macbook-pro-2.tailb7c04d.ts.net \
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
  contains "mobile preview has a tailnet-only hostname" "host: macbook-pro-2.tailb7c04d.ts.net" "$rendered"

  set +e
  invalid="$(B9S_PREVIEW_KUBECONFIG="$kubeconfig" B9S_PREVIEW_CONTEXT=preview \
    "$PREVIEW/preview-status" 'bd-b6jw;touch-pwned' "$sha" 2>&1)"
  invalid_rc=$?
  set -e
  if [[ $invalid_rc -eq 2 ]]; then ok "unsafe task identifiers stop before cluster access"; else bad "unsafe task identifiers stop before cluster access" "exit $invalid_rc"; fi
  contains "unsafe task identifier reports the boundary" "TASK_ID is not a plain identifier" "$invalid"

  set +e
  invalid_tailnet="$(B9S_PREVIEW_KUBECONFIG="$kubeconfig" B9S_PREVIEW_CONTEXT=preview \
    B9S_PREVIEW_TAILSCALE_HOST='bad host' \
    "$PREVIEW/preview-status" bd-b6jw "$sha" 2>&1)"
  invalid_tailnet_rc=$?
  set -e
  if [[ $invalid_tailnet_rc -eq 2 ]]; then ok "unsafe tailnet host stops before cluster access"; else bad "unsafe tailnet host stops before cluster access" "exit $invalid_tailnet_rc"; fi
  contains "unsafe tailnet host reports the boundary" "tailnet host is not valid" "$invalid_tailnet"
fi

# Model kubectl's partial apply: a denied Namespace does not stop it applying
# later documents. A server-side preflight must reject that deployment first.
redeploy_dir="$(mktemp -d "${TMPDIR:-/tmp}/b9s-redeploy-contract.XXXXXX")"
run_deploy() (
  export TEST_SHA="$1" TEST_DENY_NAMESPACE="$2" TEST_STATE_DIR="$3"
  export B9S_PREVIEW_KUBECONFIG="$ROOT/go.mod" B9S_PREVIEW_CONTEXT=preview
  export B9S_PREVIEW_EVIDENCE_DIR="$TEST_STATE_DIR/evidence"
  mkdir -p "$TEST_STATE_DIR"

  git() {
    case "$*" in
      *'rev-parse HEAD') printf '%s\n' "$TEST_SHA" ;;
      *'cat-file -e '*|*'status --porcelain '*) return 0 ;;
      *) command git "$@" ;;
    esac
  }
  kubectl() {
    [[ $1 == --kubeconfig && $2 == "$B9S_PREVIEW_KUBECONFIG" &&
       $3 == --context && $4 == preview ]] || return 90
    shift 4
    case "$1" in
      version) return 0 ;;
      --namespace)
        [[ $2 == b9s-bd-b6jw-2 && $3 == rollout ]] || return 91
        return 0 ;;
      apply)
        local file='' dry_run=false
        shift
        while [[ $# -gt 0 ]]; do
          case "$1" in
            --filename) file="$2"; shift 2 ;;
            --dry-run=server) dry_run=true; shift ;;
            *) return 92 ;;
          esac
        done
        [[ -f $file ]] || return 93
        grep -q '^kind: Namespace$' "$file" || return 94
        if $dry_run; then
          [[ $(grep -c '^kind:' "$file") == 1 ]] || return 95
          printf 'preflight\n' >>"$TEST_STATE_DIR/calls"
        else
          printf 'apply\n' >>"$TEST_STATE_DIR/calls"
          if grep -q '^kind: Deployment$' "$file"; then
            printf '%s\n' "$TEST_SHA" >>"$TEST_STATE_DIR/workloads"
          fi
        fi
        if [[ $TEST_DENY_NAMESPACE == 1 ]]; then
          printf 'Forbidden: namespace metadata update denied by preview policy\n' >&2
          return 1
        fi
        if ! $dry_run; then
          awk '/^---$/{exit} {print}' "$file" >"$TEST_STATE_DIR/namespace.yaml"
        fi ;;
      *) return 96 ;;
    esac
  }
  export -f git kubectl
  timeout 10 bash "$PREVIEW/preview-deploy" bd-b6jw.2 "$TEST_SHA"
)

first_sha=1111111111111111111111111111111111111111
second_sha=2222222222222222222222222222222222222222
if run_deploy "$first_sha" 0 "$redeploy_dir/allowed" >"$redeploy_dir/first.log" 2>&1; then
  ok "restricted first deployment succeeds"
else
  bad "restricted first deployment succeeds" "see $redeploy_dir/first.log"
fi
if run_deploy "$second_sha" 0 "$redeploy_dir/allowed" >"$redeploy_dir/second.log" 2>&1; then
  ok "restricted same-task deployment accepts a new SHA"
else
  bad "restricted same-task deployment accepts a new SHA" "see $redeploy_dir/second.log"
fi
contains "redeploy refreshes namespace commit label" "omnigent.osenco.dev/commit-sha: $second_sha" \
  "$(cat "$redeploy_dir/allowed/namespace.yaml" 2>/dev/null)"
if [[ $(cat "$redeploy_dir/allowed/calls" 2>/dev/null) == $'preflight\napply\npreflight\napply' ]]; then
  ok "each deployment preflights namespace policy before workload apply"
else
  bad "each deployment preflights namespace policy before workload apply"
fi
if [[ $(cat "$redeploy_dir/allowed/workloads" 2>/dev/null) == "$first_sha"$'\n'"$second_sha" ]]; then
  ok "redeploy applies the new workload SHA"
else
  bad "redeploy applies the new workload SHA"
fi

if run_deploy "$second_sha" 1 "$redeploy_dir/denied" >"$redeploy_dir/denied.log" 2>&1; then
  bad "namespace policy denial fails deployment"
else
  denied_rc=$?
  if [[ $denied_rc -eq 1 ]]; then
    ok "namespace policy denial fails deployment"
  else
    bad "namespace policy denial fails deployment" "expected exit 1, got $denied_rc"
  fi
fi
if [[ ! -f $redeploy_dir/denied/workloads ]]; then
  ok "namespace policy denial prevents partial workload deployment"
else
  bad "namespace policy denial prevents partial workload deployment"
fi
contains "namespace policy denial identifies the infrastructure prerequisite" 'osenco-infra' \
  "$(cat "$redeploy_dir/denied.log")"

printf '=== preview-contract DONE pass=%d fail=%d ===\n' "$pass" "$fail"
[[ $fail -eq 0 ]]
