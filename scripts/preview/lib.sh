#!/usr/bin/env bash
# Shared, repository-owned contract for the five preview commands.
set -euo pipefail

PREVIEW_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PREVIEW_ROOT="$(git -C "$PREVIEW_DIR" rev-parse --show-toplevel)"
PREVIEW_PASS=0
PREVIEW_FAIL=0

preview_log() { printf '[%s] %s\n' "$PREVIEW_CMD" "$*" >&2; }
check_pass() { PREVIEW_PASS=$((PREVIEW_PASS + 1)); printf 'PASS  %s: %s\n' "$1" "${2:-}"; }
check_fail() { PREVIEW_FAIL=$((PREVIEW_FAIL + 1)); printf 'FAIL  %s: %s\n' "$1" "${2:-}" >&2; }

preview_finish() {
  local verdict=pass
  [[ $PREVIEW_FAIL -eq 0 ]] || verdict=fail
  printf 'PREVIEW_URL=%s\n' "$PREVIEW_URL"
  printf 'PREVIEW_IDENTITY task=%s commit=%s namespace=%s image=%s\n' \
    "$PREVIEW_TASK_ID" "$PREVIEW_COMMIT_SHA" "$PREVIEW_NAMESPACE" "$PREVIEW_IMAGE"
  printf '=== %s DONE pass=%d fail=%d verdict=%s ===\n' \
    "$PREVIEW_CMD" "$PREVIEW_PASS" "$PREVIEW_FAIL" "$verdict"
  [[ $PREVIEW_FAIL -eq 0 ]]
}

preview_die() {
  printf '[%s] BLOCKED (%s) %s\n' "$PREVIEW_CMD" "$1" "$2" >&2
  exit 2
}

preview_usage() {
  printf 'usage: %s TASK_ID COMMIT_SHA [--kubeconfig PATH] [--context NAME]\n' "$PREVIEW_CMD" >&2
}

preview_require_env() {
  local name
  for name in "$@"; do
    [[ -n ${!name:-} ]] || preview_die usage "$name is not set; the cluster supplies it (docs/preview-contract.md)"
  done
}

