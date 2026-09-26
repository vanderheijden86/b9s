#!/usr/bin/env bash
# Builds the web image and checks one person's board against a throwaway Dolt
# server: which projects it shows, that it writes as the owner's Dolt user and
# bd actor, and that it refuses every other signed-in email.
#
#   scripts/web/test-web.sh            build from the working tree, then test
#   B9S_WEB_IMAGE=<tag> scripts/web/test-web.sh   test an image already built
#
# Needs docker. Removes its containers and network on every exit path.
set -euo pipefail

REPO=$(cd "$(dirname "$0")/../.." && pwd)
RUN_ID="b9s-web-test-$$"
DOLT_IMAGE=dolthub/dolt-sql-server:latest
ROOT_PW=root-test-pw
WORK=$(mktemp -d)

pass=0
fail=0
ok()  { printf '  ok   %s\n' "$1"; pass=$((pass + 1)); }
bad() { printf '  FAIL %s\n' "$1"; fail=$((fail + 1)); }

cleanup() {
    [ -z "${B9S_WEB_TEST_KEEP:-}" ] || { echo "kept $RUN_ID"; return; }
    docker rm -f "$RUN_ID-dolt" "$RUN_ID-web" >/dev/null 2>&1 || true
    docker network rm "$RUN_ID" >/dev/null 2>&1 || true
    rm -rf "$WORK"
}
trap cleanup EXIT

IMAGE=${B9S_WEB_IMAGE:-}
if [ -z "$IMAGE" ]; then
    IMAGE="b9s-web:test-$$"
    docker build -q -f "$REPO/scripts/web/Dockerfile" --target runtime \
        --build-arg COMMIT_SHA=test-sha -t "$IMAGE" "$REPO" >/dev/null
fi
# The bd build stage has git, which bd init needs to seed the projects.
SEED_IMAGE="b9s-web-bd:test-$$"
docker build -q -f "$REPO/scripts/web/Dockerfile" --target bd -t "$SEED_IMAGE" "$REPO" >/dev/null

docker network create "$RUN_ID" >/dev/null
docker run -d --name "$RUN_ID-dolt" --network "$RUN_ID" \
    -e DOLT_ROOT_PASSWORD="$ROOT_PW" -e DOLT_ROOT_HOST=% "$DOLT_IMAGE" >/dev/null

sql() {
    docker run --rm --network "$RUN_ID" --entrypoint dolt "$DOLT_IMAGE" \
        --host "$RUN_ID-dolt" --port 3306 --no-tls \
        --user root --password "$ROOT_PW" --use-db '' sql -q "$1" -r csv
}
deadline=$((SECONDS + 90))
until sql 'SELECT 1;' >/dev/null 2>&1; do
    [ "$SECONDS" -lt "$deadline" ] || { echo "dolt did not start in 90s"; exit 1; }
    sleep 2
done

for project in alpha_proj beta_proj shared_proj; do
    docker run --rm --network "$RUN_ID" --entrypoint sh \
        -e BEADS_DOLT_PASSWORD="$ROOT_PW" -e BD_ACTOR=seed "$SEED_IMAGE" -c "
        git config --global user.email seed@example.com && git config --global user.name seed &&
        mkdir /p && cd /p && git init -q &&
        /out-bd init --server --external --server-host=$RUN_ID-dolt --server-port=3306 \
            --server-user=root --database=$project --prefix=${project%_proj} \
            --quiet --skip-hooks --skip-agents --non-interactive >/dev/null 2>&1 &&
        /out-bd create --title 'seeded in $project' --silent >/dev/null 2>&1" \
        || { echo "could not seed $project"; exit 1; }
