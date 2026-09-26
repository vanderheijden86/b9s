#!/usr/bin/env bash
# Builds the web image and checks one person's board against a throwaway Dolt
# server: which projects it shows, that it writes as the owner's Dolt user and
# bd actor, and that b9s web refuses every email but the owner's.
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

# status EMAIL PATH prints the HTTP status b9s web answers, sending EMAIL in
# the login proxy's header unless it is empty.
status() {
    local hdr=''
    [ -z "$1" ] || hdr="--header 'X-Forwarded-Email: $1'"
    docker exec "$RUN_ID-web" sh -c "wget -S -O /dev/null $hdr 'http://127.0.0.1:7681$2' 2>&1 | sed -n 's/^ *HTTP\/[0-9.]* \([0-9]*\).*/\1/p' | tail -1"
}
# as_owner PATH [POST-BODY CSRF] prints the body b9s web returns to the owner.
as_owner() {
    if [ $# -eq 1 ]; then
        docker exec "$RUN_ID-web" wget -qO- --header 'X-Forwarded-Email: Alice@Example.com' "http://127.0.0.1:7681$1"
    else
        docker exec "$RUN_ID-web" wget -qO- --header 'X-Forwarded-Email: Alice@Example.com' \
            --header "X-B9s-CSRF: $3" --header 'Content-Type: application/json' \
            --post-data "$2" "http://127.0.0.1:7681$1"
    fi
}

deadline=$((SECONDS + 60))
until [ "$(status '' /api/health)" = 401 ]; do
    [ "$SECONDS" -lt "$deadline" ] || { docker logs "$RUN_ID-web"; echo "web did not start in 60s"; exit 1; }
    sleep 1
done

version=$(docker exec "$RUN_ID-web" wget -qO- http://127.0.0.1:7682/__version)
case "$version" in *'"commit":"test-sha"'*) ok "version endpoint names the commit" ;; *) bad "version: $version" ;; esac

for email in '' mallory@example.com alice@example.com.evil; do
    code=$(status "$email" /api/snapshot)
    [ "$code" = 401 ] && ok "refused '$email' with 401" || bad "'$email' got $code, want 401"
done
code=$(status alice@example.com /api/snapshot)
[ "$code" = 200 ] && ok "the owner is served" || bad "the owner got $code"
code=$(status '' '/pair?t=anything')
[ "$code" = 403 ] && ok "a pairing link without the header is refused" || bad "pair without header: $code"

snapshot=$(as_owner /api/snapshot)
case "$snapshot" in *'seeded in alpha_proj'*) ok "the owner sees their first project's issues, email matched without case" ;; *) bad "snapshot: ${snapshot:0:200}" ;; esac

projects=$(as_owner /api/projects)
case "$projects" in *'"name":"alpha_proj"'*'"name":"shared_proj"'*) ok "the project sheet lists alpha_proj and shared_proj" ;; *) bad "projects: ${projects:0:300}" ;; esac
case "$projects" in *beta_proj*) bad "a project without a grant is listed" ;; *) ok "a project without a grant is not listed" ;; esac
case "$projects" in *not_beads*) bad "a database without issues became a project" ;; *) ok "a database without an issues table is not a project" ;; esac

env_of_b9s=$(docker exec "$RUN_ID-web" sh -c "tr '\0' '\n' < /proc/1/environ")
case "$env_of_b9s" in *BEADS_DOLT_PASSWORD=alice-pw*) ok "b9s holds the owner's password" ;; *) bad "b9s lacks the password" ;; esac
case "$env_of_b9s" in *BD_ACTOR=alice-actor*) ok "b9s writes as the owner's bd actor" ;; *) bad "actor missing" ;; esac
case "$env_of_b9s" in *BEADS_DOLT_SERVER_PORT=3306*) ok "bd gets the port without the metadata.json warning" ;; *) bad "BEADS_DOLT_SERVER_PORT missing" ;; esac
case "$env_of_b9s" in *BEADS_DIR=*) bad "BEADS_DIR would send every write to one project" ;; *) ok "BEADS_DIR is unset, so writes follow the open project" ;; esac

args=$(docker exec "$RUN_ID-web" sh -c 'cat /proc/*/cmdline 2>/dev/null | tr "\0" " "')
case "$args" in *alice-pw*) bad "the password appears in a command line" ;; *) ok "no password in any command line" ;; esac

# A write goes through the API the way the browser sends it.
csrf=$(as_owner /api/session | sed -n 's/.*"csrf":"\([^"]*\)".*/\1/p')
[ -n "$csrf" ] && ok "the session hands the owner a CSRF value" || bad "no CSRF value"
code=$(docker exec "$RUN_ID-web" sh -c "wget -S -O /dev/null --header 'X-Forwarded-Email: alice@example.com' --header 'Content-Type: application/json' --post-data '{\"op\":\"create\",\"fields\":{\"title\":\"no csrf\"}}' http://127.0.0.1:7681/api/write 2>&1 | sed -n 's/^ *HTTP\/[0-9.]* \([0-9]*\).*/\1/p' | tail -1")
[ "$code" = 403 ] && ok "a write without the CSRF header is refused" || bad "write without CSRF: $code"
result=$(as_owner /api/write '{"op":"create","fields":{"title":"written from the web"}}' "$csrf" 2>&1 || true)
case "$result" in *'"ok":true'*) ok "the API creates an issue in alpha_proj" ;; *) bad "create: $result" ;; esac
who=$(sql "SELECT created_by FROM alpha_proj.issues WHERE title = 'written from the web';" | tail -1)
[ "$who" = alice-actor ] && ok "the new issue's creator is the owner's actor" || bad "creator is '$who'"
end=$((SECONDS + 20))
seen=no
while [ "$SECONDS" -lt "$end" ]; do
    case "$(as_owner /api/snapshot)" in *'written from the web'*) seen=yes; break ;; esac
    sleep 1
done
[ "$seen" = yes ] && ok "the snapshot shows the new issue without a restart" || bad "the snapshot did not show the new issue"

denied=$(docker exec "$RUN_ID-web" sh -c "MYSQL_PWD=alice-pw mariadb --protocol=TCP -h $RUN_ID-dolt -P 3306 -u alice -e 'INSERT INTO beta_proj.issues (id) VALUES (\"x\")' 2>&1" || true)
case "$denied" in *denied*) ok "the owner's user cannot write a project it has no grant on" ;; *) bad "write to beta_proj: $denied" ;; esac

docker rmi "$SEED_IMAGE" >/dev/null 2>&1 || true
echo "=== B9S WEB DONE pass=$pass fail=$fail ==="
[ "$fail" -eq 0 ]
