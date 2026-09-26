#!/bin/sh
# The program inside the tmux session. It reads the password itself so the
# password never appears in a command line.
set -eu

home=/data/home
export HOME="$home"
export TERM=xterm-256color
export BEADS_DOLT_PASSWORD="$(cat "${B9S_WEB_SECRETS:-/etc/b9s-web/dolt}/password")"
export B9S_TRUSTED_DOLT_ENDPOINTS="$B9S_DOLT_HOST:$B9S_DOLT_PORT"
export BD_ACTOR="$B9S_WEB_ACTOR"
export BD_DISABLE_METRICS=1
# b9s reads the port from metadata.json. bd warns about that field unless a
# source it ranks higher names the port.
export BEADS_DOLT_SERVER_PORT="$B9S_DOLT_PORT"
export BEADS_DIR="$home/projects/$(cat "$home/first-project")/.beads"
unset EDITOR VISUAL TTYD_USER

cd "$home"
exec /usr/local/bin/b9s
