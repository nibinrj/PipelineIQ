package com.pipelineiq

import com.lesfurets.jenkins.unit.BasePipelineTest
import org.junit.jupiter.api.BeforeEach
import org.junit.jupiter.api.Test

import static org.junit.jupiter.api.Assertions.assertFalse
import static org.junit.jupiter.api.Assertions.assertTrue

class ReportBuildTest extends BasePipelineTest {
    String script = ''
    int status = 0

    @Override
    @BeforeEach
    void setUp() {
        scriptRoots = ['../vars']
        super.setUp()
        binding.setVariable('env', [
            JOB_NAME     : 'pipelineiq-lab/main',
            BUILD_NUMBER : '7',
            BRANCH_NAME  : 'main',
            GIT_COMMIT   : 'abc123',
            NODE_NAME    : 'agent',
            PIPELINEIQ_URL: 'http://pipelineiq-server:8080',
        ])
        binding.setVariable('currentBuild', [
            currentResult: 'SUCCESS',
            rawBuild     : [getLog: { int n -> ['line one', 'line two'] }],
        ])
        helper.registerAllowedMethod('echo', [String]) { String message -> }
        helper.registerAllowedMethod('writeFile', [Map]) { Map args -> }
        helper.registerAllowedMethod('withCredentials', [List, Closure]) { List bindings, Closure body -> body.call() }
        helper.registerAllowedMethod('sh', [Map]) { Map args ->
            script = args.script
            return status
        }
    }

    @Test
    void callsTheCliAndDoesNotInlineTheKey() {
        def step = loadScript('reportBuild.groovy')
        step.call([repository: 'nibinrj/pipelineiq-lab'])
        assertTrue(script.contains('pipelineiq report'))
        assertTrue(script.contains('nibinrj/pipelineiq-lab'))
        assertTrue(script.contains('$PIPELINEIQ_INGEST_KEY'))
        assertFalse(script.contains('super-secret'))
    }

    @Test
    void aNonZeroCliDoesNotFailTheBuild() {
        status = 1
        def step = loadScript('reportBuild.groovy')
        step.call([repository: 'nibinrj/pipelineiq-lab'])
        assertTrue(script.contains('--result'))
    }
}
