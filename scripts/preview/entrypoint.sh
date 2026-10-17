#!/bin/sh
set -eu

: "${PREVIEW_TASK_ID:?required}"
: "${PREVIEW_NAMESPACE:?required}"
: "${B9S_DOLT_HOST:?required}"
: "${B9S_DOLT_PORT:?required}"
: "${B9S_DOLT_READ_USER:?required}"
: "${B9S_DOLT_READ_PASSWORD:?required}"
: "${PREVIEW_VIEWER_HEADER:?required}"
: "${PREVIEW_VIEWER:?required}"

case "$B9S_DOLT_HOST" in
  *[!A-Za-z0-9._:-]*|'') printf 'invalid B9S_DOLT_HOST\n' >&2; exit 1 ;;
esac
case "$B9S_DOLT_PORT" in
  *[!0-9]*|'') printf 'invalid B9S_DOLT_PORT\n' >&2; exit 1 ;;
esac
case "$B9S_DOLT_READ_USER" in
  *[!A-Za-z0-9_]*|'') printf 'invalid B9S_DOLT_READ_USER\n' >&2; exit 1 ;;
esac

commit_sha="$(sed -n '1p' /app/.commit-sha)"
mkdir -p /data/projects /tmp/.config/b9s /www

export HOME=/tmp
export TERM=xterm-256color
export BW_NO_BROWSER=1
export BW_TEST_MODE=1
export BEADS_DOLT_PASSWORD="$B9S_DOLT_READ_PASSWORD"
export B9S_TRUSTED_DOLT_ENDPOINTS="$B9S_DOLT_HOST:$B9S_DOLT_PORT"

dolt_query() {
  MYSQL_PWD="$B9S_DOLT_READ_PASSWORD" mariadb \
    --protocol=TCP \
    --host="$B9S_DOLT_HOST" \
    --port="$B9S_DOLT_PORT" \
    --user="$B9S_DOLT_READ_USER" \
    --batch --skip-column-names --raw \
    --execute="$1"
}

dolt_query 'SHOW DATABASES' > /tmp/accessible-databases
printf 'projects:\n' > /tmp/.config/b9s/config.yaml

database_count=0
current_database=''
while IFS= read -r database; do
  case "$database" in
    information_schema|mysql|performance_schema|sys|'') continue ;;
    *[!A-Za-z0-9_-]*) continue ;;
  esac

  if ! dolt_query "SELECT 1 FROM \`$database\`.issues LIMIT 0" >/dev/null 2>&1; then
    continue
  fi

  project_dir="/data/projects/$database"
  mkdir -p "$project_dir/.beads"
  printf '{"database":"dolt","backend":"dolt","dolt_mode":"server","dolt_server_host":"%s","dolt_server_port":%s,"dolt_server_user":"%s","dolt_database":"%s"}\n' \
    "$B9S_DOLT_HOST" "$B9S_DOLT_PORT" "$B9S_DOLT_READ_USER" "$database" \
    > "$project_dir/.beads/metadata.json"
  printf '  - name: "%s"\n    path: "%s"\n' "$database" "$project_dir" \
    >> /tmp/.config/b9s/config.yaml

  database_count=$((database_count + 1))
  if [ -z "$current_database" ] || [ "$database" = b9s ]; then
    current_database="$database"
  fi
done < /tmp/accessible-databases

if [ "$database_count" -eq 0 ]; then
  printf 'the read-only account cannot access a Beads database\n' >&2
  exit 1
fi

export BEADS_DIR="/data/projects/$current_database/.beads"
printf '%s\n' "/data/projects/$current_database" > /tmp/b9s-preview-project
printf '{"commit":"%s","taskId":"%s","namespace":"%s","databaseCount":%s}\n' \
  "$commit_sha" "$PREVIEW_TASK_ID" "$PREVIEW_NAMESPACE" "$database_count" > /www/__preview
httpd -f -p 7682 -h /www &

# The terminal UI waits in a detached session. A reviewer or the deployer
# attaches with kubectl exec, so the TUI is tested in a real terminal and
# never through a browser.
tmux new-session -d -s b9s -c "/data/projects/$current_database" /usr/local/bin/preview-tui

(
  attempts=0
  while [ "$attempts" -lt 100 ]; do
    if tmux capture-pane -p -t b9s 2>/dev/null | grep -Eq '0.*All'; then
      tmux send-keys -t b9s 0
      exit 0
    fi
    attempts=$((attempts + 1))
    sleep 0.1
  done
) &

# Only the ingress reaches this port, and the ingress middleware sets the
# viewer header on every request, replacing any copy a client sent (ADR 0021).
cd "/data/projects/$current_database"
exec /usr/local/bin/b9s web --listen 0.0.0.0:7681 \
  --trust-header "$PREVIEW_VIEWER_HEADER" --owner "$PREVIEW_VIEWER" \
  --projects-root /data/projects --discover-dolt
