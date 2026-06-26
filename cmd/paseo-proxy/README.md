# paseo proxy

Failover reverse proxy.

## Run

```bash
export PASEO_LISTEN=127.0.0.1:8787
export PASEO_URLS="https://ai.muling.store"
export PASEO_KEYS="sk-xxx,sk-yyy"
export PASEO_FAIL_THRESHOLD=3
go run ./cmd/paseo-proxy
```

Then point your client to:

```text
http://127.0.0.1:8787
```

For the web client, set:

```bash
VITE_PASEO_PROXY_URL=http://127.0.0.1:8787
```

The proxy uses the first URL as primary and switches to the next one after consecutive failures.
