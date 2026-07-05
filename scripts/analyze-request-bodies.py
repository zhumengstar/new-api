#!/usr/bin/env python3
"""Analyze stored request bodies in New API consume logs.

The script reads consume logs whose `other` field contains `request_body`,
extracts user input text from OpenAI/Gemini-style request bodies, writes a
100-line user-input sample, and classifies all matching requests.

It intentionally avoids printing full request bodies to stdout.
"""

from __future__ import annotations

import argparse
import collections
import hashlib
import json
import os
import re
import subprocess
import sys
from pathlib import Path
from typing import Any


DEFAULT_OUTPUT_DIR = Path("/tmp/new-api-request-body-analysis")


CLASSIFIERS: list[tuple[str, tuple[str, ...]]] = [
    ("体育赛事赔率分析", ("盘口", "赔率", "比分", "体彩", "让球", "大小球", "水位")),
    (
        "电商商品审核/文案",
        (
            "卖家SKU",
            "售卖规格",
            "listing",
            "轮播图",
            "商品图",
            "产品文案",
            "卖点文案",
            "详情文案",
            "SKU",
            "亚马逊",
            "主图",
        ),
    ),
    (
        "短剧/AI视频分镜提示词",
        (
            "剧本",
            "分镜",
            "镜头",
            "短剧",
            "运镜",
            "提示词",
            "画面景别",
            "人物资产",
            "出场人物",
            "道具：",
            "人物：",
            "即梦",
            "豆包",
            "漫剧",
            "Seedance",
            "Generation Block",
            "video prompt",
            "prompt array",
            "cinematographer",
        ),
    ),
    (
        "角色扮演/互动剧情",
        ("Start a new chat", "<PlotHistory>", "desktop_ui", "NSFW", "roleplay", "小剧场", "状态栏", "弹幕", "穿书攻略"),
    ),
    ("小说创作/改写", ("小说", "番茄风", "章节", "男频", "女频", "爽文", "续写", "人物刻画")),
    (
        "报告论文/文档写作",
        ("研究报告", "设计报告", "开题", "论文", "项目总体", "编制依据", "承办单位", "docx", "pdf", "STM32", "扩写"),
    ),
    ("装修展陈/报价方案", ("装修", "报价", "布局图", "展示区", "文化长廊", "利润", "材料", "展厅")),
    ("自媒体选题/脚本策划", ("工厂", "创业", "拍摄", "选题", "抖音", "小红书", "视频号", "账号", "脚本策划")),
    ("翻译/字幕处理", ("翻译", "译成", "字幕", "translate", "Translate")),
    ("测试/压测/计费验证", ("user message user message", "Calculate and respond with ONLY the number")),
    (
        "编程/调试",
        (
            "代码",
            "报错",
            "接口",
            "组件",
            "函数",
            "数据库",
            "前端",
            "后端",
            "typescript",
            "React",
            "react",
            "golang",
            "python",
            "SQL",
            "sql",
            "error stack",
            "diff --git",
            "api endpoint",
        ),
    ),
    ("图片理解/视觉分析", ("[image]", "图片", "图像", "识别", "审计以下图片", "根据图片")),
]


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description="Analyze request_body data in consume logs.")
    parser.add_argument(
        "--dsn",
        default=os.environ.get("REMOTE_SQL_DSN") or os.environ.get("SQL_DSN"),
        help="PostgreSQL DSN. Defaults to REMOTE_SQL_DSN or SQL_DSN.",
    )
    parser.add_argument(
        "--dsn-from-compose-comment",
        action="store_true",
        help="Use the first commented SQL_DSN=postgresql://... line in docker-compose.yml.",
    )
    parser.add_argument(
        "--psql-container",
        default=os.environ.get("PSQL_CONTAINER", "new-api-dev-pg"),
        help="Docker container that has psql installed. Default: new-api-dev-pg.",
    )
    parser.add_argument("--output-dir", type=Path, default=DEFAULT_OUTPUT_DIR)
    parser.add_argument("--sample-size", type=int, default=100)
    parser.add_argument(
        "--snippet-chars",
        type=int,
        default=50_000,
        help="Characters of logs.other to export for classification. Default: 50000.",
    )
    parser.add_argument(
        "--full-sample",
        type=int,
        default=160,
        help="Number of full request bodies to fetch for extracting sample user inputs. Default: 160.",
    )
    parser.add_argument("--start-timestamp", type=int, default=None)
    parser.add_argument("--end-timestamp", type=int, default=None)
    return parser.parse_args()


