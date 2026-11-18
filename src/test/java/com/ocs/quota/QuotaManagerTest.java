package com.ocs.quota;

import org.junit.jupiter.api.Test;
import static org.junit.jupiter.api.Assertions.*;

/**
 * JUnit 5 tests for {@link QuotaManager}.
 *
 * Two tests are intentionally failing to demonstrate bugs in the production code:
 * <ul>
 *   <li>{@code testAddUsageAccumulatesAcrossCalls} — addUsage overwrites instead of accumulating</li>
 *   <li>{@code testUtilisationPctReturnsFloat} — integer division truncates small percentages to 0</li>
 * </ul>
 */
class QuotaManagerTest {

    private QuotaManager makeManager() {
        return new QuotaManager(10.0);
    }

    // ── passing tests ───────────────────────────────────────────────────────────

    @Test
    void testInitialUsageIsZero() {
        QuotaManager qm = makeManager();
        assertEquals(0.0, qm.getUsageGib("openshift-storage"), 1e-9,
                "A fresh manager must report zero usage");
        assertEquals(0.0, qm.totalUsageGib(), 1e-9);
    }

    @Test
    void testAddUsageSingleNamespace() {
        QuotaManager qm = makeManager();
        qm.addUsage("openshift-storage", 2.5);
        assertEquals(2.5, qm.getUsageGib("openshift-storage"), 1e-6,
                "Single addUsage call must be reflected in getUsageGib");
    }

    @Test
    void testTotalUsageAcrossNamespaces() {
        QuotaManager qm = makeManager();
        qm.addUsage("ns-a", 1.0);
        qm.addUsage("ns-b", 4.0);
        assertEquals(5.0, qm.totalUsageGib(), 1e-6,
                "totalUsageGib must sum all namespaces");
    }

    @Test
    void testCheckQuotaPassesUnderLimit() {
        QuotaManager qm = makeManager();
        qm.addUsage("ns-safe", 5.0);
        assertDoesNotThrow(() -> qm.checkQuota("ns-safe"),
                "checkQuota must not throw when usage is below the limit");
    }

    @Test
    void testCheckQuotaRaisesWhenExceeded() {
        QuotaManager qm = new QuotaManager(5.0);
        qm.addUsage("ns-heavy", 6.0);
        QuotaExceededException ex = assertThrows(QuotaExceededException.class,
                () -> qm.checkQuota("ns-heavy"));
        assertTrue(ex.getMessage().contains("exceeded quota"),
                "Exception message must mention 'exceeded quota'");
    }

    @Test
    void testUtilisationPctAtFullCapacity() {
        QuotaManager qm = new QuotaManager(4.0);
        qm.addUsage("ns-a", 4.0);
        assertEquals(100.0, qm.utilisationPct(), 1e-3,
                "Full-capacity utilisation must be ~100 %");
    }

    @Test
    void testNegativeLimitThrows() {
        assertThrows(IllegalArgumentException.class, () -> new QuotaManager(-1.0),
                "Negative limitGib must throw IllegalArgumentException");
    }

    @Test
    void testZeroLimitThrows() {
        assertThrows(IllegalArgumentException.class, () -> new QuotaManager(0.0),
                "Zero limitGib must throw IllegalArgumentException");
    }

    // ── FAILING tests (intentional bugs in production code) ────────────────────

    /**
     * FAILS: addUsage overwrites the existing byte count instead of accumulating it.
     * Fix in QuotaManager: use {@code usageBytes.merge(namespace, bytes, Long::sum)}.
     */
    @Test
    void testAddUsageAccumulatesAcrossCalls() {
        QuotaManager qm = makeManager();
        qm.addUsage("openshift-storage", 2.0);
        qm.addUsage("openshift-storage", 3.0);
        // 2.0 + 3.0 = 5.0 GiB expected; the bug returns 3.0
        assertEquals(5.0, qm.getUsageGib("openshift-storage"), 1e-6,
                "addUsage must accumulate; got only the last write (3.0 GiB)");
    }

    /**
     * FAILS: integer division in utilisationPct() truncates small values to 0.
     * Fix in QuotaManager: cast to double before division.
     */
    @Test
    void testUtilisationPctReturnsFloat() {
        QuotaManager qm = new QuotaManager(10.0);
        qm.addUsage("ns-a", 1.0);   // 10 % of 10 GiB
        double pct = qm.utilisationPct();
        assertEquals(10.0, pct, 1e-3,
                "utilisationPct must return 10.0 for 1 GiB used out of 10 GiB limit; got " + pct);
    }
}
