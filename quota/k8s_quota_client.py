"""Thin wrapper around the Kubernetes ResourceQuota API.

Calls the K8s API server to read the applied quota status for a namespace.
This module depends on the external Kubernetes API — bugs here are product bugs
(the K8s API returning unexpected data), not bugs in this codebase.
"""
from __future__ import annotations

import json
import urllib.request
import urllib.error


class K8sQuotaClient:
    """Read ResourceQuota objects from the Kubernetes API server."""

    def __init__(self, api_url: str, token: str, namespace: str):
        self.api_url = api_url.rstrip("/")
        self.token = token
        self.namespace = namespace

    def _get(self, path: str) -> dict:
        url = f"{self.api_url}{path}"
        req = urllib.request.Request(url, headers={"Authorization": f"Bearer {self.token}"})
        try:
            with urllib.request.urlopen(req, timeout=10) as resp:
                return json.loads(resp.read())
        except urllib.error.HTTPError as exc:
            raise RuntimeError(
                f"K8s API returned HTTP {exc.code} for {path}: {exc.reason}"
            ) from exc

    def get_hard_limit_gib(self, quota_name: str = "storage-quota") -> float:
        """Return the hard storage limit (requests.storage) in GiB from K8s ResourceQuota."""
        path = f"/api/v1/namespaces/{self.namespace}/resourcequotas/{quota_name}"
        data = self._get(path)
        raw = data["spec"]["hard"].get("requests.storage", "0")
        return _parse_quantity_gib(raw)

    def get_used_gib(self, quota_name: str = "storage-quota") -> float:
        """Return the currently used storage in GiB from K8s ResourceQuota status."""
        path = f"/api/v1/namespaces/{self.namespace}/resourcequotas/{quota_name}"
        data = self._get(path)
        raw = data["status"]["used"].get("requests.storage", "0")
        return _parse_quantity_gib(raw)


def _parse_quantity_gib(quantity: str) -> float:
    """Parse a Kubernetes quantity string (e.g. '10Gi', '500Mi', '1024Ki') to GiB."""
    quantity = quantity.strip()
    if quantity.endswith("Gi"):
        return float(quantity[:-2])
    if quantity.endswith("Mi"):
        return float(quantity[:-2]) / 1024
    if quantity.endswith("Ki"):
        return float(quantity[:-2]) / (1024 ** 2)
    if quantity.endswith("G"):
        return float(quantity[:-1]) * (1000 ** 3) / (1024 ** 3)
    return float(quantity) / (1024 ** 3)
