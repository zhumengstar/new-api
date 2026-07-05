#!/usr/bin/env bash
set -euo pipefail

nodes="${BACKEND_NODES}"
state_file="${STATE_FILE:-/ha-state/primary}"
postgres_user="${POSTGRES_USER:-root}"
postgres_db="${POSTGRES_DB:-new-api}"
postgres_port="${POSTGRES_PORT:-55432}"

query_node() {
  local node="$1"
  PGPASSWORD="$POSTGRES_PASSWORD" psql \
    -h "$node" \
    -p "$postgres_port" \
    -U "$postgres_user" \
    -d "$postgres_db" \
    -At \
    -v ON_ERROR_STOP=1 \
    -c "$2" 2>/dev/null
}

find_primary() {
  local node result
  for node in $nodes; do
    result="$(query_node "$node" "select not pg_is_in_recovery();" || true)"
    if [ "$result" = "t" ]; then
      echo "$node"
      return 0
    fi
  done
  return 1
}

promote_first_standby() {
  local node result
  for node in $nodes; do
    result="$(query_node "$node" "select pg_is_in_recovery();" || true)"
    if [ "$result" = "t" ]; then
      echo "promoting standby ${node}" >&2
      query_node "$node" "select pg_promote(true, 60);" >/dev/null
      echo "$node"
      return 0
    fi
  done
  return 1
}

mkdir -p "$(dirname "$state_file")"

while true; do
  primary="$(find_primary || true)"
  if [ -z "$primary" ]; then
    primary="$(promote_first_standby || true)"
  fi

  if [ -n "$primary" ]; then
    printf '%s:%s\n' "$primary" "$postgres_port" > "${state_file}.tmp"
    mv "${state_file}.tmp" "$state_file"
  fi

  sleep "${CHECK_INTERVAL_SECONDS:-3}"
done
