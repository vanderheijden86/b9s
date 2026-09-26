#!/bin/sh
# Runs once per browser connection, as ttyd's command. The pod belongs to one
# person: it refuses any signed-in email but theirs, prepares their b9s
# projects on first use, and attaches every tab to their one tmux session.
set -eu

SECRETS=${B9S_WEB_SECRETS:-/etc/b9s-web/dolt}
session=b9s

refuse() {
  printf '\r\n  %s\r\n\r\n' "$1"
  # Hold the message on screen; ttyd closes the page when the command exits.
  sleep "${B9S_WEB_REFUSE_HOLD:-600}"
  exit 1
}

email=$(printf '%s' "${TTYD_USER:-}" | tr 'A-Z' 'a-z')
[ -n "$email" ] || refuse 'No signed-in user reached this terminal.'
# The proxy in front admits one email. Checking it again here means a
# misrouted request or a proxy misconfiguration shows a refusal, not a board.
[ "$email" = "$B9S_WEB_OWNER_EMAIL" ] || refuse "$email has no access to this board."

password_file="$SECRETS/password"
[ -r "$password_file" ] || refuse 'No database credential is mounted.'

# Two tabs opening at once must not both build the home and start the session.
mkdir -p /data/locks
exec 9>/data/locks/session
flock 9
if ! tmux has-session -t "=$session" 2>/dev/null; then
  home=/data/home
  rm -rf "$home"
  mkdir -p "$home/.config/b9s" "$home/projects"
  # bd picks the planning repo by this role and warns on every run without it.
  printf '[beads]\n\trole = maintainer\n' > "$home/.gitconfig"

  MYSQL_PWD=$(cat "$password_file") mariadb \
    --protocol=TCP --host="$B9S_DOLT_HOST" --port="$B9S_DOLT_PORT" \
    --user="$B9S_DOLT_USER" --batch --skip-column-names --raw \
    --execute='SHOW DATABASES' > "$home/databases" 2>/dev/null \
    || refuse 'The database refused the credential.'

  printf 'projects:\n' > "$home/.config/b9s/config.yaml"
  first=''
  count=0
  while IFS= read -r database; do
    case "$database" in
      information_schema|mysql|performance_schema|sys|dolt_cluster|'') continue ;;
      *[!A-Za-z0-9_-]*) continue ;;
    esac
    # A database without an issues table is not a Beads project.
    MYSQL_PWD=$(cat "$password_file") mariadb \
      --protocol=TCP --host="$B9S_DOLT_HOST" --port="$B9S_DOLT_PORT" \
      --user="$B9S_DOLT_USER" --batch --skip-column-names --raw \
      --execute="SELECT 1 FROM \`$database\`.issues LIMIT 0" >/dev/null 2>&1 || continue
    dir="$home/projects/$database"
    mkdir -p -m 0700 "$dir/.beads"
    printf '{"database":"dolt","backend":"dolt","dolt_mode":"server","dolt_server_host":"%s","dolt_server_port":%s,"dolt_server_user":"%s","dolt_database":"%s"}\n' \
      "$B9S_DOLT_HOST" "$B9S_DOLT_PORT" "$B9S_DOLT_USER" "$database" > "$dir/.beads/metadata.json"
    printf '  - name: "%s"\n    path: "%s"\n' "$database" "$dir" >> "$home/.config/b9s/config.yaml"
    [ -n "$first" ] || first=$database
    count=$((count + 1))
  done < "$home/databases"

  [ "$count" -gt 0 ] || refuse 'No Beads project is visible to this database user.'
  printf '%s\n' "$first" > "$home/first-project"

  tmux new-session -d -s "$session" -x 200 -y 50 /usr/local/bin/b9s-web-run
fi
flock -u 9

exec tmux attach-session -t "=$session"
