#!/usr/bin/env bash
set -Eeuo pipefail

PROJECT_DIR="${PROJECT_DIR:-/opt/new-api}"
SERVICE="${SERVICE:-new-api}"
CONTAINER_NAME="${CONTAINER_NAME:-new-api}"
COMPOSE_FILE="${COMPOSE_FILE:-docker-compose.yml}"
IMAGE_REPO="${IMAGE_REPO:-local/new-api}"
TAG="${TAG:-$(date +%H%M%S)}"
IMAGE="${IMAGE:-${IMAGE_REPO}:${TAG}}"
DOMAIN="${DOMAIN:-ai.muling.store}"
ENTRY_NGINX_CONTAINER="${ENTRY_NGINX_CONTAINER:-entry-nginx}"
ENTRY_NGINX_CONF="${ENTRY_NGINX_CONF:-/opt/entry-nginx/conf.d/${DOMAIN}.conf}"
NETWORK="${NETWORK:-new-api_new-api-network}"
HEALTH_URL="${HEALTH_URL:-http://127.0.0.1:3000/api/status}"
RESOURCE_LOG_DIR="${RESOURCE_LOG_DIR:-${PROJECT_DIR}/logs/rebuild-deploy}"
NO_CACHE=0
SKIP_NGINX=0
SKIP_PULL=0
SKIP_TESTS=0
MONITOR=0
KEEP_OLD_IMAGE=0

usage() {
  cat <<USAGE
Usage: $0 [options]

Rebuild and deploy the local NewAPI Docker image on the OC host.
Defaults are safe for /opt/new-api and ai.muling.store.

Options:
  --no-cache       Build with docker --no-cache
  --monitor        Sample docker stats during build/deploy and write a CSV report
  --skip-nginx     Do not connect entry-nginx to ${NETWORK} or patch proxy_pass
  --skip-pull      Do not run git pull --ff-only before building
  --skip-tests     Skip lightweight Go test for touched common package
  --keep-old-image Do not remove the previous local image after successful deploy
  -h, --help       Show this help

Environment overrides:
  PROJECT_DIR SERVICE CONTAINER_NAME COMPOSE_FILE IMAGE_REPO TAG IMAGE DOMAIN
  ENTRY_NGINX_CONTAINER ENTRY_NGINX_CONF NETWORK HEALTH_URL RESOURCE_LOG_DIR

Notes:
  - This script does not print SQL_DSN or other container environment secrets.
  - Nginx is configured to use Docker internal DNS: proxy_pass http://${SERVICE}:3000;
  - The script backs up docker-compose.yml and nginx conf before modifying them.
USAGE
}

log() { printf '[%s] %s\n' "$(date +%F_%T)" "$*"; }
fail() { printf 'ERROR: %s\n' "$*" >&2; exit 1; }

while [[ $# -gt 0 ]]; do
  case "$1" in
    --no-cache) NO_CACHE=1 ;;
    --monitor) MONITOR=1 ;;
    --skip-nginx) SKIP_NGINX=1 ;;
    --skip-pull) SKIP_PULL=1 ;;
    --skip-tests) SKIP_TESTS=1 ;;
    --keep-old-image) KEEP_OLD_IMAGE=1 ;;
    -h|--help) usage; exit 0 ;;
    *) fail "Unknown argument: $1" ;;
  esac
  shift
done

cd "$PROJECT_DIR"
[[ -f "$COMPOSE_FILE" ]] || fail "Compose file not found: ${PROJECT_DIR}/${COMPOSE_FILE}"
[[ -f Dockerfile ]] || fail "Dockerfile not found in ${PROJECT_DIR}"

if docker compose version >/dev/null 2>&1; then
  COMPOSE=(docker compose -f "$COMPOSE_FILE")
elif command -v docker-compose >/dev/null 2>&1; then
  COMPOSE=(docker-compose -f "$COMPOSE_FILE")
else
  fail 'Neither docker compose nor docker-compose is available'
fi

