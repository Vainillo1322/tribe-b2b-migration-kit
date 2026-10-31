"""Orchestrator: runs all collectors, isolates failures, stores aggregates."""

from collectors import ALL_COLLECTORS
from core import db


class Orchestrator:
    def __init__(self, demo: bool = False):
        self.demo = demo

    def run(self) -> int:
        mode = "demo" if self.demo else "live"
        run_id = db.start_run(mode)
        results = []

        for collector_cls in ALL_COLLECTORS:
            name = collector_cls.service
            try:
                collector = collector_cls()
                if self.demo:
                    aggregates = collector.collect_demo()
                else:
                    aggregates = collector.collect()
                results.append((name, "ok", len(aggregates)))
                for agg in aggregates:
                    db.write_aggregate(run_id, agg.service, agg.metric,
                                       agg.value, agg.unit, agg.detail)
            except NotImplementedError:
                results.append((name, "not implemented", 0))
            except Exception as exc:  # noqa: BLE001 - collectors must not crash the run
                results.append((name, f"error: {exc}", 0))

        db.finish_run(run_id, "completed")
        self._print_summary(results)
        return run_id

    @staticmethod
    def _print_summary(results) -> None:
        print("\nCollector summary")
        print("-" * 40)
        for name, status, n in results:
            print(f"  {name:<12} {status:<20} aggregates: {n}")
