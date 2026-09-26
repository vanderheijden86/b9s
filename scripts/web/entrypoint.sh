#!/bin/sh
# Starts one person's b9s web board: a tmux server, a version endpoint and
# ttyd. session.sh checks each connection against the owner.
set -eu

: "${B9S_DOLT_HOST:?required}"
: "${B9S_DOLT_PORT:?required}"
: "${B9S_DOLT_USER:?required}"
: "${B9S_WEB_OWNER_EMAIL:?required}"
: "${B9S_WEB_ACTOR:?required}"
: "${B9S_WEB_SECRETS:=/etc/b9s-web/dolt}"
: "${B9S_WEB_AUTH_HEADER:=X-Forwarded-Email}"

case "$B9S_DOLT_HOST" in
  *[!A-Za-z0-9._:-]*|'') printf 'invalid B9S_DOLT_HOST\n' >&2; exit 1 ;;
esac
case "$B9S_DOLT_PORT" in
  *[!0-9]*|'') printf 'invalid B9S_DOLT_PORT\n' >&2; exit 1 ;;
esac
case "$B9S_WEB_AUTH_HEADER" in
  *[!A-Za-z0-9-]*|'') printf 'invalid B9S_WEB_AUTH_HEADER\n' >&2; exit 1 ;;
esac
case "$B9S_DOLT_USER" in
  *[!A-Za-z0-9_]*) printf 'invalid B9S_DOLT_USER\n' >&2; exit 1 ;;
esac
case "$B9S_WEB_OWNER_EMAIL" in
  *[!a-z0-9@._+-]*) printf 'B9S_WEB_OWNER_EMAIL must be a lowercase email\n' >&2; exit 1 ;;
  *@*) ;;
  *) printf 'B9S_WEB_OWNER_EMAIL must be a lowercase email\n' >&2; exit 1 ;;
esac
case "$B9S_WEB_ACTOR" in
  *[!A-Za-z0-9@._+-]*) printf 'invalid B9S_WEB_ACTOR\n' >&2; exit 1 ;;
esac
[ -r "$B9S_WEB_SECRETS/password" ] || { printf '%s/password is not readable\n' "$B9S_WEB_SECRETS" >&2; exit 1; }

export HOME=/tmp
export TERM=xterm-256color
mkdir -p /data/home /www

printf '{"commit":"%s"}\n' "$(sed -n '1p' /app/.commit-sha)" > /www/__version

tmux start-server

httpd -f -p 7682 -h /www &

# --auth-header makes ttyd refuse any connection without the header and pass
# its value to the command as TTYD_USER. The header is only trustworthy
# because the NetworkPolicy admits nothing but the signing-in proxy.
# --check-origin refuses a websocket opened by another site's page.
exec ttyd --writable -p 7681 \
  --auth-header "$B9S_WEB_AUTH_HEADER" \
  --check-origin \
  -t disableLeaveAlert=true \
  -t titleFixed=b9s \
  -t 'theme={"background":"#18181b"}' \
  /usr/local/bin/b9s-web-session
