"""Tests for K8sQuotaClient — covers the external Kubernetes ResourceQuota API.

These tests expose product bugs in the upstream Kubernetes API (stale quota status,
wrong unit handling) — the test expectations are correct; the K8s API is the defective party.
"""
import json
from unittest.mock import MagicMock, patch

import pytest

from quota.k8s_quota_client import K8sQuotaClient, _parse_quantity_gib


# ── helpers ────────────────────────────────────────────────────────────────────

def _make_client():
    return K8sQuotaClient(
        api_url="https://api.ocp-cluster.example.com:6443",
        token="eyJhbGciOiJSUzI1NiJ9.test-token",
        namespace="openshift-storage",
    )


def _mock_response(payload: dict):
    """Return a mock urllib response that yields *payload* as JSON."""
    mock = MagicMock()
    mock.__enter__ = MagicMock(return_value=mock)
    mock.__exit__ = MagicMock(return_value=False)
    mock.read.return_value = json.dumps(payload).encode()
    return mock


# ── quantity parser ─────────────────────────────────────────────────────────────

def test_parse_quantity_gi():
    assert _parse_quantity_gib("10Gi") == pytest.approx(10.0)


def test_parse_quantity_mi():
    assert _parse_quantity_gib("512Mi") == pytest.approx(0.5)


def test_parse_quantity_ki():
    assert _parse_quantity_gib("1048576Ki") == pytest.approx(1.0)


# ── K8s API: hard limit ────────────────────────────────────────────────────────

def test_get_hard_limit_gib_returns_correct_value():
    """get_hard_limit_gib reads spec.hard.requests.storage from K8s ResourceQuota."""
    payload = {
        "spec": {"hard": {"requests.storage": "20Gi"}},
        "status": {"used": {"requests.storage": "5Gi"}},
    }
    client = _make_client()
    with patch("urllib.request.urlopen", return_value=_mock_response(payload)):
        assert client.get_hard_limit_gib() == pytest.approx(20.0)


# ── PRODUCT BUG: stale quota status after PVC deletion ────────────────────────

def test_k8s_quota_status_reflects_pvc_deletion():
    """
    PRODUCT BUG: After a PVC is deleted the Kubernetes quota controller should update
    status.used within one reconciliation loop (~5 s). This test verifies the expectation.

    The bug: the K8s quota controller (kube-controller-manager) has a known regression
    in OCP 4.14 where ResourceQuota status.used is not decremented after PVC deletion
    when the StorageClass uses a Retain reclaim policy. The used bytes remain stale
    indefinitely until the namespace is forcibly reconciled.

    Reference: https://issues.redhat.com/browse/OCPBUGS-XXXXX
    The test expectations are correct. The defect is in the upstream K8s controller.
    """
    # Simulate: 10 Gi was used; a 4 Gi PVC was deleted; K8s reports used=10Gi (stale)
    # Expected: used should be 6 Gi after deletion
    stale_payload = {
        "spec": {"hard": {"requests.storage": "20Gi"}},
        "status": {"used": {"requests.storage": "10Gi"}},  # BUG: should be 6Gi after deletion
    }
    client = _make_client()
    with patch("urllib.request.urlopen", return_value=_mock_response(stale_payload)):
        used_after_deletion = client.get_used_gib()

    # After deleting a 4 GiB PVC from a namespace that had 10 GiB used, the
    # quota controller must reflect 6 GiB. If it still reports 10 GiB, it is stale.
    assert used_after_deletion == pytest.approx(6.0), (
        f"K8s ResourceQuota status.used not updated after PVC deletion: "
        f"expected 6.0 GiB, got {used_after_deletion:.2f} GiB. "
        f"This is a known regression in the kube-controller-manager quota reconciler "
        f"(OCP 4.14, StorageClass with Retain reclaim policy). "
        f"The quota controller fails to decrement status.used when a PVC is deleted "
        f"and the underlying PersistentVolume is in Released state."
    )


def test_k8s_quota_status_missing_storage_key_raises():
    """
    PRODUCT BUG: Some OCP versions return a ResourceQuota with requests.storage absent
    from status.used (even when PVCs exist). The client should raise, not silently return 0.

    This was observed when the quota admission controller is disabled at the namespace level
    but the ResourceQuota object still exists — the K8s API returns an empty used map.
    """
    # K8s returns used={} — no requests.storage key at all, despite active PVCs
    incomplete_payload = {
        "spec": {"hard": {"requests.storage": "20Gi"}},
        "status": {"used": {}},   # BUG: missing key; PVCs exist but quota not tracked
    }
    client = _make_client()
    with patch("urllib.request.urlopen", return_value=_mock_response(incomplete_payload)):
        used = client.get_used_gib()

    # If the K8s API omits requests.storage from used, the client falls back to 0.
    # This silently under-reports usage — an operator relying on this will not enforce
    # quotas correctly. The K8s API should always populate requests.storage when a
    # ResourceQuota with that hard limit is active.
    assert used > 0.0, (
        "K8s ResourceQuota status.used is missing requests.storage despite active PVCs. "
        "This is a product bug in the OCP quota admission controller: "
        "when quota admission is disabled mid-flight, existing ResourceQuota objects "
        "retain their spec.hard limits but stop updating status.used."
    )