mkdir -p "$RESOURCE_LOG_DIR"
REPORT_PREFIX="${RESOURCE_LOG_DIR}/$(date +%Y%m%d-%H%M%S)-${TAG}"
MONITOR_CSV="${REPORT_PREFIX}-docker-stats.csv"
MONITOR_PID=''

start_monitor() {
  [[ "$MONITOR" == 1 ]] || return 0
  log "Starting docker stats monitor: ${MONITOR_CSV}"
  (
    echo 'ts,name,cpu_percent,mem_usage_limit,mem_percent,net_io,block_io,pids'
    while true; do
      ts="$(date +%s)"
      docker stats --no-stream --format "${ts},{{.Name}},{{.CPUPerc}},{{.MemUsage}},{{.MemPerc}},{{.NetIO}},{{.BlockIO}},{{.PIDs}}" || true
      sleep 2
    done
  ) > "$MONITOR_CSV" &
  MONITOR_PID=$!
}

stop_monitor() {
  if [[ -n "${MONITOR_PID}" ]]; then
    kill "$MONITOR_PID" >/dev/null 2>&1 || true
    wait "$MONITOR_PID" 2>/dev/null || true
    log 'Stopped docker stats monitor'
  fi
}
trap stop_monitor EXIT

wait_health() {
  local deadline=$((SECONDS + 180))
  log "Waiting for container health and ${HEALTH_URL}"
  while (( SECONDS < deadline )); do
    local state status body
    state="$(docker inspect "$CONTAINER_NAME" --format '{{.State.Status}}' 2>/dev/null || true)"
    status="$(docker inspect "$CONTAINER_NAME" --format '{{if .State.Health}}{{.State.Health.Status}}{{else}}no-healthcheck{{end}}' 2>/dev/null || true)"
    body="$(curl -fsS --max-time 5 "$HEALTH_URL" 2>/dev/null || true)"
    if [[ "$state" == 'running' && ( "$status" == 'healthy' || "$status" == 'no-healthcheck' ) && "$body" == *'"success":true'* ]]; then
      log "Health check passed: state=${state}, health=${status}"
      return 0
    fi
    sleep 5
  done
  docker ps --filter "name=^/${CONTAINER_NAME}$" --format 'table {{.Names}}\t{{.Image}}\t{{.Status}}' || true
  docker logs --tail 120 "$CONTAINER_NAME" || true
  fail 'Health check timed out'
}

replace_compose_image() {
  local backup="${COMPOSE_FILE}.bak_$(date +%Y%m%d_%H%M%S)"
  cp -a "$COMPOSE_FILE" "$backup"
  log "Backed up compose: ${backup}"
  python3 - "$COMPOSE_FILE" "$SERVICE" "$IMAGE" <<'PY'
import sys
from pathlib import Path
path = Path(sys.argv[1])
service = sys.argv[2]
image = sys.argv[3]
lines = path.read_text().splitlines(True)
out = []
in_services = False
in_target = False
target_indent = None
replaced = False
for line in lines:
    stripped = line.lstrip(' ')
    indent = len(line) - len(stripped)
    if stripped.startswith('services:') and indent == 0:
        in_services = True
        in_target = False
        target_indent = None
    elif in_services and stripped and not stripped.startswith('#') and indent == 2 and stripped.startswith(service + ':'):
        in_target = True
        target_indent = indent
    elif in_target and stripped and not stripped.startswith('#') and indent <= target_indent and not stripped.startswith(service + ':'):
        in_target = False
    if in_target and stripped.startswith('image:') and not stripped.startswith('#'):
        out.append(' ' * indent + f'image: {image}\n')
        replaced = True
    else:
        out.append(line)
if not replaced:
    raise SystemExit(f'Could not find image: line under service {service}')
path.write_text(''.join(out))
PY
}

