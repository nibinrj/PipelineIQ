import groovy.json.JsonOutput
import groovy.json.JsonSlurper

// timedStage wraps stage and appends one record to stages.json.
// A failure is recorded and rethrown, so the build still fails.
def call(String name, Closure body) {
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
            def existing = []
            if (fileExists('stages.json')) {
                def parsed = new JsonSlurper().parseText(readFile('stages.json'))
                if (parsed instanceof List) {
                    existing = parsed
                }
            }
            existing << [
                name       : name,
                started_at : startedAt,
                duration_ms: duration,
                result     : result,
            ]
            writeFile file: 'stages.json', text: JsonOutput.toJson(existing)
        } catch (writeErr) {
            echo "pipelineiq timedStage could not write stages.json: ${writeErr}"
        }
    }
    if (thrown != null) {
        throw thrown
    }
}
