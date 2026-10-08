package io.pipelineiq.lab.flaky;

import static org.junit.jupiter.api.Assertions.fail;

import org.junit.jupiter.api.Test;

// Injected flakiness. Not a product bug.
// Fails when the UTC minute modulo 5 is 0, about 20 percent of minutes.
// -Dpipelineiq.flaky.epochSecond replays a chosen second. Without it, the clock is wall time.
class TimeTest {
    @Test
    void failsOnSomeMinutes() {
        long epoch = Long.getLong("pipelineiq.flaky.epochSecond", System.currentTimeMillis() / 1000);
        long minute = Math.floorDiv(epoch, 60);
        if (Math.floorMod(minute, 5) == 0) {
            fail("injected time-based flake; epochSecond=" + epoch);
        }
    }
}
