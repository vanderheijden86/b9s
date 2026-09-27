#!/usr/bin/env bash
# attachments-s3.sh -- disposable S3-compatible server for blobstore tests.
#
# MinIO removed minio/minio from Docker Hub and locked quay.io/minio/minio
# behind a paid subscription in September 2026, so no MinIO image can be
# pulled anonymously anymore. This runs Adobe S3Mock instead: an actively
# maintained, MIT-licensed S3 server built for exactly this purpose.
#
# `start` runs the server bound to 127.0.0.1:19000 only, waits for its
# health endpoint, and prints the B9S_TEST_S3_ENDPOINT export the gated
# TestS3StoreContract suite reads. `stop` removes the container; safe to run
# even if it was never started.
#
# Usage:
#   eval "$(scripts/attachments-s3.sh start)"
#   go test ./internal/blobstore/ -v -timeout 120s
#   scripts/attachments-s3.sh stop
set -euo pipefail

name=b9s-attachments-s3mock
port=19000
# Pinned, not :latest: a floating tag changes what a test run exercises with
# no corresponding diff to review.
image=adobe/s3mock:3.11.0

case "${1:-start}" in
  start)
    # --platform linux/amd64: the arm64 build of this image SIGILLs its JVM
    # on some Apple Silicon Docker Desktop hosts; the emulated amd64 build
    # does not carry that crash.
    docker run -d --rm --platform linux/amd64 --name "$name" -p "127.0.0.1:${port}:9090" \
      -e COM_ADOBE_TESTING_S3MOCK_STORE_INITIAL_BUCKETS=beads-test \
      "$image" >/dev/null
    deadline=$((SECONDS + 30))
    until curl -fsS "http://127.0.0.1:${port}/favicon.ico" >/dev/null 2>&1; do
      if (( SECONDS >= deadline )); then
        echo "s3mock not ready after 30s" >&2
        docker logs "$name" >&2 || true
        docker rm -f "$name" >/dev/null 2>&1 || true
        exit 1
      fi
      sleep 0.5
    done
    echo "export B9S_TEST_S3_ENDPOINT=http://127.0.0.1:${port}"
    ;;
  stop)
    docker rm -f "$name" >/dev/null 2>&1 || true
    ;;
  *)
    echo "usage: $0 {start|stop}" >&2
    exit 2
    ;;
esac
