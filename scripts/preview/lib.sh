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

  PREVIEW_HOST_SUFFIX="${B9S_PREVIEW_HOST_SUFFIX:-previews.osenco.test}"
  [[ $PREVIEW_HOST_SUFFIX =~ ^[a-z0-9]([-a-z0-9.]*[a-z0-9])?$ ]] \
    || preview_die usage "host suffix is not valid"
  PREVIEW_HOST="${PREVIEW_NAMESPACE}.${PREVIEW_HOST_SUFFIX}"
  PREVIEW_INGRESS_PORT="${B9S_PREVIEW_INGRESS_PORT:-8081}"
  [[ $PREVIEW_INGRESS_PORT =~ ^[0-9]{1,5}$ ]] || preview_die usage "ingress port is not valid"
  PREVIEW_URL="http://${PREVIEW_HOST}:${PREVIEW_INGRESS_PORT}"

  PREVIEW_REGISTRY="${B9S_PREVIEW_REGISTRY:-registry.registry.svc.cluster.local:5000}"
  [[ $PREVIEW_REGISTRY =~ ^[a-z0-9]([-a-z0-9.:/]*[a-z0-9])?$ ]] \
    || preview_die usage "registry is not valid"
  PREVIEW_IMAGE="${PREVIEW_REGISTRY}/b9s-preview:${PREVIEW_COMMIT_SHA}"
  PREVIEW_PUSH_REGISTRY="${B9S_PREVIEW_PUSH_REGISTRY:-$PREVIEW_REGISTRY}"
  PREVIEW_PUSH_IMAGE="${PREVIEW_PUSH_REGISTRY}/b9s-preview:${PREVIEW_COMMIT_SHA}"
  PREVIEW_EVIDENCE_DIR="${B9S_PREVIEW_EVIDENCE_DIR:-${TMPDIR:-/tmp}/b9s-preview/${PREVIEW_NAMESPACE}}"
  mkdir -p "$PREVIEW_EVIDENCE_DIR"

  export PREVIEW_TASK_ID PREVIEW_COMMIT_SHA PREVIEW_NAMESPACE PREVIEW_HOST PREVIEW_URL
  export PREVIEW_KUBECONFIG PREVIEW_CONTEXT PREVIEW_IMAGE PREVIEW_PUSH_IMAGE PREVIEW_EVIDENCE_DIR
}

preview_require_boundary() {
  git -C "$PREVIEW_ROOT" cat-file -e "${PREVIEW_COMMIT_SHA}^{commit}" 2>/dev/null \
    || preview_die boundary "commit is absent from this repository"
  local head dirty
  head="$(git -C "$PREVIEW_ROOT" rev-parse HEAD)"
  [[ $head == "$PREVIEW_COMMIT_SHA" ]] \
    || preview_die boundary "HEAD is $head, not $PREVIEW_COMMIT_SHA"
  dirty="$(git -C "$PREVIEW_ROOT" status --porcelain --untracked-files=all | sed '/^?? \.codex-tmp\//d')"
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
  cat >"$output" <<YAML
apiVersion: v1
kind: Namespace
metadata:
  name: ${PREVIEW_NAMESPACE}
  labels:
    app.kubernetes.io/name: b9s
    app.kubernetes.io/managed-by: b9s-preview
    omnigent.osenco.dev/preview: "true"
    omnigent.osenco.dev/project: b9s
    omnigent.osenco.dev/task-id: ${PREVIEW_TASK_ID}
    omnigent.osenco.dev/commit-sha: ${PREVIEW_COMMIT_SHA}
---
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: preview-workloads
  namespace: ${PREVIEW_NAMESPACE}
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: omnigent-preview-workloads
subjects:
  - kind: ServiceAccount
    name: omnigent-preview-deployer-b9s
    namespace: omnigent
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
        - {protocol: TCP, port: 7683}
    - from:
        - namespaceSelector:
            matchLabels:
              kubernetes.io/metadata.name: omnigent-sandboxes
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
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: b9s
  namespace: ${PREVIEW_NAMESPACE}
  labels:
    app.kubernetes.io/name: b9s
    app.kubernetes.io/version: ${PREVIEW_COMMIT_SHA}
    omnigent.osenco.dev/task-id: ${PREVIEW_TASK_ID}
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
        omnigent.osenco.dev/task-id: ${PREVIEW_TASK_ID}
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
          ports:
            - {name: terminal, containerPort: 7681}
            - {name: identity, containerPort: 7682}
            - {name: mobile-terminal, containerPort: 7683}
          readinessProbe:
            httpGet: {path: /__preview, port: identity}
            initialDelaySeconds: 2
            periodSeconds: 3
            timeoutSeconds: 2
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
    - {name: terminal, port: 7681, targetPort: terminal}
    - {name: identity, port: 7682, targetPort: identity}
    - {name: mobile-terminal, port: 7683, targetPort: mobile-terminal}
---
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: b9s
  namespace: ${PREVIEW_NAMESPACE}
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
          - path: /b9s/terminal
            pathType: Prefix
            backend:
              service:
                name: b9s
                port: {name: mobile-terminal}
          - path: /cgi-bin/b9s-key
            pathType: Exact
            backend:
              service:
                name: b9s
                port: {name: identity}
          - path: /b9s
            pathType: Prefix
            backend:
              service:
                name: b9s
                port: {name: identity}
          - path: /
            pathType: Prefix
            backend:
              service:
                name: b9s
                port: {name: terminal}
YAML
}

preview_wait_for_rollout() {
  local timeout="${B9S_PREVIEW_ROLLOUT_TIMEOUT:-180}"
  kcn rollout status deployment/b9s --timeout="${timeout}s"
}

preview_identity_url() {
  printf 'http://b9s.%s.svc.cluster.local:7682/__preview' "$PREVIEW_NAMESPACE"
}

preview_terminal_url() {
  printf 'http://b9s.%s.svc.cluster.local:7681/' "$PREVIEW_NAMESPACE"
}
