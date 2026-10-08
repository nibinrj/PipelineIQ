import com.cloudbees.groovy.cps.NonCPS
import groovy.json.JsonOutput
import groovy.json.JsonSlurper

// timedStage wraps stage and appends one record to stages.json.
// A failure is recorded and rethrown, so the build still fails.
def call(String name, Closure body) {
    // The agent workspace is reused. A stages.json left by the previous build
    // makes this build upload two rows with the same stage name, and ingest
    // rolls the whole report back. Reset when the build number changes.
    resetStagesIfNewBuild()
    def startedMs = System.currentTimeMillis()
    def startedAt = java.time.Instant.ofEpochMilli(startedMs).toString()
    def result = 'SUCCESS'
    def thrown = null
    try {
        stage(name) {
            body()
        }
    } catch (err) {
        result = 'FAILURE'
        thrown = err
    } finally {
        def duration = System.currentTimeMillis() - startedMs
        try {
            // JsonSlurper is not serializable. Parsing has to stay out of the CPS
            // continuation, or a library step that calls timedStage cannot record stages.
            def text = fileExists('stages.json') ? readFile('stages.json') : ''
            writeFile file: 'stages.json', text: appendStage(text, name, startedAt, duration, result)
        } catch (writeErr) {
            echo "pipelineiq timedStage could not write stages.json: ${writeErr}"
        }
    }
    if (thrown != null) {
        throw thrown
    }
}

def resetStagesIfNewBuild() {
    def stamp = ''
    try {
        stamp = env?.BUILD_NUMBER?.toString() ?: ''
    } catch (ignored) {
        stamp = ''
    }
    if (!stamp) {
        return
    }
    def previous = fileExists('pipelineiq-stages.build') ? readFile('pipelineiq-stages.build').trim() : ''
    if (previous == stamp) {
        return
    }
    writeFile file: 'pipelineiq-stages.build', text: stamp + '\n'
    writeFile file: 'stages.json', text: ''
}

@NonCPS
def appendStage(String text, String name, String startedAt, long duration, String result) {
    def existing = []
    if (text) {
        def parsed = new JsonSlurper().parseText(text)
        if (parsed instanceof List) {
            existing = new ArrayList(parsed)
            existing.removeAll { it.name == name }
        }
    }
    existing.add([
        name       : name,
        started_at : startedAt,
        duration_ms: duration,
        result     : result,
    ])
    return JsonOutput.toJson(existing)
}
