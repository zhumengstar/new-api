#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")"

compose=(docker compose -f docker-compose.yml)
psql=(docker run --rm -i --network new-api-db-ha-lab_db-ha-lab -e PGPASSWORD=newapi-password postgres:15-alpine psql -h db-router -U newapi -d newapi -v ON_ERROR_STOP=1)

wait_for_pgpool() {
  local attempt
  for attempt in $(seq 1 60); do
    if "${psql[@]}" -c "select 1" >/dev/null 2>&1; then
      return 0
    fi
    sleep 2
  done

  echo "pgpool did not become ready in time" >&2
  "${compose[@]}" ps >&2
  return 1
}

show_nodes() {
  "${compose[@]}" exec -T failover-manager sh -c 'printf "current-router-target="; cat /ha-state/primary; printf "\n"'
  "${compose[@]}" ps pg-0 pg-1 pg-2 db-router failover-manager
}

primary_container() {
  "${compose[@]}" exec -T failover-manager sh -c "cat /ha-state/primary 2>/dev/null | cut -d: -f1"
}

echo "Starting temporary database HA lab..."
"${compose[@]}" up -d
wait_for_pgpool

echo
echo "Initial HA router view:"
show_nodes

primary="$(primary_container)"
if [[ -z "${primary}" ]]; then
  echo "Could not detect a writable primary through pgpool" >&2
  exit 1
fi

echo
echo "Detected primary: ${primary}"
echo "Writing before failover..."
"${psql[@]}" <<'SQL'
create table if not exists ha_smoke (
  id bigserial primary key,
  marker text not null,
  created_at timestamptz not null default now()
);
insert into ha_smoke (marker) values ('before-failover');
select inet_server_addr() as server_addr, pg_is_in_recovery() as is_replica;
select id, marker, created_at from ha_smoke order by id;
SQL

echo
echo "Stopping current primary (${primary}) to trigger failover..."
"${compose[@]}" stop "${primary}"

echo "Waiting for pgpool/repmgr to promote a standby..."
sleep 20
wait_for_pgpool

echo
echo "HA router view after failover:"
show_nodes

new_primary="$(primary_container)"
if [[ -z "${new_primary}" || "${new_primary}" == "${primary}" ]]; then
  echo "Failover did not promote a different primary" >&2
  exit 1
fi

echo
echo "Detected new primary: ${new_primary}"
echo "Writing after failover through the same pgpool endpoint..."
"${psql[@]}" <<'SQL'
insert into ha_smoke (marker) values ('after-failover');
select inet_server_addr() as server_addr, pg_is_in_recovery() as is_replica;
select id, marker, created_at from ha_smoke order by id;
SQL

echo
echo "Smoke test passed. Application DSN for this lab:"
echo "SQL_DSN=postgresql://newapi:newapi-password@127.0.0.1:15432/newapi?sslmode=disable"
