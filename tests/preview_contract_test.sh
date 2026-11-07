#!/usr/bin/env bash
# The repository-owned preview interface is the deployer's complete contract.
set -uo pipefail

ROOT="$(git -C "$(dirname "$0")/.." rev-parse --show-toplevel)"
PREVIEW="$ROOT/scripts/preview"
pass=0
fail=0
timeout_bin="$(command -v timeout || command -v gtimeout)" || {
  printf 'preview-contract requires GNU timeout (timeout or gtimeout)\n' >&2
  exit 1
}

ok() { pass=$((pass + 1)); printf 'PASS  %s\n' "$1"; }
bad() { fail=$((fail + 1)); printf 'FAIL  %s%s\n' "$1" "${2:+: $2}"; }
contains() {
  local desc="$1" needle="$2" haystack="$3"
  if grep -Fq -- "$needle" <<<"$haystack"; then ok "$desc"; else bad "$desc" "missing $needle"; fi
}

# The cluster supplies these (docs/preview-contract.md). Placeholder values
# keep the test independent of any real deployment.
export PREVIEW_HOST_SUFFIX=previews.example.test
export PREVIEW_INGRESS_MIDDLEWARE=platform-preview-only@kubernetescrd
export PREVIEW_LABEL_DOMAIN=preview.example
export PREVIEW_RUNNER_NAMESPACE=runners
export PREVIEW_WORKLOAD_ROLE=preview-workloads
export PREVIEW_DEPLOYER_SA='platform/preview-deployer-{project}'
export PREVIEW_READER_REF='{project}-reader'
export PREVIEW_VIEWER_HEADER=X-Example-Viewer
export PREVIEW_VIEWER=viewer@example.invalid
# Empty disables in-lane reader publishing, so the host's default reader path
# never leaks into a case that does not name its own fixture.
export PREVIEW_READER_ENV_FILE=
unset B9S_DOLT_PORT

# Exercise the real Git boundary in disposable fixture repositories. These
# fixtures never invoke an image builder or use Kubernetes credentials.
boundary_case() {
  local scenario="$1" expected="$2" fixture sha output rc
  fixture="$(mktemp -d "${TMPDIR:-/tmp}/b9s-preview-boundary.XXXXXX")" \
    || { bad "artifact boundary $scenario fixture"; return; }
  git -C "$fixture" init -q &&
  mkdir -p "$fixture/.beads" &&
  printf '0.63.3\n' >"$fixture/.beads/.local_version" &&
  printf '{}\n' >"$fixture/.beads/issues.jsonl" &&
  printf 'backend: dolt\n' >"$fixture/.beads/config.yaml" &&
  printf 'package fixture\n' >"$fixture/source.go" &&
  git -C "$fixture" add . &&
  git -C "$fixture" -c user.name=fixture -c user.email=fixture@example.invalid \
    -c core.hooksPath=/dev/null commit -qm fixture \
    || { bad "artifact boundary $scenario fixture"; return; }
  sha="$(git -C "$fixture" rev-parse HEAD)" \
    || { bad "artifact boundary $scenario fixture HEAD"; return; }
  case "$scenario" in
    version|both|staged-version|mixed)
      printf '1.3.0-rc.1\n' >"$fixture/.beads/.local_version" ;;
  esac
  case "$scenario" in
    lock|both|staged-lock) touch "$fixture/.beads.gate.lock" ;;
    harness) mkdir -p "$fixture/.codex-tmp"; touch "$fixture/.codex-tmp/run.log" ;;
    source|mixed) printf '// edited\n' >>"$fixture/source.go" ;;
    beads-data) printf '{"id":"edited"}\n' >>"$fixture/.beads/issues.jsonl" ;;
    beads-config) printf 'changed: true\n' >>"$fixture/.beads/config.yaml" ;;
    lookalike) touch "$fixture/.beads.gate.lock.go" ;;
    wrong-head) sha=0000000000000000000000000000000000000000 ;;
    head-mismatch)
      git -C "$fixture" -c user.name=fixture -c user.email=fixture@example.invalid \
        -c core.hooksPath=/dev/null commit --allow-empty -qm second \
        || { bad "artifact boundary $scenario second commit"; return; } ;;
  esac
  case "$scenario" in
    staged-version) git -C "$fixture" add .beads/.local_version ;;
    staged-lock) git -C "$fixture" add .beads.gate.lock ;;
  esac
  output="$("$timeout_bin" 5s bash -c '
    PREVIEW_CMD=contract-test
    source "$1"
    PREVIEW_ROOT="$2"
    PREVIEW_COMMIT_SHA="$3"
    preview_require_boundary
  ' bash "$PREVIEW/lib.sh" "$fixture" "$sha" 2>&1)"
  rc=$?
  if [[ $rc -eq $expected ]]; then
    ok "artifact boundary $scenario exits $expected"
  else
    bad "artifact boundary $scenario exits $expected" "exit $rc: $output"
  fi
}

