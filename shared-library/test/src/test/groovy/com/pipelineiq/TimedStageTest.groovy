package com.pipelineiq

import com.lesfurets.jenkins.unit.BasePipelineTest
import org.junit.jupiter.api.BeforeEach
import org.junit.jupiter.api.Test

import static org.junit.jupiter.api.Assertions.assertThrows
import static org.junit.jupiter.api.Assertions.assertTrue

class TimedStageTest extends BasePipelineTest {
    def written = [:]

    @Override
    @BeforeEach
    void setUp() {
        scriptRoots = ['../vars']
        super.setUp()
        helper.registerAllowedMethod('stage', [String, Closure]) { String name, Closure body -> body.call() }
        helper.registerAllowedMethod('fileExists', [String]) { String name -> written.containsKey(name) }
        helper.registerAllowedMethod('readFile', [String]) { String name -> written[name] }
        helper.registerAllowedMethod('writeFile', [Map]) { Map args -> written[args.file] = args.text }
        helper.registerAllowedMethod('echo', [String]) { String message -> }
    }

    @Test
    void recordsSuccess() {
        def step = loadScript('timedStage.groovy')
        step.call('Build and test') { }
        assertTrue(written['stages.json'].contains('"name":"Build and test"'))
        assertTrue(written['stages.json'].contains('"result":"SUCCESS"'))
    }

    @Test
    void recordsFailureAndRethrows() {
        def step = loadScript('timedStage.groovy')
        assertThrows(RuntimeException) {
            step.call('Broken') { throw new RuntimeException('nope') }
        }
        assertTrue(written['stages.json'].contains('"result":"FAILURE"'))
    }
}
