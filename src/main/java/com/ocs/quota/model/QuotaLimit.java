package com.ocs.quota.model;

/**
 * Immutable value object representing a storage quota limit for one namespace.
 */
public final class QuotaLimit {

    private final String namespace;
    private final double limitGib;
    private final double usedGib;

    public QuotaLimit(String namespace, double limitGib, double usedGib) {
        if (namespace == null || namespace.isBlank()) {
            throw new IllegalArgumentException("namespace must not be blank");
        }
        if (limitGib <= 0) {
            throw new IllegalArgumentException("limitGib must be positive, got: " + limitGib);
        }
        if (usedGib < 0) {
            throw new IllegalArgumentException("usedGib must not be negative, got: " + usedGib);
        }
        this.namespace = namespace;
        this.limitGib  = limitGib;
        this.usedGib   = usedGib;
    }

    public String getNamespace() { return namespace; }
    public double getLimitGib()  { return limitGib; }
    public double getUsedGib()   { return usedGib; }

    /** Remaining headroom in GiB (never negative). */
    public double remainingGib() {
        return Math.max(0.0, limitGib - usedGib);
    }

    /** Utilisation as a percentage (0–100). */
    public double utilisationPct() {
        return (usedGib / limitGib) * 100.0;
    }

    @Override
    public String toString() {
        return String.format("QuotaLimit{ns='%s', used=%.2fGi, limit=%.2fGi (%.1f%%)}",
                namespace, usedGib, limitGib, utilisationPct());
    }
}