preview_parse_args() {
  PREVIEW_TASK_ID=""
  PREVIEW_COMMIT_SHA=""
  PREVIEW_KUBECONFIG="${B9S_PREVIEW_KUBECONFIG:-/mnt/secrets/preview/preview.kubeconfig}"
  PREVIEW_CONTEXT="${B9S_PREVIEW_CONTEXT:-preview}"

  local positional=()
  while [[ $# -gt 0 ]]; do
    case "$1" in
      --kubeconfig) PREVIEW_KUBECONFIG="${2:-}"; shift 2 ;;
      --kubeconfig=*) PREVIEW_KUBECONFIG="${1#*=}"; shift ;;
      --context) PREVIEW_CONTEXT="${2:-}"; shift 2 ;;
      --context=*) PREVIEW_CONTEXT="${1#*=}"; shift ;;
      -h|--help) preview_usage; exit 0 ;;
      --*) preview_usage; preview_die usage "unknown option $1" ;;
      *) positional+=("$1"); shift ;;
    esac
  done

  [[ ${#positional[@]} -eq 2 ]] || { preview_usage; preview_die usage "expected TASK_ID and COMMIT_SHA"; }
  PREVIEW_TASK_ID="${positional[0]}"
  PREVIEW_COMMIT_SHA="${positional[1]}"

  [[ $PREVIEW_TASK_ID =~ ^[A-Za-z0-9][A-Za-z0-9._-]{0,62}$ ]] \
    || preview_die usage "TASK_ID is not a plain identifier"
  [[ $PREVIEW_COMMIT_SHA =~ ^[0-9a-f]{40}$ ]] \
    || preview_die usage "COMMIT_SHA must be a full lowercase SHA"
  [[ -f $PREVIEW_KUBECONFIG ]] || preview_die usage "kubeconfig is not a file: $PREVIEW_KUBECONFIG"
  [[ $PREVIEW_CONTEXT =~ ^[A-Za-z0-9][A-Za-z0-9._-]{0,127}$ ]] \
    || preview_die usage "context is not a plain identifier"

  local task_slug
  task_slug="$(printf '%s' "$PREVIEW_TASK_ID" | tr '[:upper:]' '[:lower:]' | tr -c 'a-z0-9' '-' | sed -e 's/--*/-/g' -e 's/^-//' -e 's/-$//')"
  PREVIEW_NAMESPACE="b9s-${task_slug#b9s-}"
  [[ ${#PREVIEW_NAMESPACE} -le 63 ]] || preview_die usage "namespace exceeds 63 characters"

  # The cluster supplies the names its own RBAC and admission policies match
  # (docs/preview-contract.md, "Deployment values"). They have no defaults: a
  # guessed name renders objects the platform refuses or cannot find again.
  preview_require_env PREVIEW_HOST_SUFFIX PREVIEW_LABEL_DOMAIN PREVIEW_RUNNER_NAMESPACE \
    PREVIEW_WORKLOAD_ROLE PREVIEW_DEPLOYER_SA PREVIEW_READER_REF \
    PREVIEW_VIEWER_HEADER PREVIEW_VIEWER
  [[ $PREVIEW_VIEWER_HEADER =~ ^[A-Za-z0-9-]+$ ]] \
    || preview_die usage "PREVIEW_VIEWER_HEADER is not an HTTP header name"
  [[ $PREVIEW_VIEWER =~ ^[^[:space:]\"@]+@[^[:space:]\"@]+$ ]] \
    || preview_die usage "PREVIEW_VIEWER is not an email address"
  [[ $PREVIEW_LABEL_DOMAIN =~ ^[a-z0-9]([-a-z0-9.]*[a-z0-9])?$ ]] \
    || preview_die usage "PREVIEW_LABEL_DOMAIN is not a DNS name"
  local name
  for name in PREVIEW_RUNNER_NAMESPACE PREVIEW_WORKLOAD_ROLE; do
    [[ ${!name} =~ ^[a-z0-9]([-a-z0-9]*[a-z0-9])?$ ]] || preview_die usage "$name is not a Kubernetes name"
  done
  PREVIEW_DEPLOYER_SA="${PREVIEW_DEPLOYER_SA//\{project\}/b9s}"
  [[ $PREVIEW_DEPLOYER_SA =~ ^([a-z0-9]([-a-z0-9]*[a-z0-9])?)/([a-z0-9]([-a-z0-9]*[a-z0-9])?)$ ]] \
    || preview_die usage "PREVIEW_DEPLOYER_SA is not NAMESPACE/NAME"
  PREVIEW_DEPLOYER_SA_NAMESPACE="${BASH_REMATCH[1]}"
  PREVIEW_DEPLOYER_SA_NAME="${BASH_REMATCH[3]}"
  PREVIEW_READER_REF="${PREVIEW_READER_REF//\{project\}/b9s}"
  [[ $PREVIEW_READER_REF =~ ^[a-z0-9]([-a-z0-9]*[a-z0-9])?$ ]] \
    || preview_die usage "PREVIEW_READER_REF is not a Kubernetes name"

  [[ $PREVIEW_HOST_SUFFIX =~ ^[a-z0-9]([-a-z0-9.]*[a-z0-9])?$ ]] \
    || preview_die usage "host suffix is not valid"
  PREVIEW_HOST="${PREVIEW_NAMESPACE}.${PREVIEW_HOST_SUFFIX}"
  PREVIEW_INGRESS_PORT="${B9S_PREVIEW_INGRESS_PORT:-8081}"
  [[ $PREVIEW_INGRESS_PORT =~ ^[0-9]{1,5}$ ]] || preview_die usage "ingress port is not valid"
  PREVIEW_URL="http://${PREVIEW_HOST}:${PREVIEW_INGRESS_PORT}"
  # The ingress controller may be public, and the board trusts a viewer header,
  # so a cluster names the Middlewares that restrict access and set that header,
  # as a comma-separated list in the order Traefik applies them. Empty renders
  # none, for a cluster that defines no such Middleware.
  PREVIEW_INGRESS_MIDDLEWARE="${PREVIEW_INGRESS_MIDDLEWARE:-}"
  local middleware_ref='[a-z0-9]([-a-z0-9]*[a-z0-9])?@kubernetescrd'
  [[ -z $PREVIEW_INGRESS_MIDDLEWARE || $PREVIEW_INGRESS_MIDDLEWARE =~ ^${middleware_ref}(,${middleware_ref})*$ ]] \
    || preview_die usage "ingress middleware is not a list of Traefik CRD references"

  PREVIEW_REGISTRY="${B9S_PREVIEW_REGISTRY:-registry.registry.svc.cluster.local:5000}"
  [[ $PREVIEW_REGISTRY =~ ^[a-z0-9]([-a-z0-9.:/]*[a-z0-9])?$ ]] \
    || preview_die usage "registry is not valid"
  PREVIEW_IMAGE="${PREVIEW_REGISTRY}/b9s-preview:${PREVIEW_COMMIT_SHA}"
  PREVIEW_PUSH_REGISTRY="${B9S_PREVIEW_PUSH_REGISTRY:-$PREVIEW_REGISTRY}"
  PREVIEW_PUSH_IMAGE="${PREVIEW_PUSH_REGISTRY}/b9s-preview:${PREVIEW_COMMIT_SHA}"
  PREVIEW_EVIDENCE_DIR="${B9S_PREVIEW_EVIDENCE_DIR:-${TMPDIR:-/tmp}/b9s-preview/${PREVIEW_NAMESPACE}}"
  mkdir -p "$PREVIEW_EVIDENCE_DIR"

  export PREVIEW_LABEL_DOMAIN
  export PREVIEW_TASK_ID PREVIEW_COMMIT_SHA PREVIEW_NAMESPACE PREVIEW_HOST PREVIEW_URL
  export PREVIEW_VIEWER_HEADER PREVIEW_VIEWER
  export PREVIEW_KUBECONFIG PREVIEW_CONTEXT PREVIEW_IMAGE PREVIEW_PUSH_IMAGE PREVIEW_EVIDENCE_DIR
}

preview_require_boundary() {
  git -C "$PREVIEW_ROOT" cat-file -e "${PREVIEW_COMMIT_SHA}^{commit}" 2>/dev/null \
    || preview_die boundary "commit is absent from this repository"
  local head dirty
  head="$(git -C "$PREVIEW_ROOT" rev-parse HEAD)"
  [[ $head == "$PREVIEW_COMMIT_SHA" ]] \
    || preview_die boundary "HEAD is $head, not $PREVIEW_COMMIT_SHA"
  # These local Beads/harness artifacts are excluded by Dockerfile.dockerignore.
  # Only runtime statuses are exempt: staged changes and Beads data still block.
  dirty="$(git -C "$PREVIEW_ROOT" status --porcelain=v1 --untracked-files=all | sed \
    -e '/^?? \.codex-tmp\//d' \
    -e '/^ M \.beads\/\.local_version$/d' \
    -e '/^?? \.beads\.gate\.lock$/d')"
  [[ -z $dirty ]] || preview_die boundary "worktree is dirty"
  check_pass artifact-boundary "$PREVIEW_COMMIT_SHA"
}

kc() { kubectl --kubeconfig "$PREVIEW_KUBECONFIG" --context "$PREVIEW_CONTEXT" "$@"; }
kcn() { kc --namespace "$PREVIEW_NAMESPACE" "$@"; }

preview_require_cluster() {
  command -v kubectl >/dev/null 2>&1 || preview_die infrastructure "kubectl is not installed"
  kc version --request-timeout=10s >/dev/null 2>&1 || preview_die infrastructure "Kubernetes API is unreachable"
}

preview_render_manifests() {
  local output="$1"
  local ingress_annotations=""
  if [[ -n $PREVIEW_INGRESS_MIDDLEWARE ]]; then
    ingress_annotations="$(printf '%s\n' \
      "  annotations:" \
      "    traefik.ingress.kubernetes.io/router.middlewares: ${PREVIEW_INGRESS_MIDDLEWARE}")"
  fi
  cat >"$output" <<YAML
apiVersion: v1
kind: Namespace
metadata:
  name: ${PREVIEW_NAMESPACE}
  labels:
    app.kubernetes.io/name: b9s
    app.kubernetes.io/managed-by: b9s-preview
    ${PREVIEW_LABEL_DOMAIN}/preview: "true"
    ${PREVIEW_LABEL_DOMAIN}/project: b9s
    ${PREVIEW_LABEL_DOMAIN}/task-id: ${PREVIEW_TASK_ID}
    ${PREVIEW_LABEL_DOMAIN}/commit-sha: ${PREVIEW_COMMIT_SHA}
---
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: preview-workloads
  namespace: ${PREVIEW_NAMESPACE}
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: ${PREVIEW_WORKLOAD_ROLE}
subjects:
  - kind: ServiceAccount
    name: ${PREVIEW_DEPLOYER_SA_NAME}
    namespace: ${PREVIEW_DEPLOYER_SA_NAMESPACE}
---
apiVersion: v1
kind: LimitRange
metadata:
  name: preview-defaults
  namespace: ${PREVIEW_NAMESPACE}
spec:
  limits:
    - type: Container
      default: {cpu: 500m, memory: 384Mi}
      defaultRequest: {cpu: 25m, memory: 96Mi}
---
apiVersion: v1
kind: ResourceQuota
metadata:
  name: preview-quota
  namespace: ${PREVIEW_NAMESPACE}
spec:
  hard:
    pods: "3"
    services: "2"
    requests.cpu: "1"
    requests.memory: 1Gi
    limits.cpu: "2"
    limits.memory: 2Gi
    count/deployments.apps: "2"
    count/ingresses.networking.k8s.io: "1"
    persistentvolumeclaims: "0"
---
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: b9s-preview
  namespace: ${PREVIEW_NAMESPACE}
spec:
  podSelector:
    matchLabels:
      app.kubernetes.io/name: b9s
  policyTypes: [Ingress, Egress]
  ingress:
    - from:
        - namespaceSelector:
            matchLabels:
              kubernetes.io/metadata.name: kube-system
      ports:
        - {protocol: TCP, port: 7681}
        - {protocol: TCP, port: 7682}
    - from:
        - namespaceSelector:
            matchLabels:
              kubernetes.io/metadata.name: ${PREVIEW_RUNNER_NAMESPACE}
      ports:
        - {protocol: TCP, port: 7681}
        - {protocol: TCP, port: 7682}
  egress:
    - to:
        - namespaceSelector:
            matchLabels:
              kubernetes.io/metadata.name: kube-system
      ports:
        - {protocol: UDP, port: 53}
        - {protocol: TCP, port: 53}
    - ports:
        - {protocol: TCP, port: 3306}
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: b9s
  namespace: ${PREVIEW_NAMESPACE}
  labels:
    app.kubernetes.io/name: b9s
    app.kubernetes.io/version: ${PREVIEW_COMMIT_SHA}
    ${PREVIEW_LABEL_DOMAIN}/task-id: ${PREVIEW_TASK_ID}
spec:
  replicas: 1
  revisionHistoryLimit: 2
  selector:
    matchLabels:
      app.kubernetes.io/name: b9s
      app.kubernetes.io/instance: ${PREVIEW_NAMESPACE}
  template:
    metadata:
      labels:
        app.kubernetes.io/name: b9s
        app.kubernetes.io/instance: ${PREVIEW_NAMESPACE}
        app.kubernetes.io/version: ${PREVIEW_COMMIT_SHA}
        ${PREVIEW_LABEL_DOMAIN}/task-id: ${PREVIEW_TASK_ID}
    spec:
      automountServiceAccountToken: false
      securityContext:
        runAsNonRoot: true
        runAsUser: 65532
        runAsGroup: 65532
        fsGroup: 65532
        seccompProfile: {type: RuntimeDefault}
      containers:
        - name: b9s
          image: ${PREVIEW_IMAGE}
          imagePullPolicy: IfNotPresent
          env:
            - {name: PREVIEW_TASK_ID, value: "${PREVIEW_TASK_ID}"}
            - {name: PREVIEW_NAMESPACE, value: "${PREVIEW_NAMESPACE}"}
            - {name: PREVIEW_VIEWER_HEADER, value: "${PREVIEW_VIEWER_HEADER}"}
            - {name: PREVIEW_VIEWER, value: "${PREVIEW_VIEWER}"}
          envFrom:
            - secretRef:
                name: ${PREVIEW_READER_REF}
          ports:
            - {name: web, containerPort: 7681}
            - {name: identity, containerPort: 7682}
          readinessProbe:
            exec:
              command:
                - sh
                - -ec
                - |
                  wget -q -T 1 -O /dev/null http://127.0.0.1:7682/__preview
                  wget -q -T 1 -O /dev/null --header="\${PREVIEW_VIEWER_HEADER}: \${PREVIEW_VIEWER}" http://127.0.0.1:7681/api/session
                  pane="\$(tmux capture-pane -p -t b9s)"
                  printf '%s\n' "\$pane" | grep -q '[^[:space:]]'
            initialDelaySeconds: 2
            periodSeconds: 3
            timeoutSeconds: 4
            failureThreshold: 20
          livenessProbe:
            httpGet: {path: /__preview, port: identity}
            initialDelaySeconds: 15
            periodSeconds: 10
            timeoutSeconds: 3
          resources:
            requests: {cpu: 25m, memory: 96Mi}
            limits: {cpu: 500m, memory: 512Mi}
          securityContext:
            allowPrivilegeEscalation: false
            readOnlyRootFilesystem: true
            capabilities: {drop: ["ALL"]}
          volumeMounts:
            - {name: data, mountPath: /data}
            - {name: web, mountPath: /www}
            - {name: tmp, mountPath: /tmp}
      volumes:
        - {name: data, emptyDir: {}}
        - {name: web, emptyDir: {}}
        - {name: tmp, emptyDir: {}}
---
apiVersion: v1
kind: Service
metadata:
  name: b9s
  namespace: ${PREVIEW_NAMESPACE}
spec:
  selector:
    app.kubernetes.io/name: b9s
    app.kubernetes.io/instance: ${PREVIEW_NAMESPACE}
  ports:
    - {name: web, port: 7681, targetPort: web}
    - {name: identity, port: 7682, targetPort: identity}
---
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: b9s
  namespace: ${PREVIEW_NAMESPACE}
${ingress_annotations}
spec:
  ingressClassName: traefik
  rules:
    - host: ${PREVIEW_HOST}
      http:
        paths:
          - path: /__preview
            pathType: Exact
            backend:
              service:
                name: b9s
                port: {name: identity}
          - path: /
            pathType: Prefix
            backend:
              service:
                name: b9s
                port: {name: web}
YAML
}

preview_wait_for_rollout() {
  local timeout="${B9S_PREVIEW_ROLLOUT_TIMEOUT:-180}"
  kcn rollout status deployment/b9s --timeout="${timeout}s"
}

preview_identity_url() {
  printf 'http://b9s.%s.svc.cluster.local:7682/__preview' "$PREVIEW_NAMESPACE"
}

preview_web_url() {
  printf 'http://b9s.%s.svc.cluster.local:7681/' "$PREVIEW_NAMESPACE"
}
