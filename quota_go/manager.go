// Package quota provides storage quota management for OCS (OpenShift Container Storage)
// namespaces. It tracks per-namespace byte usage, enforces configurable GiB limits,
// and calculates utilisation percentages.
package quota

import (
	"fmt"
	"sync"
)

// ErrQuotaExceeded is returned when a namespace exceeds its configured storage limit.
type ErrQuotaExceeded struct {
	Namespace string
	UsedGiB   float64
	LimitGiB  float64
}

func (e *ErrQuotaExceeded) Error() string {
	return fmt.Sprintf("namespace %q exceeded quota: %.2f GiB used, limit is %.2f GiB",
		e.Namespace, e.UsedGiB, e.LimitGiB)
}

const gib = float64(1024 * 1024 * 1024)

// Manager tracks per-namespace OCS storage usage against a configured GiB limit.
type Manager struct {
	mu         sync.Mutex
	limitBytes int64
	limitGiB   float64
	usage      map[string]int64 // namespace -> bytes
}

// NewManager creates a Manager with the given GiB limit.
// It returns an error if limitGiB is not positive.
func NewManager(limitGiB float64) (*Manager, error) {
	if limitGiB <= 0 {
		return nil, fmt.Errorf("limitGiB must be positive, got %v", limitGiB)
	}
	return &Manager{
		limitBytes: int64(limitGiB * gib),
		limitGiB:   limitGiB,
		usage:      make(map[string]int64),
	}, nil
}

// BUG 1: AddUsage should accumulate bytes for a namespace across multiple calls,
// but it currently OVERWRITES the existing value with each new call.
// Fix: m.usage[namespace] += bytes
//
// AddUsage records gib GiB of additional storage used by namespace.
func (m *Manager) AddUsage(namespace string, gib float64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	bytes := int64(gib * float64(1024*1024*1024))
	m.usage[namespace] = bytes // BUG: overwrites, does not accumulate
}

// GetUsageGiB returns the recorded storage usage for namespace in GiB.
func (m *Manager) GetUsageGiB(namespace string) float64 {
	m.mu.Lock()
	defer m.mu.Unlock()
	return float64(m.usage[namespace]) / gib
}

// TotalUsageGiB returns the sum of all namespace usage in GiB.
func (m *Manager) TotalUsageGiB() float64 {
	m.mu.Lock()
	defer m.mu.Unlock()
	var total int64
	for _, b := range m.usage {
		total += b
	}
	return float64(total) / gib
}

// CheckQuota returns ErrQuotaExceeded if namespace has exceeded the limit.
func (m *Manager) CheckQuota(namespace string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	used := m.usage[namespace]
	if used > m.limitBytes {
		return &ErrQuotaExceeded{
			Namespace: namespace,
			UsedGiB:   float64(used) / gib,
			LimitGiB:  m.limitGiB,
		}
	}
	return nil
}

// BUG 2: UtilisationPct performs integer division (both operands are int64)
// so sub-1-percent values are truncated to 0 instead of the correct float.
// Fix: float64(total) / float64(m.limitBytes) * 100
//
// UtilisationPct returns overall utilisation as a percentage (0–100).
func (m *Manager) UtilisationPct() float64 {
	m.mu.Lock()
	defer m.mu.Unlock()
	var total int64
	for _, b := range m.usage {
		total += b
	}
	if m.limitBytes == 0 {
		return 0
	}
	return float64(total) / float64(m.limitBytes) * 100
}
