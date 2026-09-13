#!/bin/sh
set -eu

if [ "${REQUEST_METHOD:-}" != "POST" ]; then
  printf 'Status: 405 Method Not Allowed\r\nAllow: POST\r\nContent-Type: application/json\r\n\r\n'
  printf '{"ok":false,"error":"method not allowed"}\n'
  exit 0
fi

case "${QUERY_STRING:-}" in
  key=Up) key=Up ;;
  key=Down) key=Down ;;
  key=Left) key=Left ;;
  key=Right) key=Right ;;
  key=Enter) key=Enter ;;
  key=Escape) key=Escape ;;
  key=[0-9]) key="${QUERY_STRING#key=}" ;;
  *)
    printf 'Status: 400 Bad Request\r\nContent-Type: application/json\r\n\r\n'
    printf '{"ok":false,"error":"unsupported key"}\n'
    exit 0
    ;;
esac

if ! tmux has-session -t b9s 2>/dev/null; then
  printf 'Status: 503 Service Unavailable\r\nContent-Type: application/json\r\n\r\n'
  printf '{"ok":false,"error":"terminal unavailable"}\n'
  exit 0
fi

tmux send-keys -t b9s "$key"
printf 'Status: 200 OK\r\nCache-Control: no-store\r\nContent-Type: application/json\r\n\r\n'
printf '{"ok":true,"key":"%s"}\n' "$key"
