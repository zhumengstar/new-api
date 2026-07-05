# Temporary PostgreSQL HA Lab

This directory is a disposable local lab for validating the database HA shape before touching production.

It runs:

- one PostgreSQL primary
- two streaming replicas
- one lightweight failover manager
- one local TCP router that gives applications a stable database endpoint
- no changes to the repository's existing `docker-compose.yml`
- no public database listener; the router is bound to `127.0.0.1:15432`

The app-side idea is simple: new-api connects to one stable database endpoint, while the HA layer decides which PostgreSQL node is currently writable.

This is a temporary local lab, not the final production HA stack. Production should use Patroni, repmgr, or managed PostgreSQL HA with proper quorum/fencing. The lab exists to validate the new-api connection model without touching production.

## Start

```bash
cd deploy/db-ha-lab
docker compose up -d
```

Use this DSN for a temporary local new-api instance:

```bash
SQL_DSN=postgresql://newapi:newapi-password@127.0.0.1:15432/newapi?sslmode=disable
```

## Smoke Test

The smoke test writes through the router, stops the current primary, waits for promotion, then writes through the same router endpoint again.

```bash
cd deploy/db-ha-lab
chmod +x smoke-test.sh
./smoke-test.sh
```

## Cleanup

```bash
cd deploy/db-ha-lab
docker compose down -v
```

## Production Mapping

For the three target machines:

- `149.118.50.244`: initial PostgreSQL primary candidate
- `192.9.154.16`: standby candidate
- `132.145.124.107`: standby candidate

Production should not expose PostgreSQL to the public internet. Put the database nodes on private networking or a WireGuard/Tailscale network, and expose only the HA entrypoint to new-api nodes.

Recommended production app environment shape:

```bash
SQL_DSN=postgresql://newapi:REPLACE_ME@db-ha.internal:5432/newapi?sslmode=disable
SQL_MAX_OPEN_CONNS=12
SQL_MAX_IDLE_CONNS=5
SQL_MAX_LIFETIME=60
```

Use one stable `db-ha.internal` endpoint implemented by Patroni/repmgr plus HAProxy, Pgpool, or a floating private VIP. Do not point app nodes directly at one database host; failover would otherwise require changing every app node.
