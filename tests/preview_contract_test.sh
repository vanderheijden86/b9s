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

test_artifact_boundary() {
  local fixtures seed fixture sha expected output rc scenario
  fixtures="$(mktemp -d)" || return 1
  seed="$fixtures/seed"
  mkdir -p "$seed/.beads"
  printf 'version\n' >"$seed/.beads/.local_version"
  printf 'config\n' >"$seed/.beads/config.yaml"
  printf 'source\n' >"$seed/source.go"
  git -C "$seed" init -q || return 1
  git -C "$seed" add . || return 1
  git -C "$seed" -c user.name=PreviewTest -c user.email=preview@example.invalid \
    -c core.hooksPath=/dev/null -c commit.gpgsign=false commit -qm fixture || return 1
  sha="$(git -C "$seed" rev-parse HEAD)" || return 1

  for scenario in clean version lock combined codex source staged-source untracked-source \
    unrelated-metadata version-lookalike lock-lookalike staged-version staged-lock absent-commit; do
    fixture="$fixtures/$scenario"
    git clone -q --no-hardlinks "$seed" "$fixture" || return 1
    expected=2
    case "$scenario" in
      clean) expected=0 ;;
      version) printf 'runtime\n' >>"$fixture/.beads/.local_version"; expected=0 ;;
      lock) printf 'runtime\n' >"$fixture/.beads.gate.lock"; expected=0 ;;
      combined)
        printf 'runtime\n' >>"$fixture/.beads/.local_version"
        printf 'runtime\n' >"$fixture/.beads.gate.lock"
        expected=0 ;;
      codex) mkdir "$fixture/.codex-tmp"; touch "$fixture/.codex-tmp/session"; expected=0 ;;
      source) printf 'edit\n' >>"$fixture/source.go" ;;
      staged-source) printf 'edit\n' >>"$fixture/source.go"; git -C "$fixture" add source.go || return 1 ;;
      untracked-source) touch "$fixture/new.go" ;;
      unrelated-metadata) printf 'edit\n' >>"$fixture/.beads/config.yaml" ;;
      version-lookalike) touch "$fixture/.beads/.local_version.bak" ;;
      lock-lookalike) touch "$fixture/.beads.gate.lock.bak" ;;
      staged-version)
        printf 'runtime\n' >>"$fixture/.beads/.local_version"
        git -C "$fixture" add .beads/.local_version || return 1 ;;
      staged-lock) touch "$fixture/.beads.gate.lock"; git -C "$fixture" add .beads.gate.lock || return 1 ;;
      absent-commit) sha="$(git -C "$fixture" hash-object source.go)" ;;
    esac
    if output="$(PREVIEW_CMD=contract-test timeout 5s bash -c \
      'source "$1"; PREVIEW_ROOT="$2"; PREVIEW_COMMIT_SHA="$3"; preview_require_boundary' \
      bash "$PREVIEW/lib.sh" "$fixture" "$sha" 2>&1)"; then rc=0; else rc=$?; fi
    if [[ $rc -eq $expected ]]; then
      ok "artifact boundary: $scenario"
    else
      bad "artifact boundary: $scenario" "exit $rc, expected $expected: $output"
    fi
  done
  printf 'Boundary fixtures retained at %s\n' "$fixtures"
}

test_artifact_boundary || bad "artifact boundary fixture setup"

if grep -Fxq '.beads.gate.lock' "$PREVIEW/Dockerfile.dockerignore"; then
  ok "Beads gate lock is excluded from the build context"
else
  bad "Beads gate lock is excluded from the build context"
fi

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
  kubeconfig="$(mktemp)"
  trap 'rm -f "$kubeconfig"' EXIT
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

printf '=== preview-contract DONE pass=%d fail=%d ===\n' "$pass" "$fail"
[[ $fail -eq 0 ]]
