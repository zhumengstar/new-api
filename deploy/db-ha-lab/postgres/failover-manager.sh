#!/usr/bin/env bash
set -euo pipefail

nodes="${BACKEND_NODES:-pg-0 pg-1 pg-2}"
state_file="${STATE_FILE:-/ha-state/primary}"

query_node() {
  local node="$1"
  PGPASSWORD="$POSTGRES_PASSWORD" psql \
    -h "$node" \
    -U postgres \
    -d newapi \
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
      echo "promoting standby ${node}"
      query_node "$node" "select pg_promote(true, 30);" >/dev/null
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
    printf '%s:5432\n' "$primary" > "${state_file}.tmp"
    mv "${state_file}.tmp" "$state_file"
  fi

  sleep 3
done
