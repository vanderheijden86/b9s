# b9s web image

One person's b9s board in a browser: `b9s web` serves the phone and desktop
web UI for their projects on the shared Dolt server, and `bd` inside the image
carries out every write. It runs behind a proxy that signs the person in and
trusts that proxy's email header (ADR 0021); the image itself has no login.

The deployment that uses it is `beads.osen.co` on small-hetzner, described in
osenco-infra ADR 0004 and `clusters/small-hetzner/platform/beads-web/`.

```
 browser ──▶ oauth2-proxy ──X-Forwarded-Email──▶ b9s web :7681 ──▶ bd
                                                   │  --trust-header, --owner
                                                   ▼
                                           Dolt as B9S_DOLT_USER
```

## Contract

| Setting | Meaning |
|---|---|
| `B9S_DOLT_HOST`, `B9S_DOLT_PORT` | The Dolt SQL server |
| `B9S_DOLT_USER` | The owner's own Dolt user, the one their laptop would use |
| `B9S_WEB_OWNER_EMAIL` | The one signed-in email the board accepts, in lowercase |
| `B9S_WEB_ACTOR` | The bd actor written as creator and updater |
| `B9S_WEB_AUTH_HEADER` | Header carrying the email, default `X-Forwarded-Email` |
| `/etc/b9s-web/dolt/password` | The owner's Dolt password, mounted from a Secret |
| `/data`, `/www`, `/tmp` | Writable; the root filesystem can be read-only |
| `:7681` | `b9s web`, reachable only from the proxy |
| `:7682/__version` | `{"commit":"<sha>"}` for checking which build is live |

## Behaviour

- `b9s web` answers 401 to any request whose header is missing or names
  someone other than `B9S_WEB_OWNER_EMAIL`, compared without case. The header
  is only trustworthy when nothing but the proxy can reach port 7681, which is
  the deployment's NetworkPolicy's job. Writes also need the CSRF value the
  session hands out.
- On start, `entrypoint.sh` lists the databases the Dolt user can see, keeps
  those with an `issues` table, and writes a `.beads/metadata.json` per project
  plus the b9s project list. A database granted later appears after a restart.
- `BEADS_DIR` is left unset, so each write runs `bd` in the open project's
  folder rather than always in the first one.
- The password reaches b9s and bd through the environment, never a command line.

## Build and test

```bash
docker build -f scripts/web/Dockerfile --target runtime \
  --build-arg COMMIT_SHA=$(git rev-parse HEAD) -t b9s-web:$(git rev-parse HEAD) .
scripts/web/test-web.sh
```

`test-web.sh` seeds three real Beads databases on a throwaway Dolt server,
starts the image as one owner, and checks refusals, the project list, a write
through the API with the owner's actor, and that the snapshot shows it. It ends
with `=== B9S WEB DONE pass=N fail=M ===`. Set `B9S_WEB_TEST_KEEP=1` to keep
the containers after a failure.

The bd version is `BD_VERSION` in the Dockerfile. It is built from source
because the published Linux binary needs glibc and the image is Alpine.
