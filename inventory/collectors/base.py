"""Collector base class.

Contract: a collector returns a list of aggregates
(service, metric, value, unit, detail) and never any identifying data.
Collectors must be independent: an error in one must not stop the others.
"""

from dataclasses import dataclass


@dataclass
class Aggregate:
    service: str
    metric: str
    value: float
    unit: str | None = None      # 'count' | 'bytes'
    detail: str | None = None    # aggregation key, e.g. file type


class BaseCollector:
    service: str = "base"

    def collect(self) -> list[Aggregate]:
        raise NotImplementedError

    def collect_demo(self) -> list[Aggregate]:
        """Same contract as collect(), but with simulated example data."""
        raise NotImplementedError
