#!/usr/bin/env bash
set -euo pipefail

if [ ! -s "$PGDATA/PG_VERSION" ]; then
  rm -rf "$PGDATA"/*

  until PGPASSWORD="$REPLICATION_PASSWORD" pg_basebackup \
    -h "$PRIMARY_HOST" \
    -p "${PRIMARY_PORT:-55432}" \
    -D "$PGDATA" \
    -U "$REPLICATION_USER" \
    -Fp \
    -Xs \
    -P \
    -R; do
    echo "waiting for primary base backup from ${PRIMARY_HOST}:${PRIMARY_PORT:-55432}..."
    sleep 5
  done

  chown -R postgres:postgres "$PGDATA"
  chmod 700 "$PGDATA"
fi

exec docker-entrypoint.sh "$@"
