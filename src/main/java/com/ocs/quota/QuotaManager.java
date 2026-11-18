package com.ocs.quota;

import com.ocs.quota.model.QuotaLimit;

import java.util.HashMap;
import java.util.Map;

/**
 * Tracks per-namespace OCS storage usage against a configured limit.
 *
 * <p>Usage is accumulated in bytes internally and exposed in GiB for caller convenience.
 */
public class QuotaManager {

    private final double limitGib;
    private final long   limitBytes;

    /** namespace → bytes used */
    private final Map<String, Long> usageBytes = new HashMap<>();

    public QuotaManager(double limitGib) {
        if (limitGib <= 0) {
            throw new IllegalArgumentException("limitGib must be positive, got: " + limitGib);
        }
        this.limitGib   = limitGib;
        this.limitBytes = (long) (limitGib * 1024L * 1024L * 1024L);
    }

    // ── BUG 1 ──────────────────────────────────────────────────────────────────
    // addUsage should ACCUMULATE bytes for a namespace across multiple calls, but
    // it currently OVERWRITES the existing value with each new call.
    // Fix: usageBytes.merge(namespace, bytes, Long::sum)
    /**
     * Record {@code gib} GiB of additional storage used by {@code namespace}.
     */
    public void addUsage(String namespace, double gib) {
        long bytes = (long) (gib * 1024L * 1024L * 1024L);
        usageBytes.put(namespace, bytes);           // BUG: overwrites, does not accumulate
    }

    public double getUsageGib(String namespace) {
        return usageBytes.getOrDefault(namespace, 0L) / (double) (1024L * 1024L * 1024L);
    }

    public double totalUsageGib() {
        return usageBytes.values().stream().mapToLong(Long::longValue).sum()
                / (double) (1024L * 1024L * 1024L);
    }

    /**
     * Raise {@link QuotaExceededException} if {@code namespace} has exceeded the limit.
     */
    public void checkQuota(String namespace) {
        long used = usageBytes.getOrDefault(namespace, 0L);
        if (used > limitBytes) {
            throw new QuotaExceededException(String.format(
                    "Namespace '%s' exceeded quota: %.2f GiB used, limit is %.2f GiB",
                    namespace,
                    used / (double) (1024L * 1024L * 1024L),
                    limitGib));
        }
    }

    // ── BUG 2 ──────────────────────────────────────────────────────────────────
    // utilisationPct() performs integer division because both operands are long.
    // For small usage values (<1 %) this returns 0 instead of the correct float.
    // Fix: cast numerator to double before division.
    /**
     * Return overall utilisation as a percentage (0–100).
     */
    public double utilisationPct() {
        long totalUsed = usageBytes.values().stream().mapToLong(Long::longValue).sum();
        return totalUsed / limitBytes * 100;        // BUG: integer division truncates
    }

    public QuotaLimit snapshot(String namespace) {
        return new QuotaLimit(namespace, limitGib, getUsageGib(namespace));
    }
}
