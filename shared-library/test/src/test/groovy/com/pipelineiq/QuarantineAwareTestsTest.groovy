package com.pipelineiq

import com.lesfurets.jenkins.unit.BasePipelineTest
import org.junit.jupiter.api.BeforeEach
import org.junit.jupiter.api.Test

import static org.junit.jupiter.api.Assertions.assertFalse
import static org.junit.jupiter.api.Assertions.assertTrue

class QuarantineAwareTestsTest extends BasePipelineTest {
    List<String> scripts = []
    Map files = [:]
    Map caught = [:]
    boolean serverDown = false
    boolean quarantineFails = false

    @Override
    @BeforeEach
    void setUp() {
        scriptRoots = ['../vars']
        super.setUp()
        binding.setVariable('env', [
            BUILD_NUMBER   : '12',
            PIPELINEIQ_URL : 'http://pipelineiq-server:8080',
        ])
        binding.setVariable('currentBuild', [currentResult: 'SUCCESS'])
        helper.registerAllowedMethod('echo', [String]) { String message -> }
        helper.registerAllowedMethod('echo', [GString]) { message -> }
        helper.registerAllowedMethod('pwd', []) { -> '/workspace' }
        helper.registerAllowedMethod('writeFile', [Map]) { Map args -> files[args.file] = args.text }
        helper.registerAllowedMethod('fileExists', [String]) { String name -> files.containsKey(name) }
        helper.registerAllowedMethod('readFile', [String]) { String name -> files[name] ?: '' }
        helper.registerAllowedMethod('withCredentials', [List, Closure]) { List bindings, Closure body -> body.call() }
        helper.registerAllowedMethod('timedStage', [String, Closure]) { String name, Closure body -> body.call() }
        helper.registerAllowedMethod('catchError', [Map, Closure]) { Map args, Closure body ->
            caught = args
            try {
                body.call()
            } catch (err) {
                // The real step records the stage and does not fail the build.
            }
        }
        def runSh = { script ->
            def text = script.toString()
            scripts << text
            if (serverDown && text.contains('pipelineiq quarantine')) {
                throw new RuntimeException('connection refused')
            }
            if (text.contains('--format surefire-only')) {
                files['pipelineiq-only.txt'] = files['only-body'] ?: ''
            }
            if (text.contains('--format surefire-excludes')) {
                files['pipelineiq-excludes.txt'] = files['excludes-body'] ?: "# pipelineiq quarantine — generated, do not edit\n"
            }
            if (quarantineFails && text.contains('-Dtest=')) {
                throw new RuntimeException('quarantine tests failed')
            }
        }
        helper.registerAllowedMethod('sh', [String], runSh)
        helper.registerAllowedMethod('sh', [GString], runSh)
    }

    @Test
    void emptyListSkipsTheQuarantineStage() {
        files['only-body'] = ''
        def step = loadScript('quarantineAwareTests.groovy')
        step.call([repository: 'nibinrj/pipelineiq-lab'])
        assertTrue(scripts.any { it.contains('surefire.excludesFile=/workspace/pipelineiq-excludes.txt') })
        assertFalse(scripts.any { it.contains('-Dtest=') })
        assertTrue(scripts.any { it.contains('blocking-reports') })
    }

    @Test
    void nonEmptyListRunsOnlyThoseTestsAndKeepsTheBuildResult() {
        files['only-body'] = 'com.example.lab.FlakyTest#testFlips\n'
        quarantineFails = true
        def step = loadScript('quarantineAwareTests.groovy')
        step.call([repository: 'nibinrj/pipelineiq-lab'])
        assertTrue(scripts.any { it.contains('-Dtest=com.example.lab.FlakyTest#testFlips') })
        assertTrue(scripts.any { it.contains('quarantine-reports') })
        assertTrue(caught.stageResult == 'UNSTABLE')
        assertTrue(caught.buildResult == 'SUCCESS')
        assertTrue(scripts.any { it.contains('$PIPELINEIQ_INGEST_KEY') })
        assertFalse(scripts.join('\n').contains('super-secret'))
    }

    @Test
    void serverDownStillRunsTheBlockingStage() {
        serverDown = true
        def step = loadScript('quarantineAwareTests.groovy')
        step.call([repository: 'nibinrj/pipelineiq-lab'])
        assertTrue(files['pipelineiq-excludes.txt'] == '')
        assertTrue(scripts.any { it.contains('surefire.excludesFile=/workspace/pipelineiq-excludes.txt') })
        assertFalse(scripts.any { it.contains('-Dtest=') })
    }
}
