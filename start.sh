#!/usr/bin/env bash

set -Eeuo pipefail

ROOT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
BOT_PORT="${ADMIN_PORT:-3007}"
cd "$ROOT_DIR"

if ! command -v go >/dev/null 2>&1; then
  echo "[ERROR] Go 1.23+ is required." >&2
  exit 1
fi

if [[ -z "${FARM_MASTER_KEY:-}" ]]; then
  echo "[ERROR] FARM_MASTER_KEY is required; refusing to start without credential encryption." >&2
  exit 1
fi

if [[ ! -f assets/webdist/index.html ]]; then
  if ! command -v pnpm >/dev/null 2>&1 && ! command -v corepack >/dev/null 2>&1; then
    echo "[ERROR] pnpm or corepack is required to build the web assets." >&2
    exit 1
  fi
  if command -v corepack >/dev/null 2>&1; then
    corepack pnpm install -r
    corepack pnpm -C web build
  else
    pnpm install -r
    pnpm -C web build
  fi
fi

echo "[INFO] FarmBot panel: http://localhost:$BOT_PORT"
exec env ADMIN_PORT="$BOT_PORT" go run ./cmd/farmbot
