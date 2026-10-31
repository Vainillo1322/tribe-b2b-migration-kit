"""SQLite storage for aggregates only.

Schema rule (privacy by design): there is no column anywhere for file
names, e-mail addresses, message content or third-party identifiers.
Only service names, metric names, numeric values and run metadata.
"""

import sqlite3
from pathlib import Path

DB_PATH = Path(__file__).resolve().parent.parent / "data" / "inventory.db"

SCHEMA = """
CREATE TABLE IF NOT EXISTS runs (
    run_id      INTEGER PRIMARY KEY AUTOINCREMENT,
    started_at  TEXT NOT NULL,           -- ISO 8601 UTC
    finished_at TEXT,
    mode        TEXT NOT NULL,           -- 'demo' | 'live'
    status      TEXT NOT NULL            -- 'running' | 'completed' | 'failed'
);

CREATE TABLE IF NOT EXISTS service_usage (
    id      INTEGER PRIMARY KEY AUTOINCREMENT,
    run_id  INTEGER NOT NULL REFERENCES runs(run_id),
    service TEXT NOT NULL,               -- 'drive', 'gmail', ...
    metric  TEXT NOT NULL,               -- 'total_files', 'total_bytes', ...
    value   REAL NOT NULL,
    unit    TEXT,                        -- 'count' | 'bytes'
    detail  TEXT                         -- aggregation key, e.g. 'application/vnd.google-apps.document'
);

CREATE INDEX IF NOT EXISTS idx_service_usage_run ON service_usage(run_id);
"""


def connect() -> sqlite3.Connection:
    DB_PATH.parent.mkdir(parents=True, exist_ok=True)
    conn = sqlite3.connect(DB_PATH)
    conn.row_factory = sqlite3.Row
    return conn


def init_schema() -> None:
    with connect() as conn:
        conn.executescript(SCHEMA)


def start_run(mode: str) -> int:
    init_schema()
    with connect() as conn:
        cur = conn.execute(
            "INSERT INTO runs (started_at, mode, status) "
            "VALUES (strftime('%Y-%m-%dT%H:%M:%fZ','now'), ?, 'running')",
            (mode,),
        )
        return cur.lastrowid


def finish_run(run_id: int, status: str) -> None:
    with connect() as conn:
        conn.execute(
            "UPDATE runs SET finished_at = strftime('%Y-%m-%dT%H:%M:%fZ','now'), "
            "status = ? WHERE run_id = ?",
            (status, run_id),
        )


def write_aggregate(run_id: int, service: str, metric: str,
                    value: float, unit: str | None = None,
                    detail: str | None = None) -> None:
    with connect() as conn:
        conn.execute(
            "INSERT INTO service_usage (run_id, service, metric, value, unit, detail) "
            "VALUES (?, ?, ?, ?, ?, ?)",
            (run_id, service, metric, value, unit, detail),
        )
