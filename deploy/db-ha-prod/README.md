# new-api DB HA Production Staging

This directory contains the files used to stage a three-node PostgreSQL HA setup for the new-api deployment.

Node mapping:

- `149.118.50.244`: preferred initial primary
- `192.9.154.16`: standby
- `132.145.124.107`: standby

The application should connect to the local router, not directly to a database node:

```bash
SQL_DSN=postgresql://root:REDACTED@db-ha:5432/new-api
```

The production deployment copies these files to `/opt/newapi-db-ha` on the relevant machines.

Important: this is a pragmatic HA layer for the current migration. A long-term setup should use Patroni or repmgr with quorum/fencing and a private network.
