#!/usr/bin/env bash
set -euo pipefail

for ip in ${TRUSTED_NODE_IPS}; do
  {
    echo "host replication ${REPLICATION_USER} ${ip}/32 md5"
    echo "host ${POSTGRES_DB} ${POSTGRES_USER} ${ip}/32 md5"
  } >> "$PGDATA/pg_hba.conf"
done

psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname "$POSTGRES_DB" <<SQL
create role ${REPLICATION_USER} with replication login password '${REPLICATION_PASSWORD}';
SQL
