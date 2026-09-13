# syntax=docker/dockerfile:1.7
FROM golang:1.25-alpine@sha256:1ae0735f00daffa3aaf1363a5184c0d2dc55c78e3db4ec70241cdac97bf84b59 AS builder

RUN apk add --no-cache build-base
WORKDIR /src
COPY . .
RUN CGO_ENABLED=1 go build -mod=vendor \
    -ldflags "-s -w" \
    -o /out/b9s ./cmd/b9s

FROM tsl0922/ttyd:alpine@sha256:dafdb93a65c9b487f3056933733a06c12f6e447e3bd31ccd4ef6c279cf9a7648 AS runtime
ARG COMMIT_SHA
RUN apk add --no-cache busybox-extras tmux \
    && test -n "$COMMIT_SHA" \
    && mkdir -p /app /data /www /usr/local/share/b9s-preview \
    && printf '%s\n' "$COMMIT_SHA" > /app/.commit-sha \
    && chown -R 65532:65532 /data /www
COPY --from=builder /out/b9s /usr/local/bin/b9s
COPY scripts/preview/entrypoint.sh /usr/local/bin/b9s-preview-entrypoint
COPY scripts/preview/mobile.html /usr/local/share/b9s-preview/mobile.html
COPY scripts/preview/mobile-key.cgi /usr/local/share/b9s-preview/mobile-key.cgi
RUN chmod 0755 /usr/local/bin/b9s /usr/local/bin/b9s-preview-entrypoint \
    /usr/local/share/b9s-preview/mobile-key.cgi

EXPOSE 7681 7682 7683
ENTRYPOINT ["/usr/local/bin/b9s-preview-entrypoint"]
