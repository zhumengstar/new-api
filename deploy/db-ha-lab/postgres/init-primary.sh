#!/usr/bin/env bash
set -euo pipefail

cat >> "$PGDATA/pg_hba.conf" <<EOF
host replication ${REPLICATION_USER} all md5
host all all all md5
EOF

psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname "$POSTGRES_DB" <<SQL
create role ${APP_DB_USER} with login password '${APP_DB_PASSWORD}';
grant all privileges on database ${POSTGRES_DB} to ${APP_DB_USER};
create role ${REPLICATION_USER} with replication login password '${REPLICATION_PASSWORD}';
SQL

psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname "$POSTGRES_DB" <<SQL
grant all on schema public to ${APP_DB_USER};
alter default privileges in schema public grant all on tables to ${APP_DB_USER};
alter default privileges in schema public grant all on sequences to ${APP_DB_USER};
SQL