done
# alice owns this board. beta_proj is someone else's, not_beads is no project.
sql "CREATE DATABASE not_beads; CREATE TABLE not_beads.things (id INT PRIMARY KEY);
  CREATE USER 'alice'@'%' IDENTIFIED BY 'alice-pw';
  GRANT ALL PRIVILEGES ON alpha_proj.* TO 'alice'@'%';
  GRANT ALL PRIVILEGES ON shared_proj.* TO 'alice'@'%';
  GRANT ALL PRIVILEGES ON not_beads.* TO 'alice'@'%';" >/dev/null

mkdir -p "$WORK/dolt"
printf 'alice-pw' > "$WORK/dolt/password"
chmod -R a+rX "$WORK"

docker run -d --name "$RUN_ID-web" --network "$RUN_ID" --user 65532:65532 \
    -e B9S_DOLT_HOST="$RUN_ID-dolt" -e B9S_DOLT_PORT=3306 -e B9S_DOLT_USER=alice \
    -e B9S_WEB_OWNER_EMAIL=alice@example.com -e B9S_WEB_ACTOR=alice-actor -e LANG=C.UTF-8 \
    -v "$WORK/dolt:/etc/b9s-web/dolt:ro" \
    "$IMAGE" >/dev/null

deadline=$((SECONDS + 30))
until docker exec "$RUN_ID-web" wget -qO- http://127.0.0.1:7682/__version >/dev/null 2>&1; do
    [ "$SECONDS" -lt "$deadline" ] || { docker logs "$RUN_ID-web"; echo "web did not start in 30s"; exit 1; }
    sleep 1
done

version=$(docker exec "$RUN_ID-web" wget -qO- http://127.0.0.1:7682/__version)
case "$version" in *'"commit":"test-sha"'*) ok "version endpoint names the commit" ;; *) bad "version: $version" ;; esac

code=$(docker exec "$RUN_ID-web" sh -c 'wget -S -O /dev/null http://127.0.0.1:7681/ 2>&1 | sed -n "s/^ *HTTP\/[0-9.]* \([0-9]*\).*/\1/p" | tail -1')
[ "$code" = 407 ] && ok "ttyd answers 407 without the auth header" || bad "ttyd answered '$code' without the auth header"

refused() {
    local email=$1 want=$2 out
    out=$(docker exec -t -e B9S_WEB_REFUSE_HOLD=0 -e TTYD_USER="$email" "$RUN_ID-web" /usr/local/bin/b9s-web-session 2>&1 || true)
    case "$out" in *"$want"*) ok "refused '$email'" ;; *) bad "for '$email' expected '$want', got: $out" ;; esac
}
refused mallory@example.com 'has no access to this board'
refused '' 'No signed-in user'
refused 'alice@example.com.evil' 'has no access to this board'
if docker exec "$RUN_ID-web" tmux has-session -t '=b9s' 2>/dev/null; then bad "a refused email started a session"; else ok "a refused email starts no session"; fi

# The owner's session: started detached, the way a browser tab starts it.
docker exec -d -t -e TTYD_USER=Alice@Example.com "$RUN_ID-web" /usr/local/bin/b9s-web-session
started=no
end=$((SECONDS + 30))
while [ "$SECONDS" -lt "$end" ]; do
    if docker exec "$RUN_ID-web" tmux capture-pane -p -t '=b9s:' 2>/dev/null | grep -q 'seeded in'; then started=yes; break; fi
    sleep 1
done
[ "$started" = yes ] && ok "the owner's session shows their issues, email matched without case" \
    || bad "no b9s screen for the owner: $(docker exec "$RUN_ID-web" tmux capture-pane -p -t '=b9s:' 2>&1 | head -5)"

cfg=$(docker exec "$RUN_ID-web" cat /data/home/.config/b9s/config.yaml)
case "$cfg" in *alpha_proj*shared_proj*|*shared_proj*alpha_proj*) ok "projects are alpha_proj and shared_proj" ;; *) bad "config: $cfg" ;; esac
case "$cfg" in *beta_proj*) bad "a project without a grant is listed" ;; *) ok "a project without a grant is not listed" ;; esac
case "$cfg" in *not_beads*) bad "a database without issues became a project" ;; *) ok "a database without an issues table is not a project" ;; esac