configure_nginx_internal_network() {
  [[ "$SKIP_NGINX" == 0 ]] || { log 'Skipping nginx internal network configuration'; return 0; }
  if ! docker ps --format '{{.Names}}' | grep -qx "$ENTRY_NGINX_CONTAINER"; then
    log 'entry-nginx container not running; skip nginx config'
    return 0
  fi
  if ! docker network inspect "$NETWORK" >/dev/null 2>&1; then
    fail "Docker network not found: ${NETWORK}"
  fi
  if ! docker network inspect "$NETWORK" --format '{{range .Containers}}{{.Name}}{{"\n"}}{{end}}' | grep -qx "$ENTRY_NGINX_CONTAINER"; then
    log "Connecting ${ENTRY_NGINX_CONTAINER} to ${NETWORK} for internal service DNS"
    docker network connect "$NETWORK" "$ENTRY_NGINX_CONTAINER"
  else
    log "${ENTRY_NGINX_CONTAINER} already connected to ${NETWORK}"
  fi

  if [[ -f "$ENTRY_NGINX_CONF" ]]; then
    local nginx_backup="${ENTRY_NGINX_CONF}.bak_$(date +%Y%m%d_%H%M%S)"
    cp -a "$ENTRY_NGINX_CONF" "$nginx_backup"
    log "Backed up nginx conf: ${nginx_backup}"
    python3 - "$ENTRY_NGINX_CONF" "$SERVICE" <<'PY'
import re, sys
from pathlib import Path
path = Path(sys.argv[1])
service = sys.argv[2]
text = path.read_text()
new = re.sub(r'proxy_pass\s+http://[^;]+;', f'proxy_pass http://{service}:3000;', text, count=1)
if new == text:
    raise SystemExit('No proxy_pass line was changed')
path.write_text(new)
PY
    docker exec "$ENTRY_NGINX_CONTAINER" nginx -t
    docker exec "$ENTRY_NGINX_CONTAINER" nginx -s reload
    log "Nginx reloaded with internal upstream http://${SERVICE}:3000"
  else
    log "Nginx conf not found at ${ENTRY_NGINX_CONF}; network connected only"
  fi
}

old_image="$(docker inspect "$CONTAINER_NAME" --format '{{.Config.Image}}' 2>/dev/null || true)"
log "Current container image: ${old_image:-none}"
log "Target image: ${IMAGE}"

start_monitor

if [[ "$SKIP_PULL" == 0 && -d .git ]]; then
  if [[ -n "$(git status --porcelain)" ]]; then
    log 'Git working tree has local changes; skipping git pull --ff-only'
  else
    log 'Pulling latest source with git pull --ff-only'
    git pull --ff-only
  fi
fi

if [[ "$SKIP_TESTS" == 0 ]]; then
  if command -v go >/dev/null 2>&1; then
    log 'Running lightweight Go regression tests: ./common'
    go test ./common
  else
    log 'Go not installed on host; skipping host Go tests'
  fi
fi

build_args=()
[[ "$NO_CACHE" == 1 ]] && build_args+=(--no-cache)
log "Building image ${IMAGE}"
docker build "${build_args[@]}" -t "$IMAGE" .

replace_compose_image
log "Deploying ${SERVICE} with compose up -d --no-deps"
"${COMPOSE[@]}" up -d --no-deps "$SERVICE"
wait_health
configure_nginx_internal_network

log 'Verifying service through Host header'
curl -fsS --max-time 10 -H "Host: ${DOMAIN}" 'http://127.0.0.1/api/status' | grep -q '"success":true' \
  && log "Domain smoke test passed for ${DOMAIN}" \
  || fail "Domain smoke test failed for ${DOMAIN}"

if [[ "$KEEP_OLD_IMAGE" == 0 && -n "$old_image" && "$old_image" != "$IMAGE" && "$old_image" == local/new-api:* ]]; then
  docker image rm "$old_image" >/dev/null 2>&1 || true
fi

stop_monitor
trap - EXIT

log 'Deployment completed successfully'
log "Image: ${IMAGE}"
[[ "$MONITOR" == 1 ]] && log "Resource CSV: ${MONITOR_CSV}"