def dsn_from_compose_comment() -> str:
    compose = Path("docker-compose.yml")
    if not compose.exists():
        raise SystemExit("docker-compose.yml not found")
    match = re.search(r"#\s*-\s*SQL_DSN=(postgresql://[^\n]+)", compose.read_text())
    if not match:
        raise SystemExit("No commented PostgreSQL SQL_DSN found in docker-compose.yml")
    return match.group(1).strip()


def run_psql(dsn: str, container: str, sql: str, timeout: int = 240) -> str:
    cmd = [
        "docker",
        "exec",
        "-i",
        "-e",
        f"REMOTE_DSN={dsn}",
        container,
        "sh",
        "-lc",
        'psql "$REMOTE_DSN" -At',
    ]
    proc = subprocess.run(cmd, input=sql, text=True, capture_output=True, timeout=timeout)
    if proc.returncode != 0:
        raise SystemExit(proc.stderr.strip() or f"psql failed with exit code {proc.returncode}")
    return proc.stdout


def json_loads_lenient(value: Any) -> dict[str, Any]:
    if isinstance(value, dict):
        return value
    if not value:
        return {}
    try:
        return json.loads(str(value).replace("\x00", ""))
    except json.JSONDecodeError:
        return {}


def text_from_content(content: Any) -> str:
    if content is None:
        return ""
    if isinstance(content, str):
        return content
    if isinstance(content, list):
        parts: list[str] = []
        for item in content:
            if isinstance(item, str):
                parts.append(item)
                continue
            if not isinstance(item, dict):
                continue
            item_type = str(item.get("type") or "")
            if isinstance(item.get("text"), str):
                parts.append(item["text"])
            elif isinstance(item.get("content"), str):
                parts.append(item["content"])
            elif "parts" in item:
                parts.append(text_from_content(item["parts"]))
            elif item_type in {"image_url", "input_image"} or "image_url" in item or "image" in item_type:
                parts.append("[image]")
        return "\n".join(part for part in parts if part)
    if isinstance(content, dict):
        for key in ("text", "content", "prompt", "input"):
            if isinstance(content.get(key), str):
                return content[key]
        if "parts" in content:
            return text_from_content(content["parts"])
    return ""


def extract_user_input(row: dict[str, Any]) -> str:
    other = json_loads_lenient(row.get("other"))
    request_body = json_loads_lenient(other.get("request_body")) if other else {}
    body = request_body.get("body", request_body) if isinstance(request_body, dict) else {}
    texts: list[str] = []

    if isinstance(body, dict):
        if body.get("prompt") is not None:
            texts.append(text_from_content(body.get("prompt")))
        if body.get("input") is not None:
            texts.append(text_from_content(body.get("input")))
        for message in body.get("messages") or []:
            if isinstance(message, dict) and message.get("role") == "user":
                texts.append(text_from_content(message.get("content")))
        for content in body.get("contents") or []:
            if isinstance(content, dict) and content.get("role") in (None, "user"):
                texts.append(text_from_content(content.get("parts", content)))
    else:
        texts.append(text_from_content(body))

    text = "\n\n".join(part.strip() for part in texts if part and part.strip())
    return re.sub(r"\s+", " ", text).strip()


def classify_text(text: str) -> str:
    for category, keywords in CLASSIFIERS:
        if any(keyword in text for keyword in keywords):
            return category
    return "其他/未细分"


def where_clause(args: argparse.Namespace) -> str:
    clauses = ["type=2", "other like '%request_body%'"]
    if args.start_timestamp is not None:
        clauses.append(f"created_at >= {int(args.start_timestamp)}")
    if args.end_timestamp is not None:
        clauses.append(f"created_at <= {int(args.end_timestamp)}")
    return " and ".join(clauses)


