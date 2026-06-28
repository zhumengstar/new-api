#!/usr/bin/env python3
"""Auto-test NewAPI auto-disabled channels and re-enable channels that pass.

Runs locally on the NewAPI host. It intentionally never prints SQL_DSN, channel keys,
or API tokens. It uses an enabled NewAPI token with the admin-only specific-channel
suffix (`<token>-<channel_id>`) and briefly marks an auto-disabled channel enabled
only for the duration of its own specific-channel smoke test. Passing channels are
left enabled; failing channels are restored to status=3.
"""
from __future__ import annotations

import argparse
import fcntl
import json
import os
import subprocess
import sys
import time
import urllib.error
import urllib.request
from datetime import datetime, timezone
from pathlib import Path
from typing import Any

PROJECT_DIR = Path("/opt/new-api")
LOCK_PATH = PROJECT_DIR / "cron" / "auto_recover_channels.lock"
STATE_DIR = PROJECT_DIR / "cron" / "state"
LOCAL_BASE_URL = os.environ.get("NEWAPI_LOCAL_BASE_URL", "http://127.0.0.1:3000")
REQUEST_TIMEOUT = int(os.environ.get("NEWAPI_RECOVER_TIMEOUT", "180"))
SLEEP_BETWEEN = float(os.environ.get("NEWAPI_RECOVER_INTERVAL", "1.0"))

IMAGE_MODELS = {
    "dall-e-2",
    "dall-e-3",
    "gpt-image-1",
    "gpt-image-2",
    "imagen-3.0-generate-002",
    "imagen-4.0-generate-preview-06-06",
    "imagen-4.0-ultra-generate-preview-06-06",
}


def now() -> str:
    return datetime.now().strftime("%Y-%m-%d %H:%M:%S %z")


def log(msg: str) -> None:
    print(f"[{now()}] {msg}", flush=True)


def run(cmd: list[str], *, input_text: str | None = None, check: bool = True) -> subprocess.CompletedProcess[str]:
    return subprocess.run(
        cmd,
        input=input_text,
        text=True,
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
        check=check,
    )


def get_sql_dsn() -> str:
    proc = run([
        "docker",
        "inspect",
        "new-api",
        "--format",
        "{{range .Config.Env}}{{println .}}{{end}}",
    ])
    for line in proc.stdout.splitlines():
        if line.startswith("SQL_DSN="):
            dsn = line.split("=", 1)[1].strip()
            if dsn:
                return dsn
    raise RuntimeError("SQL_DSN not found in new-api container env")


def psql(dsn: str, sql: str, *, fields: bool = True) -> str:
    args = ["docker", "exec", "-i", "postgres", "psql", dsn, "-v", "ON_ERROR_STOP=1"]
    if fields:
        args += ["-tA", "-F", "\t"]
    args += ["-c", sql]
    proc = run(args)
    return proc.stdout


def sql_literal(value: str) -> str:
    return "'" + value.replace("'", "''") + "'"


def get_enabled_token(dsn: str) -> str:
    sql = """
select key
from tokens
where deleted_at is null and status=1
order by
  case
    when name='模型测试' then 0
    when name='测试key' then 1
    when lower(name) in ('model-test','model_test','test','test-key','test_key') then 2
    when name='token' then 3
    else 4
  end,
  case when user_id=1 then 0 else 1 end,
  id asc
limit 1;
"""
    token = psql(dsn, sql).strip()
    if not token:
        raise RuntimeError("no enabled model-test token found")
    return token


def get_auto_disabled_channels(dsn: str) -> list[dict[str, Any]]:
    sql = """
select id, name, type, coalesce(test_model,''), coalesce(models,''), coalesce("group",''), coalesce(other_info,'')
from channels
where status=3
order by id asc;
"""
    rows = []
    for line in psql(dsn, sql).splitlines():
        if not line.strip():
            continue
        parts = line.split("\t")
        while len(parts) < 7:
            parts.append("")
        rows.append({
            "id": int(parts[0]),
            "name": parts[1],
            "type": parts[2],
            "test_model": parts[3].strip(),
            "models": parts[4].strip(),
            "group": parts[5].strip(),
            "other_info": parts[6].strip(),
        })
    return rows


def choose_model(channel: dict[str, Any]) -> str | None:
    if channel["test_model"]:
        return channel["test_model"]
    models = [m.strip() for m in channel["models"].split(",") if m.strip()]
    if not models:
        return None
    # Prefer a concrete image model when present because this deployment had image endpoint regressions.
    for m in models:
        if is_image_model(m):
            return m
    return models[0]


def is_image_model(model: str) -> bool:
    m = model.strip().lower()
    return m in IMAGE_MODELS or m.startswith("gpt-image-") or m.startswith("dall-e-") or m.startswith("imagen-")


def update_status_for_test(dsn: str, channel_id: int, enabled: bool, reason: str | None = None, update_abilities: bool = True) -> None:
    status = 1 if enabled else 3
    ability_enabled = "true" if enabled else "false"
    if reason is None:
        other_expr = "other_info"
    else:
        # Preserve existing JSON fields where possible and update status_reason/status_time.
        reason_lit = sql_literal(reason)
        other_expr = f"""
        jsonb_set(
          jsonb_set(coalesce(nullif(other_info,''),'{{}}')::jsonb, '{{status_reason}}', to_jsonb({reason_lit}::text), true),
          '{{status_time}}', to_jsonb(extract(epoch from now())::bigint), true
        )::text
        """
    sql = f"""
begin;
update channels set status={status}, other_info={other_expr} where id={channel_id};
"""
    if update_abilities:
        sql += f"update abilities set enabled={ability_enabled} where channel_id={channel_id};\n"
    sql += "commit;\n"
    psql(dsn, sql, fields=False)


