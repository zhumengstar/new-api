#!/usr/bin/env bash

set -u

usage() {
  cat <<'EOF'
Usage:
  PASEO_URLS="https://a.example.com,https://b.example.com" \
  PASEO_KEYS="sk-xxx,sk-yyy" \
  ./bin/paseo_failover.sh [message]

Optional env:
  PASEO_CONFIG=./bin/paseo_failover.env
  PASEO_PATH=/v1/chat/completions
  PASEO_MODEL=gpt-4o-mini
  PASEO_FAIL_THRESHOLD=3
  PASEO_TIMEOUT=30
  PASEO_MAX_TOKENS=1
  PASEO_STREAM=false
  PASEO_TEMPERATURE=0
EOF
}

CONFIG_FILE="${PASEO_CONFIG:-./bin/paseo_failover.env}"
if [[ -f "$CONFIG_FILE" ]]; then
  # shellcheck disable=SC1090
  source "$CONFIG_FILE"
fi

trim() {
  local value="$1"
  value="${value#"${value%%[![:space:]]*}"}"
  value="${value%"${value##*[![:space:]]}"}"
  printf '%s' "$value"
}

split_csv() {
  local input="$1"
  local -n out="$2"
  IFS=',' read -r -a out <<<"$input"
  for i in "${!out[@]}"; do
    out[$i]="$(trim "${out[$i]}")"
  done
}

if [[ "${1:-}" == "-h" || "${1:-}" == "--help" ]]; then
  usage
  exit 0
fi

if [[ -z "${PASEO_URLS:-}" ]]; then
  echo "PASEO_URLS is required." >&2
  usage
  exit 1
fi

if [[ -z "${PASEO_KEYS:-}" ]]; then
  echo "PASEO_KEYS is required." >&2
  usage
  exit 1
fi

PASEO_PATH="${PASEO_PATH:-/v1/chat/completions}"
PASEO_MODEL="${PASEO_MODEL:-gpt-4o-mini}"
PASEO_FAIL_THRESHOLD="${PASEO_FAIL_THRESHOLD:-3}"
PASEO_TIMEOUT="${PASEO_TIMEOUT:-30}"
PASEO_MAX_TOKENS="${PASEO_MAX_TOKENS:-1}"
PASEO_STREAM="${PASEO_STREAM:-false}"
PASEO_TEMPERATURE="${PASEO_TEMPERATURE:-0}"
PROMPT="${*:-echo hi}"

urls=()
keys=()
split_csv "$PASEO_URLS" urls
split_csv "$PASEO_KEYS" keys

if [[ "${#urls[@]}" -eq 0 ]]; then
  echo "No valid URLs found in PASEO_URLS." >&2
  exit 1
fi

if [[ "${#keys[@]}" -eq 0 ]]; then
  echo "No valid keys found in PASEO_KEYS." >&2
  exit 1
fi

if [[ "${#keys[@]}" -ne 1 && "${#keys[@]}" -ne "${#urls[@]}" ]]; then
  echo "PASEO_KEYS must contain either 1 key or the same number of entries as PASEO_URLS." >&2
  exit 1
fi

current=0
fail_count=0

payload() {
  python3 - "$PASEO_MODEL" "$PASEO_STREAM" "$PASEO_MAX_TOKENS" "$PASEO_TEMPERATURE" "$PROMPT" <<'PY'
import json
import sys

model = sys.argv[1]
stream = sys.argv[2].lower() == "true"
max_tokens = int(sys.argv[3])
temperature = float(sys.argv[4])
prompt = sys.argv[5]

print(json.dumps({
    "model": model,
    "messages": [{"role": "user", "content": prompt}],
    "stream": stream,
    "max_tokens": max_tokens,
    "temperature": temperature,
}, separators=(",", ":")))
PY
}

while true; do
  url="${urls[$current]}"
  if [[ "${#keys[@]}" -eq 1 ]]; then
    key="${keys[0]}"
  else
    key="${keys[$current]}"
  fi

  request_url="${url%/}${PASEO_PATH}"
  response_file="$(mktemp)"
  request_body="$(payload)"
  http_code="$(
    curl -sS --max-time "$PASEO_TIMEOUT" \
      -o "$response_file" \
      -w "%{http_code}" \
      "$request_url" \
      -H "Content-Type: application/json" \
      -H "Authorization: Bearer $key" \
      --data-binary "$request_body"
  )"
  curl_status=$?

  if [[ "$curl_status" -eq 0 && "$http_code" =~ ^2[0-9][0-9]$ ]]; then
    fail_count=0
    cat "$response_file"
    rm -f "$response_file"
    exit 0
  fi

  fail_count=$((fail_count + 1))
  echo "request failed: url=$request_url http_code=${http_code:-curl_error} fail_count=$fail_count/$PASEO_FAIL_THRESHOLD" >&2
  if [[ -s "$response_file" ]]; then
    cat "$response_file" >&2
  fi
  rm -f "$response_file"

  if [[ "$fail_count" -ge "$PASEO_FAIL_THRESHOLD" ]]; then
    fail_count=0
    current=$(((current + 1) % ${#urls[@]}))
    echo "switched to next endpoint: ${urls[$current]}" >&2
  fi
done
