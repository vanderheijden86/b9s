# syntax=docker/dockerfile:1.7
# b9s for several people at once, behind an authenticating reverse proxy.
# See scripts/web/README.md.
FROM golang:1.25-alpine@sha256:1ae0735f00daffa3aaf1363a5184c0d2dc55c78e3db4ec70241cdac97bf84b59 AS builder

RUN apk add --no-cache build-base
WORKDIR /src
COPY . .
RUN CGO_ENABLED=1 go build -mod=vendor \
    -ldflags "-s -w" \
    -o /out/b9s ./cmd/b9s

# b9s writes by running bd, so the image carries the bd release a laptop runs.
# It is built here because the published binary is linked against glibc.
FROM golang:1.26-alpine@sha256:8ac98ca534ac3f51e1f420a1dd2c15e74c75cfa0f23f3ad27eb5d7236c349a0c AS bd
ARG BD_VERSION=v1.3.0
RUN apk add --no-cache build-base git
ENV GOTOOLCHAIN=auto
RUN git clone --depth 1 --branch "$BD_VERSION" https://github.com/gastownhall/beads /src \
    && cd /src && CGO_ENABLED=1 go build -tags gms_pure_go \
       -ldflags "-s -w -X main.Build=$BD_VERSION" -o /out-bd ./cmd/bd

FROM alpine:3.22@sha256:5291449c3df73caf6ed85e649dec1b9e818b39a5d8c871e97afc13e9cd5e8fa8 AS runtime
ARG COMMIT_SHA
# git: bd reads beads.role from git config. busybox-extras: httpd for /__version.
RUN apk add --no-cache busybox-extras git mariadb-client \
    && test -n "$COMMIT_SHA" \
    && mkdir -p /app /data /www \
    && printf '%s\n' "$COMMIT_SHA" > /app/.commit-sha \
    && chown -R 65532:65532 /data /www
COPY --from=builder /out/b9s /usr/local/bin/b9s
COPY --from=bd /out-bd /usr/local/bin/bd
COPY scripts/web/entrypoint.sh /usr/local/bin/b9s-web-entrypoint
RUN chmod 0755 /usr/local/bin/b9s /usr/local/bin/b9s-web-entrypoint

USER 65532:65532
EXPOSE 7681 7682
ENTRYPOINT ["/usr/local/bin/b9s-web-entrypoint"]
