package quota

import (
	"math"
	"testing"
)

// approxEqual returns true if a and b are within tol of each other.
func approxEqual(a, b, tol float64) bool {
	return math.Abs(a-b) <= tol
}

// ── passing tests ────────────────────────────────────────────────────────────

func TestNewManagerRejectsZeroLimit(t *testing.T) {
	_, err := NewManager(0)
	if err == nil {
		t.Fatal("expected error for zero limitGiB, got nil")
	}
}

func TestNewManagerRejectsNegativeLimit(t *testing.T) {
	_, err := NewManager(-1.5)
	if err == nil {
		t.Fatal("expected error for negative limitGiB, got nil")
	}
}

func TestInitialUsageIsZero(t *testing.T) {
	m, _ := NewManager(10.0)
	if got := m.GetUsageGiB("openshift-storage"); got != 0.0 {
		t.Errorf("initial usage: want 0.0, got %v", got)
	}
	if got := m.TotalUsageGiB(); got != 0.0 {
		t.Errorf("initial total: want 0.0, got %v", got)
	}
}

func TestAddUsageSingleNamespace(t *testing.T) {
	m, _ := NewManager(10.0)
	m.AddUsage("openshift-storage", 2.5)
	if got := m.GetUsageGiB("openshift-storage"); !approxEqual(got, 2.5, 1e-6) {
		t.Errorf("single addUsage: want 2.5 GiB, got %v", got)
	}
}

func TestTotalUsageAcrossNamespaces(t *testing.T) {
	m, _ := NewManager(10.0)
	m.AddUsage("ns-a", 1.0)
	m.AddUsage("ns-b", 4.0)
	if got := m.TotalUsageGiB(); !approxEqual(got, 5.0, 1e-6) {
		t.Errorf("total usage: want 5.0 GiB, got %v", got)
	}
}

func TestCheckQuotaPassesUnderLimit(t *testing.T) {
	m, _ := NewManager(10.0)
	m.AddUsage("ns-safe", 5.0)
	if err := m.CheckQuota("ns-safe"); err != nil {
		t.Errorf("checkQuota below limit: unexpected error %v", err)
	}
}

func TestCheckQuotaRaisesWhenExceeded(t *testing.T) {
	m, _ := NewManager(5.0)
	m.AddUsage("ns-heavy", 6.0)
	err := m.CheckQuota("ns-heavy")
	if err == nil {
		t.Fatal("expected ErrQuotaExceeded, got nil")
	}
	if _, ok := err.(*ErrQuotaExceeded); !ok {
		t.Errorf("expected *ErrQuotaExceeded, got %T: %v", err, err)
	}
}

func TestUtilisationPctAtFullCapacity(t *testing.T) {
	m, _ := NewManager(4.0)
	m.AddUsage("ns-a", 4.0)
	if got := m.UtilisationPct(); !approxEqual(got, 100.0, 1e-3) {
		t.Errorf("full-capacity utilisation: want 100.0, got %v", got)
	}
}

// ── FAILING tests (intentional bugs in production code) ──────────────────────

// TestAddUsageAccumulatesAcrossCalls fails because AddUsage overwrites instead
// of accumulating the byte count.
// Fix in manager.go: m.usage[namespace] += bytes
func TestAddUsageAccumulatesAcrossCalls(t *testing.T) {
	m, _ := NewManager(10.0)
	m.AddUsage("openshift-storage", 2.0)
	m.AddUsage("openshift-storage", 3.0)
	// 2.0 + 3.0 = 5.0 GiB expected; the bug returns 3.0
	got := m.GetUsageGiB("openshift-storage")
	if !approxEqual(got, 5.0, 1e-6) {
		t.Errorf("addUsage must accumulate: want 5.0 GiB, got %v GiB (only the last write was kept)", got)
	}
}

// TestUtilisationPctReturnsFloat fails because integer division in UtilisationPct
// truncates small percentages to 0.
// Fix in manager.go: float64(total) / float64(m.limitBytes) * 100
func TestUtilisationPctReturnsFloat(t *testing.T) {
	m, _ := NewManager(10.0)
	m.AddUsage("ns-a", 1.0) // 10 % of 10 GiB
	got := m.UtilisationPct()
	if !approxEqual(got, 10.0, 1e-3) {
		t.Errorf("utilisationPct: want ~10.0, got %v (integer division truncated result)", got)
	}
}
