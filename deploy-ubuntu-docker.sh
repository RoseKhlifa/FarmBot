#!/usr/bin/env bash
# Fast Ubuntu deployment for the single-process FarmBot Docker image.
# Run this script from a checked-out FarmBot repository.

set -Eeuo pipefail

SCRIPT_DIR="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
ENV_FILE="${SCRIPT_DIR}/.env"
PUBLIC_PORT="7777"

log() {
    printf '[FarmBot] %s\n' "$*"
}

die() {
    printf '[FarmBot] ERROR: %s\n' "$*" >&2
    exit 1
}

command -v docker >/dev/null 2>&1 || die "Docker is not installed. Install Docker Engine first."
docker info >/dev/null 2>&1 || die "Docker daemon is not running or the current user has no Docker permission."

if docker compose version >/dev/null 2>&1; then
    COMPOSE=(docker compose)
elif command -v docker-compose >/dev/null 2>&1; then
    COMPOSE=(docker-compose)
else
    die "Docker Compose is not installed. Install the Docker Compose plugin first."
fi

compose() {
    "${COMPOSE[@]}" "$@"
}

read_env_value() {
    local key="$1"
    if [[ ! -f "${ENV_FILE}" ]]; then
        return 0
    fi
    awk -F= -v key="${key}" '$1 == key {sub(/^[^=]*=/, ""); print; exit}' "${ENV_FILE}"
}

set_env_value() {
    local key="$1"
    local value="$2"
    if grep -qE "^${key}=" "${ENV_FILE}" 2>/dev/null; then
        sed -i -E "s|^${key}=.*$|${key}=${value}|" "${ENV_FILE}"
    else
        printf '%s=%s\n' "${key}" "${value}" >>"${ENV_FILE}"
    fi
}

random_hex() {
    local bytes="$1"
    if command -v openssl >/dev/null 2>&1; then
        openssl rand -hex "${bytes}"
        return
    fi
    od -An -N "${bytes}" -tx1 /dev/urandom | tr -d ' \n'
}

cd -- "${SCRIPT_DIR}"

if [[ ! -f docker-compose.yml ]]; then
    die "docker-compose.yml was not found in ${SCRIPT_DIR}"
fi

umask 077
touch "${ENV_FILE}"
chmod 600 "${ENV_FILE}" 2>/dev/null || true

# Docker Compose reads .env automatically. Keep the public port persistent so
# later `docker compose restart` commands keep using 7777 -> container 3007.
set_env_value PORT "${PUBLIC_PORT}"

master_key="$(read_env_value FARM_MASTER_KEY || true)"
if [[ -z "${master_key}" || "${master_key}" == replace-with-* ]]; then
    master_key="$(random_hex 32)"
    set_env_value FARM_MASTER_KEY "${master_key}"
    log "Generated a new FARM_MASTER_KEY and stored it in ${ENV_FILE}"
fi

admin_password="$(read_env_value ADMIN_PASSWORD || true)"
generated_admin_password=0
if [[ -z "${admin_password}" || "${admin_password}" == change-this-before-first-start ]]; then
    admin_password="$(random_hex 12)"
    set_env_value ADMIN_PASSWORD "${admin_password}"
    generated_admin_password=1
fi

compose config >/dev/null || die "Docker Compose configuration validation failed"

log "Building and starting FarmBot on public port ${PUBLIC_PORT}"
compose up -d --build --remove-orphans

container_id="$(compose ps -q farmbot | head -n 1)"
[[ -n "${container_id}" ]] || die "FarmBot container was not created"

healthy=0
for _ in $(seq 1 60); do
    status="$(docker inspect --format '{{if .State.Health}}{{.State.Health.Status}}{{else}}{{.State.Status}}{{end}}' "${container_id}" 2>/dev/null || true)"
    case "${status}" in
        healthy)
            healthy=1
            break
            ;;
        unhealthy|exited|dead)
            break
            ;;
    esac
    sleep 2
done

if [[ "${healthy}" != 1 ]]; then
    log "Container did not become healthy. Recent logs:"
    compose logs --tail=120 farmbot || true
    die "FarmBot startup failed; inspect with: ${COMPOSE[*]} logs -f farmbot"
fi

http_ok=0
http_checker=0
if command -v curl >/dev/null 2>&1; then
    http_checker=1
    curl -fsS --max-time 5 "http://127.0.0.1:${PUBLIC_PORT}/api/health" >/dev/null && http_ok=1 || true
elif command -v wget >/dev/null 2>&1; then
    http_checker=1
    wget -qO- --timeout=5 "http://127.0.0.1:${PUBLIC_PORT}/api/health" >/dev/null && http_ok=1 || true
else
    log "curl/wget is unavailable; Docker healthcheck passed, but host HTTP check was skipped"
fi

if [[ "${http_checker}" == 1 && "${http_ok}" != 1 ]]; then
    die "Container is healthy but port ${PUBLIC_PORT} is not responding locally"
fi

log "FarmBot is running: http://<your-server-ip>:${PUBLIC_PORT}"
log "Health endpoint: http://<your-server-ip>:${PUBLIC_PORT}/api/health"
if [[ "${generated_admin_password}" == 1 ]]; then
    log "Generated initial admin credentials: admin / ${admin_password}"
    log "Change the admin password after the first login."
fi
if command -v ufw >/dev/null 2>&1 && ufw status 2>/dev/null | grep -q 'Status: active'; then
    log "UFW is active. Allow the public port if needed: sudo ufw allow ${PUBLIC_PORT}/tcp"
fi
log "Logs: ${COMPOSE[*]} logs -f farmbot"