for scenario in clean version lock both harness; do
  boundary_case "$scenario" 0
done
for scenario in source beads-data beads-config staged-version staged-lock lookalike mixed wrong-head head-mismatch; do
  boundary_case "$scenario" 2
done

for excluded in .beads .beads.gate.lock .codex-tmp; do
  if grep -qxF "$excluded" "$PREVIEW/Dockerfile.dockerignore"; then
    ok "preview build context excludes $excluded"
  else
    bad "preview build context excludes $excluded"
  fi
done

for command in build deploy status verify destroy; do
  file="$PREVIEW/preview-$command"
  if [[ -x $file ]]; then ok "preview-$command is executable"; else bad "preview-$command is executable" "$file"; fi
done

if [[ -f $PREVIEW/Dockerfile ]]; then ok "preview image has a Dockerfile"; else bad "preview image has a Dockerfile"; fi
if [[ -f $PREVIEW/lib.sh ]]; then ok "preview commands share a contract library"; else bad "preview commands share a contract library"; fi
if [[ -x $PREVIEW/preview-provision-reader ]]; then ok "preview has an executable read-only credential provisioner"; else bad "preview has an executable read-only credential provisioner"; fi

entrypoint="$(<"$PREVIEW/entrypoint.sh")"
contains "terminal UI waits in a tmux session for kubectl exec" "tmux new-session -d -s b9s" "$entrypoint"
contains "tmux uses the exec wrapper" '/usr/local/bin/preview-tui' "$entrypoint"
contains "startup records the selected project" '> /tmp/b9s-preview-project' "$entrypoint"
contains "image installs the exec wrapper" 'COPY scripts/preview/preview-tui /usr/local/bin/preview-tui' "$(<"$PREVIEW/Dockerfile")"
contains "preview serves the web board" "exec /usr/local/bin/b9s web --listen 0.0.0.0:7681" "$entrypoint"
contains "preview web discovers readable databases at runtime under its project root" \
  '--projects-root /data/projects --discover-dolt' "${entrypoint##*exec /usr/local/bin/b9s web }"
contains "web board trusts the cluster-supplied viewer header" '--trust-header "$PREVIEW_VIEWER_HEADER" --owner "$PREVIEW_VIEWER"' "$entrypoint"
contains "preview requires a viewer header" 'PREVIEW_VIEWER_HEADER:?required' "$entrypoint"
contains "preview requires a viewer" 'PREVIEW_VIEWER:?required' "$entrypoint"
for retired in ttyd mobile.html mobile-key.cgi --no-token; do
  if grep -Fq -- "$retired" <<<"$entrypoint$(<"$PREVIEW/Dockerfile")"; then
    bad "preview image carries no $retired"
  else
    ok "preview image carries no $retired"
  fi
done
contains "preview requires a read-only Dolt user" 'B9S_DOLT_READ_USER:?required' "$entrypoint"
contains "preview requires a read-only Dolt password" 'B9S_DOLT_READ_PASSWORD:?required' "$entrypoint"
contains "preview explicitly trusts its provisioned Dolt endpoint" 'B9S_TRUSTED_DOLT_ENDPOINTS="$B9S_DOLT_HOST:$B9S_DOLT_PORT"' "$entrypoint"
contains "preview discovers only accessible databases" 'SHOW DATABASES' "$entrypoint"
contains "preview verifies the Beads issues table" 'SELECT 1 FROM' "$entrypoint"
contains "preview starts in the all-projects view" 'send-keys -t b9s 0' "$entrypoint"
if grep -Fq '"id":"demo"' <<<"$entrypoint"; then
  bad "preview does not seed demo tickets"
else
  ok "preview does not seed demo tickets"
fi

contract_library="$(<"$PREVIEW/lib.sh")"
if grep -Fq 'port: 7683' <<<"$contract_library"; then
  bad "network policy admits only the web and identity ports"
else
  ok "network policy admits only the web and identity ports"
fi
contains "deployment reads the dedicated Dolt secret" 'secretRef:' "$contract_library"
contains "deployment names the cluster-supplied reader secret" 'name: ${PREVIEW_READER_REF}' "$contract_library"

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