def request_newapi(token: str, channel_id: int, model: str) -> tuple[bool, int | None, str, float]:
    if is_image_model(model):
        url = f"{LOCAL_BASE_URL}/v1/images/generations"
        payload = {
            "model": model,
            "prompt": "Minimal automated channel recovery test image: a single gray dot on white background.",
            "size": "1024x1536",
            "quality": "standard",
            "n": 1,
        }
    else:
        url = f"{LOCAL_BASE_URL}/v1/chat/completions"
        payload = {
            "model": model,
            "messages": [{"role": "user", "content": "Reply with OK for channel health check."}],
            "max_tokens": 8,
            "stream": False,
        }
    body = json.dumps(payload).encode("utf-8")
    req = urllib.request.Request(
        url,
        data=body,
        headers={
            "Authorization": f"Bearer {token}-{channel_id}",
            "Content-Type": "application/json",
        },
        method="POST",
    )
    start = time.monotonic()
    try:
        with urllib.request.urlopen(req, timeout=REQUEST_TIMEOUT) as resp:
            text = resp.read(4096).decode("utf-8", errors="replace")
            elapsed = time.monotonic() - start
            return 200 <= resp.status < 300, resp.status, summarize_response(text), elapsed
    except urllib.error.HTTPError as exc:
        text = exc.read(4096).decode("utf-8", errors="replace")
        elapsed = time.monotonic() - start
        return False, exc.code, summarize_response(text), elapsed
    except Exception as exc:  # noqa: BLE001 - cron should log controlled failures per channel
        elapsed = time.monotonic() - start
        return False, None, f"{type(exc).__name__}: {exc}", elapsed


def summarize_response(text: str) -> str:
    if not text:
        return "empty response"
    try:
        data = json.loads(text)
    except Exception:
        return text[:300].replace("\n", " ")
    if "error" in data:
        err = data["error"]
        if isinstance(err, dict):
            return "; ".join(str(err.get(k, "")) for k in ("type", "code", "message") if err.get(k))[:300]
        return str(err)[:300]
    if isinstance(data.get("data"), list) and data["data"]:
        return "ok: data returned"
    if "choices" in data:
        return "ok: choices returned"
    return "ok: json returned"


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--dry-run", action="store_true", help="list target channels without testing or modifying status")
    parser.add_argument("--limit", type=int, default=0, help="max channels to process this run; 0 means all")
    args = parser.parse_args()

    LOCK_PATH.parent.mkdir(parents=True, exist_ok=True)
    STATE_DIR.mkdir(parents=True, exist_ok=True)
    with LOCK_PATH.open("w") as lock_file:
        try:
            fcntl.flock(lock_file.fileno(), fcntl.LOCK_EX | fcntl.LOCK_NB)
        except BlockingIOError:
            # Another run is still active; stay quiet for cron.
            return 0

        dsn = get_sql_dsn()
        channels = get_auto_disabled_channels(dsn)
        if args.limit > 0:
            channels = channels[: args.limit]
        if not channels:
            return 0

        log(f"auto-disabled channels to test: {len(channels)}")
        if args.dry_run:
            for ch in channels:
                model = choose_model(ch) or "<none>"
                log(f"dry-run channel #{ch['id']} model={model} name={ch['name'][:80]}")
            return 0

        token = get_enabled_token(dsn)
        recovered = 0
        failed = 0
        skipped = 0
        for ch in channels:
            channel_id = ch["id"]
            model = choose_model(ch)
            if not model:
                skipped += 1
                log(f"skip channel #{channel_id}: no test_model/models configured")
                continue
            log(f"testing channel #{channel_id} model={model} name={ch['name'][:80]}")
            update_status_for_test(dsn, channel_id, True, "auto-recover cron: temporarily enabled for health test", update_abilities=False)
            ok, http_status, summary, elapsed = request_newapi(token, channel_id, model)
            if ok:
                recovered += 1
                update_status_for_test(dsn, channel_id, True, f"auto-recover cron: test passed at {now()}", update_abilities=True)
                log(f"RECOVERED channel #{channel_id}: http={http_status} elapsed={elapsed:.2f}s {summary}")
            else:
                failed += 1
                reason = f"auto-recover cron: test failed http={http_status} elapsed={elapsed:.2f}s {summary}"
                update_status_for_test(dsn, channel_id, False, reason[:900], update_abilities=True)
                log(f"still disabled channel #{channel_id}: {reason}")
            time.sleep(SLEEP_BETWEEN)

        state = {
            "finished_at": datetime.now(timezone.utc).isoformat(),
            "tested": len(channels),
            "recovered": recovered,
            "failed": failed,
            "skipped": skipped,
        }
        (STATE_DIR / "auto_recover_last_run.json").write_text(json.dumps(state, ensure_ascii=False, indent=2), encoding="utf-8")
        log(f"done tested={len(channels)} recovered={recovered} failed={failed} skipped={skipped}")
        return 0


if __name__ == "__main__":
    raise SystemExit(main())
