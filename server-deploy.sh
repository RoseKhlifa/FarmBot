#!/usr/bin/env bash
set -Eeuo pipefail

cd "$(dirname "$0")"
echo "==> building and starting the single-process FarmBot container"
docker compose up -d --build

echo "==> waiting for /api/health"
for _ in $(seq 1 30); do
  if curl -fsS -m 5 http://127.0.0.1:3007/api/health >/dev/null; then
    echo "==> FarmBot is ready at http://<server-ip>:3007"
    exit 0
  fi
  sleep 2
done

echo "==> service did not become healthy; inspect: docker compose logs --tail=100" >&2
exit 1
