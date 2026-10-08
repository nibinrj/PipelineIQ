package io.pipelineiq.lab.flaky;

import java.util.Collections;

import org.junit.jupiter.api.MethodDescriptor;
import org.junit.jupiter.api.MethodOrderer;
import org.junit.jupiter.api.MethodOrdererContext;

// Injected. Orders methods by name hash XOR the order seed, so the same seed
// always runs the methods in the same order and a different seed can reverse it.
class SeedOrderer implements MethodOrderer {
    @Override
    public void orderMethods(MethodOrdererContext context) {
        long seed = Long.parseLong(System.getProperty("pipelineiq.flaky.orderSeed", "1"));
        Collections.sort(context.getMethodDescriptors(), (left, right) -> Integer.compare(key(left, seed), key(right, seed)));
    }

    private static int key(MethodDescriptor method, long seed) {
        // 32-bit xor. Seed 1 runs partner first. Seed 2147483648 runs the dependent method first.
        return method.getMethod().getName().hashCode() ^ (int) seed;
    }
}
