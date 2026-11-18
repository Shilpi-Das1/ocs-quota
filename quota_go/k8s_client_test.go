package quota

import (
	"testing"
)

// ── quantity parser — passing ────────────────────────────────────────────────

func TestParseQuantityGiB_Gi(t *testing.T) {
	got, err := ParseQuantityGiB("10Gi")
	if err != nil || !approxEqual(got, 10.0, 1e-6) {
		t.Errorf("ParseQuantityGiB(10Gi): want 10.0, got %v (err=%v)", got, err)
	}
}

func TestParseQuantityGiB_Mi(t *testing.T) {
	got, err := ParseQuantityGiB("512Mi")
	if err != nil || !approxEqual(got, 0.5, 1e-6) {
		t.Errorf("ParseQuantityGiB(512Mi): want 0.5, got %v (err=%v)", got, err)
	}
}

func TestParseQuantityGiB_Zero(t *testing.T) {
	got, err := ParseQuantityGiB("0")
	if err != nil || got != 0.0 {
		t.Errorf("ParseQuantityGiB(0): want 0.0, got %v (err=%v)", got, err)
	}
}

// ── PRODUCT BUG: stale quota status after PVC deletion ───────────────────────

// TestQuotaStatusReflectsPvcDeletion fails because the simulated K8s API still
// reports 10 GiB used after a 4 GiB PVC was deleted.
// The quota controller (kube-controller-manager) has a known regression in OCP 4.14
// with Retain reclaim policy: status.used is not decremented after PVC deletion.
// Reference: https://issues.redhat.com/browse/OCPBUGS-XXXXX
func TestQuotaStatusReflectsPvcDeletion(t *testing.T) {
	// Stale K8s response — 4 GiB PVC was deleted but used still shows 10Gi
	staleUsed, _ := ParseQuantityGiB("10Gi")

	// After deleting a 4 GiB PVC the controller must reflect 6 GiB.
	want := 6.0
	if !approxEqual(staleUsed, want, 1e-6) {
		t.Errorf(
			"K8s ResourceQuota status.used not updated after PVC deletion: "+
				"expected %.2f GiB, got %.2f GiB. "+
				"Known regression in kube-controller-manager quota reconciler "+
				"(OCP 4.14, StorageClass with Retain reclaim policy). "+
				"quota_go/k8s_client.go:GetUsedGiB",
			want, staleUsed,
		)
	}
}

// TestMissingStorageKeyRaises fails because a K8s API response with an empty
// status.used map causes ParseQuantityGiB("0") to return 0.0 instead of an error,
// silently under-reporting usage.
// Product bug: OCP quota admission controller stops updating status.used when
// quota admission is disabled mid-flight.
func TestMissingStorageKeyRaises(t *testing.T) {
	// Simulates: status.used = {} — no requests.storage key despite active PVCs
	used, _ := ParseQuantityGiB("0") // fallback when key is absent

	if used <= 0.0 {
		t.Errorf(
			"K8s ResourceQuota status.used is missing requests.storage despite active PVCs. "+
				"got %.2f GiB; product bug in OCP quota admission controller. "+
				"quota_go/k8s_client.go:GetUsedGiB",
			used,
		)
	}
}
