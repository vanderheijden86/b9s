# Into the browser

A terminal UI is a fine thing on a laptop and a poor thing on a phone. The first attempt to fix that, decision records 0005 and 0006, put the terminal itself on the phone: a `ttyd` session in a tmux window, served over Tailscale, with a row of buttons that posted keys into tmux. It worked, in the sense that a screen appeared. It was not something anyone wanted to use on a train.

`b9s web` is the second attempt, and the whole of decision record 0019 is one idea: **serve a small browser app from the same binary, and reuse everything that is not the terminal.** The readers, the watcher, the query parser, the identity registry and `IssueWriter` are all Go packages with no dependency on Bubble Tea. An HTTP server in front of them is a few hundred lines.

```
   phone ──https──▶ tailscale serve ──▶ 127.0.0.1:7979   pkg/web.Server
                                             │
          GET /api/snapshot, /issue, /query, │  Store: one open project,
              /health, /projects, /session   │  version counter, watcher
          GET /api/events  (SSE) ◀───────────┤
          POST /api/write ───────────────────┼──▶ ui.IssueWriter ──▶ bd
          POST /api/projects/open, /reload   │
                                             ▼
                                   Dolt / SQLite / JSONL
```

## The store

`Store` in `pkg/web/store.go` is the web server's counterpart of the TUI model's project state. It holds the open project, its issues, the identity registry, and a version number that goes up by one on every watcher event. The same readers load it and the same watchers refresh it. Where the TUI keeps that state in the Bubble Tea model, the web keeps it behind a mutex, because HTTP handlers run concurrently.

The snapshot the browser fetches is lean: IDs, titles, statuses, priorities, parents, dependency counts, assignees. Descriptions and comments are long and rarely needed, so `/api/issue` returns them for one issue when its detail opens. A project with a thousand issues loads on a phone in one small request.

## Events, then a fetch

`/api/events` is a server-sent event stream. On connect it sends `hello` with the current version. After that it sends `changed` when the version moves, `project` when the open project changes, and `health` when the source does.

The browser does not receive data in an event. It receives a version, compares it with the one it has, and fetches a fresh snapshot when they differ. That design is what makes a missed event harmless: the next event, or a reconnect, carries the current version, and one fetch catches up. There is no incremental patch protocol to get subtly wrong. A phone that slept for an hour reconnects, sees a higher number, and fetches once.

## Writes

`POST /api/write` in `write.go` accepts a closed set of operations, checks each one's fields, and calls the same `IssueWriter` method the TUI would. A failed `bd` answers 422 with `bd`'s output, and the phone shows that text. A write needs the session cookie and the `X-B9s-CSRF` header, so a page on another origin cannot make one.

The browser app shows an undo toast after a write. Undo is another write, the reverse operation, sent through the same endpoint. Nothing is optimistic. The list changes when the event arrives.

## Pairing

The server listens on loopback and reaches the phone through Tailscale Serve, which proxies HTTPS from the tailnet to `127.0.0.1:7979`. That means the server sees every request as coming from loopback, so the peer address proves nothing, and decision record 0020 makes the token do the proving.

**`b9s web` requires pairing on every address.** The pairing token, the session cookie and the CSRF value all derive by HMAC-SHA256 from one secret in `~/.config/b9s/web-secret`, a file created with mode 0600 on first run. A pair link `/pair?t=<token>` sets a cookie that lasts ninety days, and the link keeps working across restarts because the secret does. `--new-token` replaces the secret, which unpairs every browser at once. `--no-token` is accepted only on a loopback address, for development and for the test harness.

Decision record 0021 adds a second mode for a server behind a login proxy. `--trust-header X-Forwarded-Email --owner you@example.com` pairs a request when that header carries the owner's email. In that mode no pairing cookie is issued or accepted, since the proxy's own cookie is the session, and writes still need the CSRF value. The two flags go together, and neither can be combined with `--no-token`.

## Two things the tests keep honest

`web/src/api.gen.ts` is generated from `pkg/web/types.go`. `TestGeneratedTypesAreCurrent` regenerates it in memory and fails when the committed file differs. The TypeScript types cannot drift from the Go ones without a red test.

`pkg/web/dist` is the built bundle, committed and embedded with `go:embed`, so `go install` gives a working `b9s web` with no Node on the machine. `dist/source.sha256` hashes every input, and `TestEmbeddedBundleIsCurrent` fails when `web/src` changed without a `make web`. A stale bundle cannot reach `main`.

## The app

`web/src` is about 2,700 lines of TypeScript with no framework, bundled by esbuild. One module per concern:

| Module | Owns |
|--------|------|
| `api.ts` | fetch and the SSE client |
| `data.ts` | the snapshot and derived fields: lanes, epic, progress |
| `state.ts` | the view state, filters, board settings, persisted in `localStorage` |
| `render.ts` | the DOM: tree, board, wide board, search, detail |
| `actions.ts` | writes, sheets, undo |
| `gestures.ts` | pointer handlers: swipe, long-press, drag, and the keyboard |
| `nav.ts` | browser history |
| `main.ts` | boot and live updates |

The render is a full re-render of the active view from state, like the TUI's `View`. It is fast enough at a thousand issues, and `perf.spec.ts` keeps it that way by loading a thousand and timing a re-render and a swipe deep in the list.

## History without knowing it

The app's views change state in dozens of places: a tap, a swipe, a key, an event, a sheet. None of them know about the browser's history. `nav.ts` derives the place, the view, the detail stack and the graph path, from the state after every render, compares it with the entry the history holds, and does one of three things:

```
   new place equals this      same view, same depth       anything else
   entry's previous place     (cursor move, sibling)      (new view, open,
   (‹, ✕, Esc)                                             relation)
          │                          │                          │
     history.back()             replaceState                pushState
```

The URL carries the view and the top issue, `#/board/bd-12`, so a reload or a shared link opens the same place. The detail stack lives in `history.state`. Back closes the detail, then leaves the board, then, on the first entry, does nothing, because the app makes sure it never sits on the entry that would leave the site. `Backspace` does the same from the keyboard. A `restoring` flag stops the sync from firing during a popstate, and a `backing` timer stops it during the app's own `history.back()`.

The design mirrors the TUI's `Esc`: one key that steps out of whatever you stepped into. On the web that key is Back, and it belongs to the browser, so the app has to make the browser's history mean what `Esc` means.

## Wide screens

On a laptop, a desktop browser or an iPad, the board shows every column side by side, with the same epic rail as the terminal in the first column and `v` switching to epic rows. The detail opens as a side panel that leaves the board in view, `h` `j` `k` `l` and the arrows move the cursor, a mouse drags a card between columns, and `z` folds a column into a rail. The breakpoint is one media query, `(min-width: 720px) and (min-height: 500px)`, and a phone turned sideways does not cross it.

The point of the wide board is not that a browser can do what the terminal does. It is that a person who knows the terminal board knows this one, because the state, the keys and the epic designs are the same decisions, made once.