if [[ $fail -eq 0 ]]; then
  sha="$(git -C "$ROOT" rev-parse HEAD)"
  kubeconfig="$ROOT/go.mod"
  rendered="$(
    PREVIEW_CMD=contract-test \
    B9S_PREVIEW_KUBECONFIG="$kubeconfig" \
    B9S_PREVIEW_CONTEXT=preview \
    bash -c 'source "$1"; preview_parse_args bd-b6jw "$2"; preview_render_manifests /dev/stdout' \
      bash "$PREVIEW/lib.sh" "$sha"
  )"
  render_rc=$?
  if [[ $render_rc -eq 0 ]]; then ok "preview manifest renders without cluster writes"; else bad "preview manifest renders without cluster writes"; fi

  contains "namespace is project-prefixed" "name: b9s-bd-b6jw" "$rendered"
  contains "namespace is labelled for b9s" "preview.example/project: b9s" "$rendered"
  contains "namespace records its task" "preview.example/task-id: bd-b6jw" "$rendered"
  contains "deployer binds the shared workload role" "name: preview-workloads" "$rendered"
  contains "binding names the deployer namespace" "namespace: platform" "$rendered"
  contains "runner namespace may reach the preview" "kubernetes.io/metadata.name: runners" "$rendered"
  contains "deployment names the project reader secret" "name: b9s-reader" "$rendered"
  contains "binding names the b9s deployer" "name: preview-deployer-b9s" "$rendered"
  contains "image is immutable by full commit" "b9s-preview:$sha" "$rendered"
  contains "web board is exposed" "name: web" "$rendered"
  contains "deployment passes the viewer header name" 'name: PREVIEW_VIEWER_HEADER, value: "X-Example-Viewer"' "$rendered"
  contains "deployment passes the viewer" 'name: PREVIEW_VIEWER, value: "viewer@example.invalid"' "$rendered"
  if grep -Eq 'mobile-terminal|/b9s/terminal|cgi-bin' <<<"$rendered"; then
    bad "rendered preview has no browser terminal route"
  else
    ok "rendered preview has no browser terminal route"
  fi
  contains "identity endpoint is exposed" "name: identity" "$rendered"
  contains "preview has a clickable hostname" "host: b9s-bd-b6jw.previews.example.test" "$rendered"

  set +e
  invalid="$(B9S_PREVIEW_KUBECONFIG="$kubeconfig" B9S_PREVIEW_CONTEXT=preview \
    "$PREVIEW/preview-status" 'bd-b6jw;touch-pwned' "$sha" 2>&1)"
  invalid_rc=$?
  set -e
  if [[ $invalid_rc -eq 2 ]]; then ok "unsafe task identifiers stop before cluster access"; else bad "unsafe task identifiers stop before cluster access" "exit $invalid_rc"; fi
  contains "unsafe task identifier reports the boundary" "TASK_ID is not a plain identifier" "$invalid"

  for viewer_case in 'PREVIEW_VIEWER_HEADER=bad header:PREVIEW_VIEWER_HEADER is not an HTTP header name' \
    'PREVIEW_VIEWER=no-at-sign:PREVIEW_VIEWER is not an email address'; do
    assignment="${viewer_case%%:*}"
    expected="${viewer_case#*:}"
    set +e
    invalid_viewer="$(env B9S_PREVIEW_KUBECONFIG="$kubeconfig" B9S_PREVIEW_CONTEXT=preview \
      "$assignment" "$PREVIEW/preview-status" bd-b6jw "$sha" 2>&1)"
    invalid_viewer_rc=$?
    set -e
    if [[ $invalid_viewer_rc -eq 2 ]]; then ok "invalid ${assignment%%=*} stops before cluster access"; else bad "invalid ${assignment%%=*} stops before cluster access" "exit $invalid_viewer_rc"; fi
    contains "invalid ${assignment%%=*} reports the boundary" "$expected" "$invalid_viewer"
  done

  chained_rendered="$(
    B9S_PREVIEW_KUBECONFIG="$kubeconfig" B9S_PREVIEW_CONTEXT=preview \
    PREVIEW_INGRESS_MIDDLEWARE=platform-preview-only@kubernetescrd,platform-viewer@kubernetescrd \
    bash -c 'source "$1"; preview_parse_args bd-b6jw "$2"; preview_render_manifests /dev/stdout' \
      bash "$PREVIEW/lib.sh" "$sha"
  )"
  contains "preview Ingress chains several Middlewares in order" \
    "router.middlewares: platform-preview-only@kubernetescrd,platform-viewer@kubernetescrd" "$chained_rendered"

  contains "preview Ingress admits only port-forward traffic" \
    "traefik.ingress.kubernetes.io/router.middlewares: platform-preview-only@kubernetescrd" "$rendered"
  open_rendered="$(
    B9S_PREVIEW_KUBECONFIG="$kubeconfig" B9S_PREVIEW_CONTEXT=preview PREVIEW_INGRESS_MIDDLEWARE= \
    bash -c 'source "$1"; preview_parse_args bd-b6jw "$2"; preview_render_manifests /dev/stdout' \
      bash "$PREVIEW/lib.sh" "$sha"
  )"
  if [[ -n $open_rendered && $open_rendered != *router.middlewares* ]]; then
    ok "an empty middleware override renders no Middleware"
  else
    bad "an empty middleware override renders no Middleware"
  fi
  set +e
  invalid_middleware="$(B9S_PREVIEW_KUBECONFIG="$kubeconfig" B9S_PREVIEW_CONTEXT=preview \
    PREVIEW_INGRESS_MIDDLEWARE='x"; evil' \
    "$PREVIEW/preview-status" bd-b6jw "$sha" 2>&1)"
  invalid_middleware_rc=$?
  set -e
  if [[ $invalid_middleware_rc -eq 2 ]]; then ok "unsafe middleware stops before cluster access"; else bad "unsafe middleware stops before cluster access" "exit $invalid_middleware_rc"; fi
  contains "unsafe middleware reports the boundary" "ingress middleware is not a list of Traefik CRD references" "$invalid_middleware"

  render_with() {
    env "$@" B9S_PREVIEW_KUBECONFIG="$kubeconfig" B9S_PREVIEW_CONTEXT=preview \
      bash -c 'source "$1"; preview_parse_args bd-b6jw "$2"; preview_render_manifests /dev/stdout' \
        bash "$PREVIEW/lib.sh" "$sha" 2>&1
  }
  refused_with() {
    local desc="$1" expected="$2" output rc
    shift 2
    set +e
    output="$(env "$@" B9S_PREVIEW_KUBECONFIG="$kubeconfig" B9S_PREVIEW_CONTEXT=preview \
      "$timeout_bin" 20 "$PREVIEW/preview-status" bd-b6jw "$sha" 2>&1)"
    rc=$?
    set -e
    if [[ $rc -eq 2 && $output == *"$expected"* ]]; then ok "$desc"; else bad "$desc" "exit $rc: $output"; fi
  }
  # Only the NetworkPolicy has these two-space keys; the Ingress has none.
  policy_ingress() { awk '/^  ingress:$/{f=1; next} /^  egress:$/{f=0} f' <<<"$1"; }
  policy_egress() { awk '/^  egress:$/{f=1; next} /^---$/{f=0} f' <<<"$1"; }

  default_ingress="$(policy_ingress "$rendered")"
  contains "ingress namespace defaults to kube-system" "kubernetes.io/metadata.name: kube-system" "$default_ingress"
  traefik_rendered="$(render_with PREVIEW_INGRESS_NAMESPACE=traefik)"
  traefik_ingress="$(policy_ingress "$traefik_rendered")"
  contains "ingress namespace admits the named namespace" "kubernetes.io/metadata.name: traefik" "$traefik_ingress"
  contains "ingress namespace keeps the runner rule" "kubernetes.io/metadata.name: runners" "$traefik_ingress"
  if [[ $traefik_ingress == *kube-system* ]]; then
    bad "ingress namespace replaces kube-system"
  else
    ok "ingress namespace replaces kube-system"
  fi
  contains "ingress namespace leaves DNS egress on kube-system" "kubernetes.io/metadata.name: kube-system" \
    "$(policy_egress "$traefik_rendered")"
  refused_with "unsafe ingress namespace stops before cluster access" \
    "PREVIEW_INGRESS_NAMESPACE is not a Kubernetes name" PREVIEW_INGRESS_NAMESPACE=Bad_NS

  contains "database egress defaults to TCP 3306" "{protocol: TCP, port: 3306}" "$(policy_egress "$rendered")"
  port_egress="$(policy_egress "$(render_with B9S_DOLT_PORT=3307)")"
  contains "database egress follows B9S_DOLT_PORT" "{protocol: TCP, port: 3307}" "$port_egress"
  if [[ $port_egress == *3306* ]]; then bad "database egress opens only B9S_DOLT_PORT"; else ok "database egress opens only B9S_DOLT_PORT"; fi
  refused_with "invalid B9S_DOLT_PORT stops before cluster access" "B9S_DOLT_PORT is not a TCP port" B9S_DOLT_PORT=99999

  reader_dir="$(mktemp -d "${TMPDIR:-/tmp}/b9s-reader-contract.XXXXXX")"
  printf '%s\n' '# issued by the platform' 'B9S_DOLT_HOST=dolt-2.dolt-2.svc.cluster.local' \
    'B9S_DOLT_PORT=3307' 'B9S_DOLT_READ_USER=b9s_reader' 'B9S_DOLT_READ_PASSWORD="s3cr3t pass"' \
    >"$reader_dir/reader.env"
  contains "database egress follows the reader env file" "{protocol: TCP, port: 3307}" \
    "$(policy_egress "$(render_with PREVIEW_READER_ENV_FILE="$reader_dir/reader.env")")"
  refused_with "a reader port that disagrees with B9S_DOLT_PORT is refused" "disagrees with B9S_DOLT_PORT" \
    PREVIEW_READER_ENV_FILE="$reader_dir/reader.env" B9S_DOLT_PORT=3306
  refused_with "a named reader env file must exist" "reader env file is not a file" \
    PREVIEW_READER_ENV_FILE="$reader_dir/absent.env"
  grep -v PASSWORD "$reader_dir/reader.env" >"$reader_dir/no-password.env"
  refused_with "a reader env file without a password is refused" "B9S_DOLT_READ_PASSWORD is missing" \
    PREVIEW_READER_ENV_FILE="$reader_dir/no-password.env"
  if [[ ! -e /mnt/secrets/reader/reader.env ]]; then
    default_reader="$(render_with -u PREVIEW_READER_ENV_FILE)"
    contains "an absent default reader env file leaves the render unchanged" "{protocol: TCP, port: 3306}" "$default_reader"
  fi

  # A quota-enforcing cluster may admit preview Pods only within these bounds.
  quota="$(awk '/^kind: ResourceQuota$/{f=1} f && /^---$/{exit} f' <<<"$rendered")"
  for bound in 'pods: "3"' 'limits.cpu: "2"' 'limits.memory: 2Gi' 'persistentvolumeclaims: "0"'; do
    contains "quota bounds $bound" "$bound" "$quota"
  done
  if grep -Eq 'scopes|scopeSelector' <<<"$quota"; then bad "quota is unscoped"; else ok "quota is unscoped"; fi
  images="$(grep -c '^ *image: ' <<<"$rendered" || true)"
  limited="$(grep -cE '^ *limits: \{cpu: [^,]+, memory: [^}]+\}' <<<"$rendered" || true)"
  if [[ $images -gt 0 && $images == "$limited" ]]; then
    ok "every container sets cpu and memory limits"
  else
    bad "every container sets cpu and memory limits" "$images containers, $limited limited"
  fi
