package com.pipelineiq

import com.lesfurets.jenkins.unit.BasePipelineTest
import org.junit.jupiter.api.BeforeEach
import org.junit.jupiter.api.Test

import static org.junit.jupiter.api.Assertions.assertFalse
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
        binding.setVariable('env', [BUILD_NUMBER: '7'])
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

    @Test
    void aNewBuildNumberReplacesThePreviousFile() {
        written['stages.json'] = '[{"name":"Blocking tests","result":"OLD"}]'
        written['pipelineiq-stages.build'] = '6\n'
        def step = loadScript('timedStage.groovy')
        step.call('Blocking tests') { }
        assertTrue(written['pipelineiq-stages.build'].contains('7'))
        assertTrue(written['stages.json'].count('Blocking tests') == 1)
        assertFalse(written['stages.json'].contains('OLD'))
    }
}
