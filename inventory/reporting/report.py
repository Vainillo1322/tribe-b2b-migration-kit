"""Report generator: reads aggregates from SQLite and produces
reports/reporte.md (migration impact matrix) and reports/agregados.csv.

The report labels are in English (project deliverable language).
File names keep the names agreed in the migration plan.
"""

import csv
from pathlib import Path

from core import db

REPORTS_DIR = Path(__file__).resolve().parent.parent / "reports"

# Friendly names for Drive MIME types (detail keys are friendly already,
# this maps MIME values if a real collector passes them directly).
MIME_NAMES = {
    "application/vnd.google-apps.document": "Google Docs",
    "application/vnd.google-apps.spreadsheet": "Google Sheets",
    "application/vnd.google-apps.presentation": "Google Slides",
    "application/vnd.google-apps.form": "Google Forms",
    "application/vnd.google-apps.folder": "Folders",
}

SERVICE_TITLES = {
    "drive": "Drive",
    "gmail": "Gmail",
    "calendar": "Calendar",
    "contacts": "Contacts",
    "photos": "Photos",
    "tasks": "Tasks",
    "apps_script": "Apps Script",
}

# Relative migration effort per service, used in the impact matrix.
SERVICE_EFFORT = {
    "drive": "High: native formats (Docs, Sheets, Slides, Forms) need conversion; binaries are copy-only",
    "gmail": "High: separate migration project (coexistence, MX records, history)",
    "calendar": "Medium: calendars, rooms and resources migrate in their own phase",
    "contacts": "Low: standard import/export",
    "photos": "Medium: high volume, plain copy",
    "tasks": "Low: small volume, product decision",
    "apps_script": "High: scripts must be rewritten on the target platform; the integration list drives third-party reconnection",
}


def _fmt_bytes(n: float) -> str:
    for unit_name in ["B", "KB", "MB", "GB"]:
        if n < 1024 or unit_name == "GB":
            return f"{n:.1f} {unit_name}" if unit_name != "B" else f"{int(n)} B"
        n /= 1024
    return f"{n:.1f} GB"


def _latest_run() -> dict | None:
    with db.connect() as conn:
        row = conn.execute(
            "SELECT run_id, started_at, mode, status FROM runs "
            "WHERE status = 'completed' ORDER BY run_id DESC LIMIT 1"
        ).fetchone()
        return dict(row) if row else None


def _aggregates(run_id: int) -> dict[str, list[dict]]:
    grouped: dict[str, list[dict]] = {}
    with db.connect() as conn:
        rows = conn.execute(
            "SELECT service, metric, value, unit, detail "
            "FROM service_usage WHERE run_id = ? ORDER BY id", (run_id,)
        ).fetchall()
    for row in rows:
        d = dict(row)
        if d["detail"] in MIME_NAMES:
            d["detail"] = MIME_NAMES[d["detail"]]
        grouped.setdefault(d["service"], []).append(d)
    return grouped


def _metric(service_rows: list[dict], metric: str) -> dict | None:
    for row in service_rows:
        if row["metric"] == metric:
            return row
    return None


def _service_overview(service: str, rows: list[dict]) -> tuple[str, str, str]:
    """Return (in_use, volume, effort) for the impact matrix."""
    if not rows:
        return "No data", "-", SERVICE_EFFORT.get(service, "-")
    total_bytes = _metric(rows, "total_bytes")
    volume = _fmt_bytes(total_bytes["value"]) if total_bytes else "-"
    return "Yes", volume, SERVICE_EFFORT.get(service, "-")


def build_markdown(run: dict, grouped: dict[str, list[dict]]) -> str:
    lines: list[str] = []
    lines.append("# Google Inventory Report")
    lines.append("")
    lines.append(f"Run {run['run_id']} | mode: {run['mode']} | "
                 f"date: {run['started_at']}")
    lines.append("")
    lines.append("Aggregates only: no file names, e-mail addresses or content "
                 "are stored anywhere in this report or its source database.")
    lines.append("")

    # Impact matrix
    lines.append("## Migration impact matrix")
    lines.append("")
    lines.append("| Service | In use | Volume | Migration effort (relative) |")
    lines.append("|---|---|---|---|")
    for service in SERVICE_TITLES:
        in_use, volume, effort = _service_overview(service, grouped.get(service, []))
        lines.append(f"| {SERVICE_TITLES[service]} | {in_use} | {volume} | {effort} |")
    lines.append("| Keep, Chat | Verify manually | - | No public API: manual check |")
    lines.append("")

    # Per-service details
    lines.append("## Per-service detail")
    for service, rows in grouped.items():
        if not rows:
            continue
        lines.append("")
        lines.append(f"### {SERVICE_TITLES.get(service, service)}")
        lines.append("")
        lines.append("| Metric | Value | Detail |")
        lines.append("|---|---|---|")
        for row in rows:
            value = row["value"]
            if row["unit"] == "bytes":
                value_str = _fmt_bytes(value)
            elif value == int(value):
                value_str = f"{int(value):,}".replace(",", " ")
            else:
                value_str = str(value)
            lines.append(
                f"| {row['metric'].replace('_', ' ')} | {value_str} | "
                f"{row['detail'] or '-'} |"
            )
    lines.append("")
    return "\n".join(lines)


def build_csv_rows(run_id: int, grouped: dict[str, list[dict]]) -> list[list]:
    header = ["run_id", "service", "metric", "value", "unit", "detail"]
    rows = [header]
    for service, items in grouped.items():
        for item in items:
            rows.append([run_id, service, item["metric"], item["value"],
                         item["unit"] or "", item["detail"] or ""])
    return rows


def generate_report() -> None:
    run = _latest_run()
    if run is None:
        print("report: no completed run found. Run 'inventory.py run --demo' first.")
        return
    grouped = _aggregates(run["run_id"])

    REPORTS_DIR.mkdir(parents=True, exist_ok=True)
    md_path = REPORTS_DIR / "reporte.md"
    csv_path = REPORTS_DIR / "agregados.csv"

    md_path.write_text(build_markdown(run, grouped), encoding="utf-8")
    with open(csv_path, "w", newline="", encoding="utf-8") as f:
        csv.writer(f).writerows(build_csv_rows(run["run_id"], grouped))

    print(f"Report generated: {md_path}")
    print(f"Aggregates exported: {csv_path}")