fi

# Model kubectl's partial apply: a denied Namespace does not stop it applying
# later documents. A server-side preflight must reject that deployment first.
redeploy_dir="$(mktemp -d "${TMPDIR:-/tmp}/b9s-redeploy-contract.XXXXXX")"
run_deploy() (
  export TEST_SHA="$1" TEST_DENY_NAMESPACE="$2" TEST_STATE_DIR="$3"
  export PREVIEW_READER_ENV_FILE="${4:-}" TEST_SECRET_EXISTS="${5:-0}"
  export B9S_PREVIEW_KUBECONFIG="$ROOT/go.mod" B9S_PREVIEW_CONTEXT=preview
  export B9S_PREVIEW_EVIDENCE_DIR="$TEST_STATE_DIR/evidence"
  mkdir -p "$TEST_STATE_DIR"

  git() {
    case "$*" in
      *'rev-parse HEAD') printf '%s\n' "$TEST_SHA" ;;
      *'cat-file -e '*|*'status --porcelain'*) return 0 ;;
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
        [[ $2 == b9s-bd-b6jw-2 ]] || return 91
        case "$3" in
          rollout) return 0 ;;
          wait) printf 'wait %s\n' "${*: -1}" >>"$TEST_STATE_DIR/calls"; return 0 ;;
          *) return 91 ;;
        esac ;;
      create)
        [[ $2 == --filename=- && $# == 2 ]] || return 97
        printf 'secret\n' >>"$TEST_STATE_DIR/calls"
        printf '%s\n' "$*" >>"$TEST_STATE_DIR/argv"
        cat >"$TEST_STATE_DIR/secret.yaml"
        if [[ $TEST_SECRET_EXISTS == 1 ]]; then
          printf 'Error from server (AlreadyExists): secrets "b9s-reader" already exists\n' >&2
          return 1
        fi ;;
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
        if $dry_run; then
          [[ $(grep -c '^kind:' "$file") == 1 ]] && grep -q '^kind: Namespace$' "$file" || return 95
          printf 'preflight\n' >>"$TEST_STATE_DIR/calls"
        else
          printf 'apply %s\n' "$(sed -n 's/^kind: //p' "$file" | paste -sd, -)" >>"$TEST_STATE_DIR/calls"
          if grep -q '^kind: Deployment$' "$file"; then
            printf '%s\n' "$TEST_SHA" >>"$TEST_STATE_DIR/workloads"
          fi
        fi
        if [[ $TEST_DENY_NAMESPACE == 1 ]]; then
          printf 'Forbidden: namespace metadata update denied by preview policy\n' >&2
          return 1
        fi
        if ! $dry_run && [[ $(sed -n '1,2{s/^kind: //p}' "$file") == Namespace ]]; then
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
contains "redeploy refreshes namespace commit label" "preview.example/commit-sha: $second_sha" \
  "$(cat "$redeploy_dir/allowed/namespace.yaml" 2>/dev/null)"
# Admission may refuse a Deployment applied before the namespace's quota,
# so the guardrails go first and the quota must be settled before workloads.
guarded_deploy=$'preflight\napply Namespace,RoleBinding,LimitRange,ResourceQuota,NetworkPolicy\nwait resourcequota/preview-quota\napply Deployment,Service,Ingress'
if [[ $(cat "$redeploy_dir/allowed/calls" 2>/dev/null) == "$guarded_deploy"$'\n'"$guarded_deploy" ]]; then
  ok "each deployment applies namespace guardrails and waits for the quota before workloads"
else
  bad "each deployment applies namespace guardrails and waits for the quota before workloads" \
    "$(cat "$redeploy_dir/allowed/calls" 2>/dev/null | paste -sd'|' -)"
fi
if [[ $(cat "$redeploy_dir/allowed/workloads" 2>/dev/null) == "$first_sha"$'\n'"$second_sha" ]]; then
  ok "redeploy applies the new workload SHA"
else
  bad "redeploy applies the new workload SHA"
fi

# A lane publishes an already-issued read-only credential from a mounted env
# file. The deployer may create Secrets but cannot read them, so the Secret is
# created (never applied, which reads first) after the quota and before the
# Deployment that mounts it.
reader_fixture="$redeploy_dir/reader.env"
printf '%s\n' 'B9S_DOLT_HOST=dolt-2.dolt-2.svc.cluster.local' 'B9S_DOLT_PORT=3307' \
  'B9S_DOLT_READ_USER=b9s_reader' "B9S_DOLT_READ_PASSWORD='s3cr3t pass'" >"$reader_fixture"
if run_deploy "$first_sha" 0 "$redeploy_dir/reader" "$reader_fixture" >"$redeploy_dir/reader.log" 2>&1; then
  ok "a lane deployment publishes the reader Secret"
else
  bad "a lane deployment publishes the reader Secret" "see $redeploy_dir/reader.log"
fi
reader_calls="$(paste -sd'|' - <"$redeploy_dir/reader/calls" 2>/dev/null)"
if [[ $reader_calls == 'preflight|apply Namespace,RoleBinding,LimitRange,ResourceQuota,NetworkPolicy|wait resourcequota/preview-quota|secret|apply Deployment,Service,Ingress' ]]; then
  ok "the reader Secret lands after the quota and before the Deployment"
else
  bad "the reader Secret lands after the quota and before the Deployment" "$reader_calls"
fi
secret="$(cat "$redeploy_dir/reader/secret.yaml" 2>/dev/null || true)"
contains "reader Secret takes the cluster-supplied name" "name: b9s-reader" "$secret"
contains "reader Secret lives in the preview namespace" "namespace: b9s-bd-b6jw-2" "$secret"
b64() { printf '%s' "$1" | base64 | tr -d '\n'; }
contains "reader Secret carries the host" "B9S_DOLT_HOST: $(b64 dolt-2.dolt-2.svc.cluster.local)" "$secret"
contains "reader Secret carries the port" "B9S_DOLT_PORT: $(b64 3307)" "$secret"
contains "reader Secret carries the user" "B9S_DOLT_READ_USER: $(b64 b9s_reader)" "$secret"
contains "reader Secret carries the unquoted password" "B9S_DOLT_READ_PASSWORD: $(b64 's3cr3t pass')" "$secret"
if grep -rqF 's3cr3t' "$redeploy_dir/reader/argv" "$redeploy_dir/reader/evidence" "$redeploy_dir/reader.log" 2>/dev/null; then
  bad "the reader password stays out of argv, evidence and logs"
else
  ok "the reader password stays out of argv, evidence and logs"
fi
if run_deploy "$second_sha" 0 "$redeploy_dir/reader" "$reader_fixture" 1 >"$redeploy_dir/reader-again.log" 2>&1; then
  ok "a redeploy accepts the reader Secret it cannot read back"
else
  bad "a redeploy accepts the reader Secret it cannot read back" "see $redeploy_dir/reader-again.log"
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
contains "namespace policy denial identifies the infrastructure prerequisite" 'the cluster must permit' \
  "$(cat "$redeploy_dir/denied.log")"

# The repository ships no deployment values: each one must come from the
# cluster, and a missing one stops before any cluster access.
for name in PREVIEW_HOST_SUFFIX PREVIEW_LABEL_DOMAIN PREVIEW_RUNNER_NAMESPACE \
  PREVIEW_WORKLOAD_ROLE PREVIEW_DEPLOYER_SA PREVIEW_READER_REF \
  PREVIEW_VIEWER_HEADER PREVIEW_VIEWER; do
  set +e
  missing="$(env -u "$name" B9S_PREVIEW_KUBECONFIG="$ROOT/go.mod" B9S_PREVIEW_CONTEXT=preview \
    "$timeout_bin" 20 "$PREVIEW/preview-status" bd-b6jw "$(git -C "$ROOT" rev-parse HEAD)" 2>&1)"
  missing_rc=$?
  set -e
  if [[ $missing_rc -ne 0 && $missing == *"$name is not set"* ]]; then
    ok "missing $name stops before cluster access"
  else
    bad "missing $name stops before cluster access" "exit $missing_rc"
  fi
done

set +e
bad_sa="$(PREVIEW_DEPLOYER_SA=no-namespace B9S_PREVIEW_KUBECONFIG="$ROOT/go.mod" B9S_PREVIEW_CONTEXT=preview \
  "$timeout_bin" 20 "$PREVIEW/preview-status" bd-b6jw "$(git -C "$ROOT" rev-parse HEAD)" 2>&1)"
bad_sa_rc=$?
set -e
if [[ $bad_sa_rc -ne 0 ]]; then ok "a deployer without namespace/name is refused"; else bad "a deployer without namespace/name is refused"; fi

# Simulate the transport boundary, while executing the real verifier and JSON
# parser. Each invocation has a deadline and records every HTTP destination.
verify_dir="$(mktemp -d "${TMPDIR:-/tmp}/b9s-verify-contract.XXXXXX")"
run_verify() (
  export TEST_SCENARIO="$1" TEST_SERVICE_RC="$2" TEST_INGRESS_RC="$3" TEST_STATE_DIR="$4"
  export TEST_SHA=1111111111111111111111111111111111111111
  export B9S_PREVIEW_KUBECONFIG="$ROOT/go.mod" B9S_PREVIEW_CONTEXT=preview
  export B9S_PREVIEW_EVIDENCE_DIR="$TEST_STATE_DIR/evidence"
  export B9S_PREVIEW_INGRESS_PORT=18081
  mkdir -p "$TEST_STATE_DIR"
  git() {
    case "$*" in
      *'rev-parse HEAD') printf '%s\n' "$TEST_SHA" ;;
      *'cat-file -e '*|*'status --porcelain'*) return 0 ;;
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
        [[ $2 == b9s-bd-b6jw-4 ]] || return 91
        shift 2
        case "$1" in
          rollout) return 0 ;;
          exec)
            printf '%s\n' "$*" >>"$TEST_STATE_DIR/kubectl"
            [[ $* == *'deploy/b9s -- tmux capture-pane -p -t b9s' ]] || return 92
            printf 'b9s terminal UI\n' ;;
          *) return 93 ;;
        esac ;;
      *) return 94 ;;
    esac
  }
  curl() {
    local url='' output='' header='' connect='' max='' flags=''
    while [[ $# -gt 0 ]]; do
      case "$1" in
        -fsS) flags="$1"; shift ;;
        --connect-timeout) connect="$2"; shift 2 ;;
        --max-time) max="$2"; shift 2 ;;
        -H) header="$2"; shift 2 ;;
        -o) output="$2"; shift 2 ;;
        http://*) url="$1"; shift ;;
        *) return 95 ;;
      esac
    done
    [[ $flags == -fsS && $connect == 3 && $max == 10 && -n $output ]] || return 96
    printf '%s\n' "$url" >>"$TEST_STATE_DIR/http"
    case "$url" in
      */__preview)
        if [[ $url == *svc.cluster.local* ]]; then
          [[ $TEST_SERVICE_RC == 0 ]] || return "$TEST_SERVICE_RC"
        else
          [[ $TEST_INGRESS_RC == 0 ]] || return "$TEST_INGRESS_RC"
        fi
        local commit="$TEST_SHA" task=bd-b6jw.4 namespace=b9s-bd-b6jw-4
        case "$TEST_SCENARIO" in
          bad-commit) commit=2222222222222222222222222222222222222222 ;;
          bad-task) task=bd-other ;;
          bad-namespace) namespace=b9s-bd-other ;;
          malformed) printf '{' >"$output"; return 0 ;;
          array) printf '[]' >"$output"; return 0 ;;
          missing) printf '{"commit":"%s"}' "$commit" >"$output"; return 0 ;;
          nonstring) printf '{"commit":42,"taskId":"bd-b6jw.4","namespace":"b9s-bd-b6jw-4"}' >"$output"; return 0 ;;
        esac
        printf '{"commit":"%s","taskId":"%s","namespace":"%s"}' "$commit" "$task" "$namespace" >"$output" ;;
      */api/session)
        [[ $header == "$PREVIEW_VIEWER_HEADER: $PREVIEW_VIEWER" ]] || return 97
        [[ $TEST_SCENARIO != web-failure ]] || return 22
        local commit="$TEST_SHA"
        [[ $TEST_SCENARIO != web-commit ]] || commit=2222222222222222222222222222222222222222
        printf '{"commit":"%s"}' "$commit" >"$output" ;;
      *) return 98 ;;
    esac
  }
  export -f git kubectl curl
  "$timeout_bin" 5s bash "$PREVIEW/preview-verify" bd-b6jw.4 "$TEST_SHA"
)