def write_sample(dsn: str, args: argparse.Namespace, output_dir: Path) -> Path:
    sql = f"""
select json_build_object(
  'id', id,
  'other', other
)
from logs
where {where_clause(args)}
order by md5(coalesce(request_id, '') || id::text)
limit {int(args.full_sample)};
"""
    stdout = run_psql(dsn, args.psql_container, sql)
    path = output_dir / "user_inputs_100_only.txt"
    seen: set[str] = set()
    count = 0
    with path.open("w", encoding="utf-8") as handle:
        for line in stdout.splitlines():
            if not line.strip():
                continue
            text = extract_user_input(json.loads(line))
            if not text:
                continue
            digest = hashlib.sha1(text.encode("utf-8")).hexdigest()
            if digest in seen:
                continue
            seen.add(digest)
            count += 1
            handle.write(f"{count}. {text}\n")
            if count >= args.sample_size:
                break
    return path


def export_snippets(dsn: str, args: argparse.Namespace, output_dir: Path) -> Path:
    path = output_dir / "request_body_snippets.jsonl"
    sql = f"""
select json_build_object(
  'id', id,
  'prompt_tokens', prompt_tokens,
  'completion_tokens', completion_tokens,
  'quota', quota,
  'other', left(other, {int(args.snippet_chars)})
)
from logs
where {where_clause(args)};
"""
    stdout = run_psql(dsn, args.psql_container, sql, timeout=600)
    path.write_text(stdout, encoding="utf-8")
    return path


def classify_all(snippets_path: Path, output_dir: Path) -> tuple[Path, Path]:
    detail_path = output_dir / "request_body_classification.tsv"
    summary_path = output_dir / "request_body_classification_summary.tsv"
    stats: dict[str, list[int]] = collections.defaultdict(lambda: [0, 0, 0, 0])

    with snippets_path.open(encoding="utf-8") as source, detail_path.open("w", encoding="utf-8") as detail:
        detail.write("id\tcategory\tprompt_tokens\tcompletion_tokens\tquota\n")
        for line in source:
            if not line.strip():
                continue
            row = json.loads(line)
            text = str(row.get("other") or "")
            category = classify_text(text)
            prompt_tokens = int(row.get("prompt_tokens") or 0)
            completion_tokens = int(row.get("completion_tokens") or 0)
            quota = int(row.get("quota") or 0)
            stat = stats[category]
            stat[0] += 1
            stat[1] += prompt_tokens
            stat[2] += completion_tokens
            stat[3] += quota
            detail.write(f"{row.get('id')}\t{category}\t{prompt_tokens}\t{completion_tokens}\t{quota}\n")

    total = sum(stat[0] for stat in stats.values())
    with summary_path.open("w", encoding="utf-8") as summary:
        summary.write("category\tcount\tpct\tinput_tokens\toutput_tokens\tquota\n")
        for category, stat in sorted(stats.items(), key=lambda item: item[1][0], reverse=True):
            pct = (stat[0] * 100 / total) if total else 0
            summary.write(f"{category}\t{stat[0]}\t{pct:.2f}%\t{stat[1]}\t{stat[2]}\t{stat[3]}\n")
    return detail_path, summary_path


def main() -> int:
    args = parse_args()
    dsn = dsn_from_compose_comment() if args.dsn_from_compose_comment else args.dsn
    if not dsn:
        print("Provide --dsn, set REMOTE_SQL_DSN/SQL_DSN, or use --dsn-from-compose-comment.", file=sys.stderr)
        return 2

    output_dir = args.output_dir
    output_dir.mkdir(parents=True, exist_ok=True)

    sample_path = write_sample(dsn, args, output_dir)
    snippets_path = export_snippets(dsn, args, output_dir)
    detail_path, summary_path = classify_all(snippets_path, output_dir)

    print(f"user_input_sample={sample_path}")
    print(f"classification_detail={detail_path}")
    print(f"classification_summary={summary_path}")
    print(summary_path.read_text(encoding="utf-8"))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
