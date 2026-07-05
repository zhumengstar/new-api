# Request Body Log Analysis

Analyze consume logs that stored `other.request_body`.

## Outputs

The script writes:

- `user_inputs_100_only.txt` — user input text extracted from request bodies only.
- `request_body_snippets.jsonl` — truncated local snippets used for classification.
- `request_body_classification.tsv` — per-log category detail.
- `request_body_classification_summary.tsv` — category counts and token totals.

## Usage

Use an explicit DSN:

```bash
REMOTE_SQL_DSN='postgresql://...' \
  ./scripts/analyze-request-bodies.py --output-dir /tmp/new-api-request-body-analysis
```

For the local development setup where `docker-compose.yml` contains a commented
remote PostgreSQL DSN and `new-api-dev-pg` has `psql` installed:

```bash
./scripts/analyze-request-bodies.py \
  --dsn-from-compose-comment \
  --output-dir /tmp/new-api-request-body-analysis
```

Limit by timestamp if needed:

```bash
./scripts/analyze-request-bodies.py \
  --dsn-from-compose-comment \
  --start-timestamp 1782715800 \
  --end-timestamp 1782716160 \
  --output-dir /tmp/new-api-request-body-analysis-window
```

The script does not print full request bodies to stdout.
