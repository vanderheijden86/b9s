#!/bin/sh
set -eu

: "${PREVIEW_TASK_ID:?required}"
: "${PREVIEW_NAMESPACE:?required}"

commit_sha="$(sed -n '1p' /app/.commit-sha)"
mkdir -p /data/.beads /www/b9s /www/cgi-bin

cat > /data/.beads/issues.jsonl <<'JSONL'
{"id":"demo","title":"Epic: Make dependency blocking visible in the TUI","status":"open","priority":1,"issue_type":"epic","created_at":"2026-09-12T09:00:00Z","updated_at":"2026-09-12T09:00:00Z"}
{"id":"demo.1","title":"Repair interactive dependency graph view","status":"deferred","priority":2,"issue_type":"task","parent":"demo","created_at":"2026-09-12T09:01:00Z","updated_at":"2026-09-12T09:01:00Z"}
{"id":"demo.2","title":"Show computed dependency blockers in tree view","status":"deferred","priority":1,"issue_type":"task","parent":"demo","created_at":"2026-09-12T09:02:00Z","updated_at":"2026-09-12T09:02:00Z"}
{"id":"demo.3","title":"Show dispatcher lane stage in overview","status":"closed","priority":1,"issue_type":"task","parent":"demo","created_at":"2026-09-12T09:03:00Z","updated_at":"2026-09-12T09:03:00Z","closed_at":"2026-09-12T10:00:00Z"}
JSONL

printf '{"commit":"%s","taskId":"%s","namespace":"%s"}\n' \
  "$commit_sha" "$PREVIEW_TASK_ID" "$PREVIEW_NAMESPACE" > /www/__preview
cp /usr/local/share/b9s-preview/mobile.html /www/b9s/index.html
cp /usr/local/share/b9s-preview/mobile-key.cgi /www/cgi-bin/b9s-key
cp /www/__preview /www/b9s/__preview

export BEADS_DIR=/data/.beads
export HOME=/tmp
export TERM=xterm-256color
export BW_NO_BROWSER=1
export BW_TEST_MODE=1

httpd -f -p 7682 -h /www &
tmux new-session -d -s b9s /usr/local/bin/b9s

ttyd --writable -p 7681 \
  -t disableLeaveAlert=true \
  -t 'theme={"background":"#18181b"}' \
  tmux attach-session -t b9s &

exec ttyd --writable -p 7683 \
  --base-path /b9s/terminal \
  -t disableLeaveAlert=true \
  -t 'theme={"background":"#18181b"}' \
  tmux attach-session -t b9s
