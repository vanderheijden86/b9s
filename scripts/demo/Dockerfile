# syntax=docker/dockerfile:1.7
# The public b9s demo: one writable sample project, reset every half hour.
# See scripts/demo/README.md. Build from the repository root.
FROM golang:1.25-alpine@sha256:1ae0735f00daffa3aaf1363a5184c0d2dc55c78e3db4ec70241cdac97bf84b59 AS builder

RUN apk add --no-cache build-base
WORKDIR /src
COPY . .
RUN CGO_ENABLED=1 go build -mod=vendor \
    -ldflags "-s -w" \
    -o /out/b9s ./cmd/b9s

# Built here, as in scripts/web, because the published bd is linked against glibc.
FROM golang:1.26-alpine@sha256:8ac98ca534ac3f51e1f420a1dd2c15e74c75cfa0f23f3ad27eb5d7236c349a0c AS bd
ARG BD_VERSION=v1.3.0
RUN apk add --no-cache build-base git
ENV GOTOOLCHAIN=auto
RUN git clone --depth 1 --branch "$BD_VERSION" https://github.com/gastownhall/beads /src \
    && cd /src && CGO_ENABLED=1 go build -tags gms_pure_go \
       -ldflags "-s -w -X main.Build=$BD_VERSION" -o /out-bd ./cmd/bd

FROM alpine:3.22@sha256:5291449c3df73caf6ed85e649dec1b9e818b39a5d8c871e97afc13e9cd5e8fa8 AS runtime
ARG SEED=examples/space-programme/.beads/issues.jsonl
RUN apk add --no-cache git \
    && mkdir -p /app/seed /data \
    && chown 65532:65532 /data
COPY --from=builder /out/b9s /usr/local/bin/b9s
COPY --from=bd /out-bd /usr/local/bin/bd
COPY ${SEED} /app/seed/issues.jsonl
COPY scripts/demo/entrypoint.sh /usr/local/bin/b9s-demo-entrypoint
RUN chmod 0755 /usr/local/bin/b9s /usr/local/bin/b9s-demo-entrypoint

USER 65532:65532
EXPOSE 7979
ENTRYPOINT ["/usr/local/bin/b9s-demo-entrypoint"]