verify_case() {
  local name="$1" scenario="$2" service_rc="$3" ingress_rc="$4"
  local expected_rc="$5" boundary="$6" session="$7" detail="${8:-}"
  local state="$verify_dir/$name" output rc expected
  if output="$(run_verify "$scenario" "$service_rc" "$ingress_rc" "$state" 2>&1)"; then rc=0; else rc=$?; fi
  printf '%s\n' "$output" >"$state/result.log"
  if [[ $rc == "$expected_rc" ]]; then ok "verifier $name exit $expected_rc"; else bad "verifier $name exit $expected_rc" "exit $rc: $output"; fi
  contains "verifier $name prints final counts" '=== preview-verify DONE pass=' "$output"
  expected='http://b9s.b9s-bd-b6jw-4.svc.cluster.local:7682/__preview'
  if [[ $boundary == ingress ]]; then
    expected+=$'\nhttp://b9s-bd-b6jw-4.previews.example.test:18081/__preview'
  fi
  if [[ $session == 1 ]]; then
    if [[ $boundary == ingress ]]; then
      expected+=$'\nhttp://b9s-bd-b6jw-4.previews.example.test:18081/api/session'
    else
      expected+=$'\nhttp://b9s.b9s-bd-b6jw-4.svc.cluster.local:7681/api/session'
    fi
  fi
  if [[ $(cat "$state/http" 2>/dev/null) == "$expected" ]]; then
    ok "verifier $name uses only the selected HTTP boundary with bounded requests"
  else
    bad "verifier $name uses only the selected HTTP boundary with bounded requests" "$(cat "$state/http" 2>/dev/null)"
  fi
  if [[ $session == 1 ]]; then
    contains "verifier $name reads the task terminal through Kubernetes" 'deploy/b9s -- tmux capture-pane -p -t b9s' \
      "$(cat "$state/kubectl" 2>/dev/null)"
  fi
  if [[ -n $detail ]]; then contains "verifier $name reports the failure" "$detail" "$output"; fi
}

verify_case cluster healthy 0 0 0 service 1
for transport in 6 7 28; do
  verify_case "laptop-$transport" healthy "$transport" 0 0 ingress 1
done
for transport in 5 22 35 52 56; do
  verify_case "service-error-$transport" healthy "$transport" 0 1 service 0 identity-http
done
verify_case ingress-unreachable healthy 6 7 1 ingress 0 'check preview DNS, ingress access'
for boundary in service ingress; do
  service_rc=0
  [[ $boundary != ingress ]] || service_rc=6
  for field in commit task namespace; do
    verify_case "$boundary-bad-$field" "bad-$field" "$service_rc" 0 1 "$boundary" 1 "served-$field"
  done
  for invalid in malformed array missing nonstring; do
    verify_case "$boundary-$invalid" "$invalid" "$service_rc" 0 1 "$boundary" 0 identity-json
  done
  verify_case "$boundary-web-failure" web-failure "$service_rc" 0 1 "$boundary" 1 web-board
  verify_case "$boundary-web-commit" web-commit "$service_rc" 0 1 "$boundary" 1 web-board
done

printf '=== preview-contract DONE pass=%d fail=%d ===\n' "$pass" "$fail"
[[ $fail -eq 0 ]]
