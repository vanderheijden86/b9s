# b9s Makefile
#
# Build with SQLite FTS5 (full-text search) support enabled

.PHONY: build install clean test web web-dev web-types web-e2e book

# Enable FTS5 for full-text search in SQLite exports
export CGO_CFLAGS := -DSQLITE_ENABLE_FTS5

build:
	go build -o b9s ./cmd/b9s

install:
	go install ./cmd/b9s

clean:
	rm -f b9s
	go clean

test:
	go test ./...

# The web UI (b9s web). pkg/web/dist is committed, so only a change under
# web/ needs Node; TestEmbeddedBundleIsCurrent fails when dist is stale.
web:
	npm --prefix web ci
	npm --prefix web run typecheck
	npm --prefix web run build

# Live preview of the web UI: esbuild rebuilds into .b9s/web-dev on every
# save under web/, and b9s web serves that folder and reloads open browsers.
# pkg/web/dist is untouched; run `make web` before committing. Pass server
# flags with WEB_DEV_ARGS, for example WEB_DEV_ARGS="--no-token".
WEB_DEV_DIR := $(CURDIR)/.b9s/web-dev
web-dev:
	@test -d web/node_modules || npm --prefix web ci
	mkdir -p $(WEB_DEV_DIR)
	node web/build.mjs --watch --out $(WEB_DEV_DIR) & trap "kill $$!" EXIT INT TERM; \
	go run ./cmd/b9s web --dev-assets $(WEB_DEV_DIR) $(WEB_DEV_ARGS)

# Regenerates web/src/api.gen.ts from the Go API types.
web-types:
	B9S_UPDATE_TS=1 go test ./pkg/web -run TestGeneratedTypesAreCurrent

# Browser tests: a real b9s web per test, a fake bd, Chromium and WebKit.
web-e2e:
	npm --prefix web run test:e2e

# The book on the architecture (docs/book). Needs pandoc; the EPUB is committed.
book:
	cd docs/book && pandoc 00-metadata.yaml [0-9][0-9]-*.md --from markdown --to epub3 \
	  --toc --toc-depth=1 --number-sections --css=style.css -o how-b9s-works.epub
