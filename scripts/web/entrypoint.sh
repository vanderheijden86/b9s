#!/bin/sh
# Starts one person's b9s web board: a version endpoint, their Beads projects
# and b9s web. b9s web serves only requests the login proxy signed in as the
# owner; the header is trustworthy because the NetworkPolicy admits nothing
# but that proxy.
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
password_file="$B9S_WEB_SECRETS/password"
[ -r "$password_file" ] || { printf '%s is not readable\n' "$password_file" >&2; exit 1; }

mkdir -p /www
printf '{"commit":"%s"}\n' "$(sed -n '1p' /app/.commit-sha)" > /www/__version
httpd -f -p 7682 -h /www &

# The home is rebuilt on every start, so a database the user gained or lost
# since the last start shows up or disappears.
home=/data/home
rm -rf "$home"
mkdir -p "$home/.config/b9s" "$home/projects"
# bd picks the planning repo by this role and warns on every run without it.
printf '[beads]\n\trole = maintainer\n' > "$home/.gitconfig"

sql() {
  MYSQL_PWD=$(cat "$password_file") mariadb \
    --protocol=TCP --host="$B9S_DOLT_HOST" --port="$B9S_DOLT_PORT" \
    --user="$B9S_DOLT_USER" --batch --skip-column-names --raw --execute="$1"
}

sql 'SHOW DATABASES' > "$home/databases" \
  || { printf 'the database refused the credential for %s\n' "$B9S_DOLT_USER" >&2; exit 1; }

# b9s web lists every checkout under projects/ (--projects-root). The config
# file only keeps the recent projects the owner opens.
: > "$home/.config/b9s/config.yaml"
first=''
count=0
while IFS= read -r database; do
  case "$database" in
    information_schema|mysql|performance_schema|sys|dolt_cluster|'') continue ;;
    *[!A-Za-z0-9_-]*) continue ;;
  esac
  # A database without an issues table is not a Beads project.
  sql "SELECT 1 FROM \`$database\`.issues LIMIT 0" >/dev/null 2>&1 || continue
  dir="$home/projects/$database"
  mkdir -p -m 0700 "$dir/.beads"
  printf '{"database":"dolt","backend":"dolt","dolt_mode":"server","dolt_server_host":"%s","dolt_server_port":%s,"dolt_server_user":"%s","dolt_database":"%s"}\n' \
    "$B9S_DOLT_HOST" "$B9S_DOLT_PORT" "$B9S_DOLT_USER" "$database" > "$dir/.beads/metadata.json"
  [ -n "$first" ] || first=$database
  count=$((count + 1))
done < "$home/databases"
[ "$count" -gt 0 ] || { printf 'no Beads project is visible to %s\n' "$B9S_DOLT_USER" >&2; exit 1; }
printf 'b9s web: %d projects, starting in %s\n' "$count" "$first"

export HOME="$home"
# b9s and bd read the password from the environment, so it never appears in
# a command line.
BEADS_DOLT_PASSWORD="$(cat "$password_file")"
export BEADS_DOLT_PASSWORD
export B9S_TRUSTED_DOLT_ENDPOINTS="$B9S_DOLT_HOST:$B9S_DOLT_PORT"
export BD_ACTOR="$B9S_WEB_ACTOR"
export BD_DISABLE_METRICS=1
# b9s reads the port from metadata.json. bd warns about that field unless a
# source it ranks higher names the port.
export BEADS_DOLT_SERVER_PORT="$B9S_DOLT_PORT"
unset EDITOR VISUAL

cd "$home/projects/$first"
exec /usr/local/bin/b9s web --listen 0.0.0.0:7681 \
  --trust-header "$B9S_WEB_AUTH_HEADER" --owner "$B9S_WEB_OWNER_EMAIL" \
  --projects-root "$home/projects"
