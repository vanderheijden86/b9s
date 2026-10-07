#!/bin/sh
# Serves the public b9s demo: one Beads project in an embedded Dolt database,
# seeded from a JSONL file and reseeded on every half hour, so whatever
# visitors write lasts until the next reset. See scripts/demo/README.md.
set -eu

: "${B9S_DEMO_SEED:=/app/seed/issues.jsonl}"
: "${B9S_DEMO_PREFIX:=shop}"
: "${B9S_DEMO_NAME:=webshop}"
: "${B9S_DEMO_RESET_SECONDS:=1800}"
: "${B9S_DEMO_LISTEN:=:7979}"
: "${B9S_DEMO_BANNER:=Live demo · edits reset every 30 min}"
: "${B9S_DEMO_BANNER_LINK:=https://github.com/vanderheijden86/b9s}"

case "$B9S_DEMO_PREFIX$B9S_DEMO_NAME" in
  *[!A-Za-z0-9_-]*|'') printf 'invalid B9S_DEMO_PREFIX or B9S_DEMO_NAME\n' >&2; exit 1 ;;
esac
case "$B9S_DEMO_RESET_SECONDS" in
  *[!0-9]*|''|0) printf 'invalid B9S_DEMO_RESET_SECONDS\n' >&2; exit 1 ;;
esac
[ -r "$B9S_DEMO_SEED" ] || { printf '%s is not readable\n' "$B9S_DEMO_SEED" >&2; exit 1; }

export HOME=/data/home
export BEADS_ACTOR=visitor
mkdir -p "$HOME"
printf '[user]\n\tname = visitor\n\temail = visitor@demo.invalid\n[beads]\n\trole = maintainer\n' > "$HOME/.gitconfig"
project="/data/$B9S_DEMO_NAME"

# seed builds a fresh project beside the live one, so the server is down only
# for the rename and restart.
seed() {
  next="/data/.next"
  rm -rf "$next"
  mkdir -p "$next/.beads"
  cp "$B9S_DEMO_SEED" "$next/.beads/issues.jsonl"
  (cd "$next" && git init -q && bd init --from-jsonl --prefix "$B9S_DEMO_PREFIX" --non-interactive --skip-hooks) \
    >/data/.seed.log 2>&1 || { cat /data/.seed.log >&2; exit 1; }
}

pid=
stop_server() {
  if [ -n "$pid" ]; then
    kill "$pid" 2>/dev/null || true
    wait "$pid" 2>/dev/null || true
    pid=
  fi
}
trap 'stop_server; exit 0' TERM INT

seed
while :; do
  stop_server
  rm -rf "$project"
  mv /data/.next "$project"
  (cd "$project" && exec b9s web --public --listen "$B9S_DEMO_LISTEN" \
    --banner "$B9S_DEMO_BANNER" --banner-link "$B9S_DEMO_BANNER_LINK") &
  pid=$!
  printf '%s reset; next at the next %ss boundary\n' "$(date -u +%H:%M:%S)" "$B9S_DEMO_RESET_SECONDS"
  now=$(date +%s)
  # Resets land on clock boundaries (:00 and :30 for 1800), so the banner's
  # promise holds for every visitor, not only those who came at start-up.
  sleep $((B9S_DEMO_RESET_SECONDS - now % B9S_DEMO_RESET_SECONDS)) &
  wait $! || true
  kill -0 "$pid" 2>/dev/null || { printf 'b9s web exited\n' >&2; exit 1; }
  seed
done
