# b9s web image

One person's b9s board in a browser: `b9s web` serves the phone and desktop
web UI for their projects on the shared Dolt server, and `bd` inside the image
carries out every write. It runs behind a proxy that signs the person in and
trusts that proxy's email header (ADR 0021); the image itself has no login.

A deployment runs one such pod per person, for example at
`<person>.beads.example.com`; its manifests live in the deployment repository.

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
| `B9S_WEB_CONFIG_FILE` | Optional read-only b9s config file to copy into the generated home on each start |
| `B9S_ATTACHMENTS_S3_ACCESS_KEY_ID`, `B9S_ATTACHMENTS_S3_SECRET_ACCESS_KEY` | Attachment credentials, supplied to the b9s container from a Secret and inherited by b9s |
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

## Attachment settings

Mount a config file into the b9s container and name it with
`B9S_WEB_CONFIG_FILE`, for example `/etc/b9s-web/config/config.yaml`:

```yaml
attachments:
  backend: s3
  prefix: workspace
  s3:
    endpoint: https://s3.example.com
    region: eu-central-1
    bucket: workspace-attachments
    path_style: true
    url_ttl: 15m
```

Supply `B9S_ATTACHMENTS_S3_ACCESS_KEY_ID` and
`B9S_ATTACHMENTS_S3_SECRET_ACCESS_KEY` as environment variables on the b9s
container, not the login proxy. The entrypoint preserves them when it starts
`b9s web` with the proxy's trusted email header. Secrets never belong in YAML,
proxy headers or command arguments. A configured file that is missing or
unreadable fails startup.

The entrypoint copies the mounted file into its writable generated home because
b9s saves recent projects there. The mount is unchanged. Without a config file,
startup uses the default settings.

## Build and test

```bash
docker build -f scripts/web/Dockerfile --target runtime \
  --build-arg COMMIT_SHA=$(git rev-parse HEAD) -t b9s-web:$(git rev-parse HEAD) .
scripts/web/test-web.sh
```

The startup regression needs no Docker or SQL server:

```bash
go test ./scripts/web -run '^TestWebEntrypointAttachmentSettings$' -count=1 -timeout=30s -v
```

It runs the entrypoint with container paths mapped into a temporary directory
and stubbed SQL, version server and b9s commands. It checks config seeding,
credential inheritance, proxy arguments and refusal of a missing config file.
It does not build the image or test proxy traffic.

`test-web.sh` seeds three real Beads databases on a throwaway Dolt server,
starts the image as one owner, and checks refusals, the project list, a write
through the API with the owner's actor, and that the snapshot shows it. It ends
with `=== B9S WEB DONE pass=N fail=M ===`. Set `B9S_WEB_TEST_KEEP=1` to keep
the containers after a failure.

The bd version is `BD_VERSION` in the Dockerfile. It is built from source
because the published Linux binary needs glibc and the image is Alpine.
