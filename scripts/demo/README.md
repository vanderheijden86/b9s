# b9s public demo image

The board behind `demo.b9s.osen.co`: `b9s web --public` (ADR 0029) serving
one sample project that anyone can read and change. The project lives in an
embedded Dolt database inside the pod, seeded from a JSONL file baked into the
image, and is reseeded on every half hour (:00 and :30).

```
 visitor ──▶ Traefik (TLS, per-IP limit) ──▶ b9s web --public :7979 ──▶ bd
                                                 ▲                       │
                              entrypoint loop ───┘   /data/<name> (embedded Dolt)
                              stop, reseed, start       reseeded from /app/seed
```

Build from the repository root:

```bash
docker build --platform linux/amd64 -f scripts/demo/Dockerfile -t b9s-demo:latest .
docker run --rm -p 7979:7979 b9s-demo:latest   # http://localhost:7979
```

`--build-arg SEED=examples/sample-project/.beads/issues.jsonl` bakes another
sample in; set `B9S_DEMO_PREFIX` and `B9S_DEMO_NAME` to match it.

## Contract

| Setting | Default | Meaning |
|---|---|---|
| `B9S_DEMO_SEED` | `/app/seed/issues.jsonl` | The JSONL every reset starts from |
| `B9S_DEMO_PREFIX` | `mars` | Issue prefix of the seed |
| `B9S_DEMO_NAME` | `space-programme` | Project name shown in the header |
| `B9S_DEMO_RESET_SECONDS` | `1800` | Reset period; resets land on multiples of it |
| `B9S_DEMO_BANNER` | `Live demo · edits reset every 30 min` | The line above the board |
| `B9S_DEMO_BANNER_LINK` | `https://github.com/vanderheijden86/b9s` | Where the banner links |
| `/data` | | Writable; everything else can be read-only |
| `:7979` | | `b9s web --public` |

## What the deployment must add

The server trusts nobody and limits only its own totals, so the deployment
carries the rest:

- a per-IP rate limit at the proxy, because every request reaches b9s from it;
- a NetworkPolicy denying all egress, because `bd` runs on visitor input;
- a size limit on `/data`, and CPU and memory limits.

The small-hetzner manifests are in the osenco repository, `deploy/base/b9s-demo/`.
