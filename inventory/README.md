# Google Inventory MVP

Usage analyzer for a Google account, built for the Workspace migration
project. It inventories **which Google services an account uses and how
much work each one represents for migration** — counts, sizes and date
ranges only.

## Privacy by design (non-negotiable)

The tool stores **only aggregates**. The database schema has no columns
for file names, e-mail addresses, message content, photos, event details
or third-party identifiers, so collectors cannot store them. Nothing is
sent to any server: everything runs locally and lives in
`data/inventory.db` plus the generated reports.

## Project structure

```
google-inventory-mvp/
├── inventory.py          # CLI entry point (run / report / auth)
├── core/
│   ├── db.py             # SQLite schema: runs, service_usage (aggregates only)
│   └── orchestrator.py   # Runs all collectors, isolates failures
├── collectors/           # One module per service (Drive, Gmail, Calendar,
│                         #  Contacts, Photos, Tasks, Apps Script)
├── reporting/
│   └── report.py         # Generates reporte.md and agregados.csv
└── data/                 # Created at runtime; SQLite database lives here
```

## Commands

```
python3 inventory.py run --demo   # Run with simulated example data (step 1.2)
python3 inventory.py run          # Run against the real account (Phase 2+)
python3 inventory.py report       # Generate reports from stored aggregates
python3 inventory.py auth         # OAuth consent flow (Phase 2)
```

## Current status

- Phase 1, step 1.1: project skeleton, SQLite schema, CLI, collector
  stubs, orchestrator. **Validated.**
- Phase 1, step 1.2: demo data mode + report generator
  (`run --demo`, `report` -> reports/reporte.md, reports/agregados.csv).
  **Validated.**
- Phase 2, step 2.1: `auth` command implemented (`--check` gate +
  consent flow with per-service access report). **In progress**:
  waiting for credentials created per docs/credentials-setup.md.
