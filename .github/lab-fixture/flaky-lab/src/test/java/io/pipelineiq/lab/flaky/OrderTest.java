package io.pipelineiq.lab.flaky;

import static org.junit.jupiter.api.Assertions.fail;

import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.TestMethodOrder;

// Injected flakiness. Not a product bug.
// dependsOnPartner fails unless partner already ran in this JVM.
// -Dpipelineiq.flaky.orderSeed=1 runs partner first. Seed 2147483648 runs the
// dependent method first. The same seed repeats the same order.
@TestMethodOrder(SeedOrderer.class)
class OrderTest {
    private static boolean partnerRan;

    @Test
    void partner() {
        partnerRan = true;
    }

    @Test
    void dependsOnPartner() {
        if (!partnerRan) {
            fail("injected order dependence: partner did not run first");
        }
    }
}
