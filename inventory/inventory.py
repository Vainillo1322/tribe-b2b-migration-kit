#!/usr/bin/env python3
"""Google Inventory MVP - usage analyzer for a Google account.

Inventories which Google services an account uses and how much work
each one represents for migration (counts and sizes only).

Privacy by design: this tool stores ONLY aggregates. It never stores
file names, e-mail addresses, message content, or third-party IDs.
"""

import argparse
import sys

from core import db
from core.orchestrator import Orchestrator
from reporting.report import generate_report


def build_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(
        prog="inventory.py",
        description=(
            "Google Inventory MVP: inventories Google service usage of one "
            "account (aggregates only, no personal data)."
        ),
    )
    sub = parser.add_subparsers(dest="command", required=True)

    p_run = sub.add_parser(
        "run",
        help="Run the inventory (all collectors) and store aggregates.",
    )
    p_run.add_argument(
        "--demo",
        action="store_true",
        help="Run with simulated example data instead of the real account.",
    )

    sub.add_parser(
        "report",
        help="Generate reporte.md and agregados.csv from stored aggregates.",
    )

    p_auth = sub.add_parser(
        "auth",
        help="Run the OAuth consent flow and report per-service access.",
    )
    p_auth.add_argument(
        "--check",
        action="store_true",
        help="Verify setup only: libraries and credentials file (no browser).",
    )
    p_auth.add_argument(
        "--no-photos",
        action="store_true",
        help="Exclude the Photos scope (restricted scope; skip if consent fails).",
    )
    p_auth.add_argument(
        "--only",
        help=(
            "Request consent for specific services only, comma-separated "
            "(e.g. --only drive,gmail). Useful to isolate a blocked scope; "
            "previously granted scopes are kept."
        ),
    )

    return parser


def main(argv=None) -> int:
    parser = build_parser()
    args = parser.parse_args(argv)

    if args.command == "run":
        orchestrator = Orchestrator(demo=args.demo)
        run_id = orchestrator.run()
        print(f"Run {run_id} completed. Aggregates stored in data/inventory.db")
        return 0

    if args.command == "report":
        generate_report()
        return 0

    if args.command == "auth":
        from core.auth import check_setup, run_consent
        if args.check:
            return check_setup()
        only = [s.strip() for s in args.only.split(",")] if args.only else None
        return run_consent(only=only, include_photos=not args.no_photos)

    parser.print_help()
    return 1


if __name__ == "__main__":
    sys.exit(main())
