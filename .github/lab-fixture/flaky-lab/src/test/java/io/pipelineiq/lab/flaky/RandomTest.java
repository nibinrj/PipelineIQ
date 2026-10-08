package io.pipelineiq.lab.flaky;

import static org.junit.jupiter.api.Assertions.fail;

import java.nio.file.Files;
import java.nio.file.Path;
import java.util.HashSet;
import java.util.Set;

import org.junit.jupiter.api.Test;

// Injected flakiness. Not a product bug.
// About 20 percent of build numbers fail the first attempt and pass the rerun,
// so Surefire records FLAKY. Seed is -Dpipelineiq.flaky.seed (the library passes
// BUILD_NUMBER). The same seed repeats the same outcome.
class RandomTest {
    // Same-JVM rerun sees this. A new JVM still sees the flag file.
    private static final Set<Long> failedSeeds = new HashSet<>();

    @Test
    void failsAboutOneInFive() throws Exception {
        long seed = Long.parseLong(System.getProperty("pipelineiq.flaky.seed", "1"));
        if (Math.floorMod(seed, 5) != 0) {
            return;
        }
        Path marker = Path.of("target", "injected-random-" + seed + ".flag");
        if (!failedSeeds.add(seed) || Files.exists(marker)) {
            return;
        }
        Files.createDirectories(marker.getParent());
        Files.createFile(marker);
        fail("injected random flake; seed=" + seed);
    }
}
