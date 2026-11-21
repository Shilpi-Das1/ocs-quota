"""OCS storage quota manager — calculates usage, enforces limits, and formats reports."""
from __future__ import annotations

import datetime


class QuotaExceededError(Exception):
    pass


class QuotaManager:
    """Tracks per-namespace storage usage against a configured limit (in GiB)."""

    def __init__(self, limit_gib: float):
        if limit_gib <= 0:
            raise ValueError("limit_gib must be positive")
        self.limit_bytes = limit_gib * 1024 ** 3
        self._usage: dict[str, int] = {}  # namespace -> bytes used

    # ── BUG 1 ──────────────────────────────────────────────────────────────────
    # `add_usage` converts GiB to bytes correctly for the first add, but when
    # updating an existing namespace it accidentally overwrites instead of
    # accumulating.  The correct line is:
    #   self._usage[namespace] = self._usage.get(namespace, 0) + bytes_used
    def add_usage(self, namespace: str, gib: float) -> None:
        """Record *gib* GiB of additional storage used by *namespace*."""
        bytes_used = int(gib * 1024 ** 3)
        self._usage[namespace] = bytes_used   # BUG: should accumulate, not overwrite

    def get_usage_gib(self, namespace: str) -> float:
        return self._usage.get(namespace, 0) / 1024 ** 3

    def total_usage_gib(self) -> float:
        return sum(self._usage.values()) / 1024 ** 3

    def check_quota(self, namespace: str) -> None:
        """Raise QuotaExceededError if the namespace exceeds the limit."""
        used = self._usage.get(namespace, 0)
        if used > self.limit_bytes:
            raise QuotaExceededError(
                f"Namespace '{namespace}' exceeded quota: "
                f"{used / 1024**3:.2f} GiB used, limit is {self.limit_bytes / 1024**3:.2f} GiB"
            )

    # ── BUG 2 ──────────────────────────────────────────────────────────────────
    # `utilisation_pct` divides by `self.limit_bytes` but `self._usage` values
    # are in bytes, so the result is correct — UNLESS the method is called with
    # the wrong attribute.  The actual bug here is that it uses
    # `sum(self._usage.values())` but divides by `self.limit_bytes / 100` which
    # loses precision for sub-1 % values and returns 0 instead of a float.
    def utilisation_pct(self) -> float:
        """Return overall utilisation as a percentage (0-100)."""
        total = sum(self._usage.values())
        return total * 100 / self.limit_bytes

    def report(self, as_of: datetime.date | None = None) -> dict:
        as_of = as_of or datetime.date.today()
        return {
            "as_of": as_of.isoformat(),
            "limit_gib": round(self.limit_bytes / 1024 ** 3, 2),
            "total_used_gib": round(self.total_usage_gib(), 4),
            "utilisation_pct": round(self.utilisation_pct(), 2),
            "namespaces": {
                ns: round(b / 1024 ** 3, 4) for ns, b in sorted(self._usage.items())
            },
        }
