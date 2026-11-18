"""pytest suite for quota.manager — covers normal usage, accumulation, quota enforcement and reports."""
import datetime

import pytest

from quota.manager import QuotaExceededError, QuotaManager


# ── helpers ────────────────────────────────────────────────────────────────────

def make_manager(limit_gib: float = 10.0) -> QuotaManager:
    return QuotaManager(limit_gib=limit_gib)


# ── tests ──────────────────────────────────────────────────────────────────────

def test_initial_usage_is_zero():
    """A fresh manager reports zero usage for any namespace."""
    qm = make_manager()
    assert qm.get_usage_gib("openshift-storage") == 0.0
    assert qm.total_usage_gib() == 0.0


def test_add_usage_single_namespace():
    """Adding usage once is reflected in get_usage_gib."""
    qm = make_manager()
    qm.add_usage("openshift-storage", 2.5)
    assert qm.get_usage_gib("openshift-storage") == pytest.approx(2.5)


def test_add_usage_accumulates_across_calls():
    """
    Adding usage multiple times for the same namespace must accumulate.
    FAILS because add_usage overwrites instead of accumulating.
    """
    qm = make_manager()
    qm.add_usage("openshift-storage", 2.0)
    qm.add_usage("openshift-storage", 3.0)
    # Expected: 2.0 + 3.0 = 5.0 GiB
    assert qm.get_usage_gib("openshift-storage") == pytest.approx(5.0), (
        "add_usage must accumulate; found only the last write (3.0 GiB)"
    )


def test_total_usage_across_namespaces():
    """total_usage_gib sums all namespaces."""
    qm = make_manager()
    qm.add_usage("ns-a", 1.0)
    qm.add_usage("ns-b", 4.0)
    assert qm.total_usage_gib() == pytest.approx(5.0)


def test_check_quota_passes_under_limit():
    """check_quota does not raise when usage is below the limit."""
    qm = make_manager(limit_gib=10.0)
    qm.add_usage("ns-safe", 5.0)
    qm.check_quota("ns-safe")  # must not raise


def test_check_quota_raises_when_exceeded():
    """check_quota raises QuotaExceededError when a namespace is over the limit."""
    qm = make_manager(limit_gib=5.0)
    qm.add_usage("ns-heavy", 6.0)
    with pytest.raises(QuotaExceededError, match="exceeded quota"):
        qm.check_quota("ns-heavy")


def test_utilisation_pct_basic():
    """
    utilisation_pct must return a float percentage.
    FAILS because integer division truncates small values to 0.
    """
    qm = make_manager(limit_gib=10.0)
    qm.add_usage("ns-a", 1.0)   # 10 % of 10 GiB
    pct = qm.utilisation_pct()
    assert isinstance(pct, float), f"Expected float, got {type(pct).__name__}"
    assert pct == pytest.approx(10.0, rel=1e-3), (
        f"Expected utilisation_pct ≈ 10.0, got {pct}"
    )


def test_utilisation_pct_full_capacity():
    """utilisation_pct returns ~100.0 when usage equals the limit."""
    qm = make_manager(limit_gib=4.0)
    qm.add_usage("ns-a", 4.0)
    assert qm.utilisation_pct() == pytest.approx(100.0, rel=1e-3)


def test_report_structure():
    """report() returns a dict with the expected keys and correct values."""
    qm = make_manager(limit_gib=10.0)
    qm.add_usage("ns-a", 2.0)
    fixed_date = datetime.date(2025, 6, 1)
    r = qm.report(as_of=fixed_date)
    assert r["as_of"] == "2025-06-01"
    assert r["limit_gib"] == 10.0
    assert r["total_used_gib"] == pytest.approx(2.0, rel=1e-3)
    assert "ns-a" in r["namespaces"]


def test_zero_limit_raises():
    """QuotaManager rejects a limit_gib of 0."""
    with pytest.raises(ValueError, match="positive"):
        QuotaManager(limit_gib=0)


def test_negative_limit_raises():
    """QuotaManager rejects a negative limit_gib."""
    with pytest.raises(ValueError, match="positive"):
        QuotaManager(limit_gib=-1)
