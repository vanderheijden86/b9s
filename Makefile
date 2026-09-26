# b9s Makefile
#
# Build with SQLite FTS5 (full-text search) support enabled

.PHONY: build install clean test web web-types web-e2e book

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