pid=$(docker exec "$RUN_ID-web" tmux display-message -p -t '=b9s:' '#{pane_pid}')
env_of_b9s=$(docker exec "$RUN_ID-web" sh -c "tr '\0' '\n' < /proc/$pid/environ")
case "$env_of_b9s" in *BEADS_DOLT_PASSWORD=alice-pw*) ok "b9s holds the owner's password" ;; *) bad "b9s lacks the password" ;; esac
case "$env_of_b9s" in *BD_ACTOR=alice-actor*) ok "b9s writes as the owner's bd actor" ;; *) bad "actor missing" ;; esac
case "$env_of_b9s" in *BEADS_DOLT_SERVER_PORT=3306*) ok "bd gets the port without the metadata.json warning" ;; *) bad "BEADS_DOLT_SERVER_PORT missing" ;; esac
case "$env_of_b9s" in *TTYD_USER*) bad "TTYD_USER reached b9s" ;; *) ok "TTYD_USER does not reach b9s" ;; esac

args=$(docker exec "$RUN_ID-web" sh -c 'cat /proc/*/cmdline 2>/dev/null | tr "\0" " "')
case "$args" in *alice-pw*) bad "the password appears in a command line" ;; *) ok "no password in any command line" ;; esac

# A write runs bd the way b9s runs it: from a project checkout, with the
# session's environment.
bd_as_b9s() {
    local project=$1; shift
    docker exec -w "/data/home/projects/$project" \
        -e HOME=/data/home -e BEADS_DOLT_PASSWORD=alice-pw -e BD_ACTOR=alice-actor -e BD_DISABLE_METRICS=1 -e BEADS_DOLT_SERVER_PORT=3306 \
        -e B9S_TRUSTED_DOLT_ENDPOINTS="$RUN_ID-dolt:3306" \
        "$RUN_ID-web" bd "$@"
}
if created=$(bd_as_b9s alpha_proj create --title 'written from the web' --silent 2>&1); then
    ok "bd creates an issue in alpha_proj"
    # b9s shows bd's stderr after a write, so a warning here is noise on screen.
    [ "$(printf '%s\n' "$created" | wc -l)" -eq 1 ] && ok "bd prints only the new id" || bad "bd printed more than the id: $created"
    who=$(sql "SELECT created_by FROM alpha_proj.issues WHERE title = 'written from the web';" | tail -1)
    [ "$who" = alice-actor ] && ok "the new issue's creator is the owner's actor" || bad "creator is '$who'"
else
    bad "bd create failed in alpha_proj: $created"
fi
end=$((SECONDS + 20))
seen=no
while [ "$SECONDS" -lt "$end" ]; do
    if docker exec "$RUN_ID-web" tmux capture-pane -p -t '=b9s:' | grep -q 'written from the web'; then seen=yes; break; fi
    sleep 1
done
[ "$seen" = yes ] && ok "b9s shows the new issue without a restart" || bad "b9s did not show the new issue"

denied=$(docker exec "$RUN_ID-web" sh -c "MYSQL_PWD=alice-pw mariadb --protocol=TCP -h $RUN_ID-dolt -P 3306 -u alice -e 'INSERT INTO beta_proj.issues (id) VALUES (\"x\")' 2>&1" || true)
case "$denied" in *denied*) ok "the owner's user cannot write a project it has no grant on" ;; *) bad "write to beta_proj: $denied" ;; esac

binds=$(docker exec "$RUN_ID-web" sh -c 'tmux list-keys 2>/dev/null | wc -l')
[ "$binds" = 0 ] && ok "tmux has no key bindings a browser could reach" || bad "tmux still has $binds key bindings"

docker rmi "$SEED_IMAGE" >/dev/null 2>&1 || true
echo "=== B9S WEB DONE pass=$pass fail=$fail ==="
[ "$fail" -eq 0 ]
