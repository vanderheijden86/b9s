# syntax=docker/dockerfile:1.7
FROM golang:1.25-alpine@sha256:1ae0735f00daffa3aaf1363a5184c0d2dc55c78e3db4ec70241cdac97bf84b59 AS builder

RUN apk add --no-cache build-base
WORKDIR /src
COPY . .
RUN CGO_ENABLED=1 go build -mod=vendor \
    -ldflags "-s -w" \
    -o /out/b9s ./cmd/b9s

FROM alpine:3.22@sha256:5291449c3df73caf6ed85e649dec1b9e818b39a5d8c871e97afc13e9cd5e8fa8 AS runtime
ARG COMMIT_SHA
ARG WORKTREE_DIRTY
ARG WORKTREE_HASH
RUN apk add --no-cache busybox-extras mariadb-client tmux \
    && test -n "$COMMIT_SHA" \
    && mkdir -p /app /data /www \
    && printf '%s\n' "$COMMIT_SHA" > /app/.commit-sha \
    && chown -R 65532:65532 /data /www
RUN if [ -n "$WORKTREE_DIRTY$WORKTREE_HASH" ]; then \
      case "$WORKTREE_DIRTY" in true|false) ;; *) exit 1 ;; esac; \
      test "${#COMMIT_SHA}" -eq 40 \
      && test "${#WORKTREE_HASH}" -eq 64 \
      && case "$COMMIT_SHA$WORKTREE_HASH" in *[!0-9a-f]*) exit 1 ;; esac \
      && printf '{"commit":"%s","dirty":%s,"tree_hash":"%s"}\n' \
        "$COMMIT_SHA" "$WORKTREE_DIRTY" "$WORKTREE_HASH" > /app/.source.json; \
    fi
LABEL org.opencontainers.image.revision="$COMMIT_SHA" \
      dev.b9s.worktree.dirty="$WORKTREE_DIRTY" \
      dev.b9s.worktree.tree-hash="$WORKTREE_HASH"
COPY --from=builder /out/b9s /usr/local/bin/b9s
COPY scripts/preview/entrypoint.sh /usr/local/bin/b9s-preview-entrypoint
COPY scripts/preview/preview-tui /usr/local/bin/preview-tui
RUN chmod 0755 /usr/local/bin/b9s /usr/local/bin/b9s-preview-entrypoint /usr/local/bin/preview-tui

EXPOSE 7681 7682
ENTRYPOINT ["/usr/local/bin/b9s-preview-entrypoint"]
