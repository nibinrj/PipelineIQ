// reportBuild uploads the build. It never fails the build.
// PIPELINEIQ_URL and PIPELINEIQ_INGEST_KEY must already be in the environment.
// The Jenkinsfile binds the key with withCredentials so the shell text does not contain it.
def call(Map args = [:]) {
    try {
        def lines = []
        try {
            lines = currentBuild.rawBuild.getLog(500)
        } catch (logErr) {
            echo "pipelineiq reportBuild could not read the log: ${logErr}"
        }
        writeFile file: 'pipelineiq-log-tail.txt', text: (lines ?: []).join('\n')

        def job = args.job ?: env.JOB_NAME
        def buildNumber = args.buildNumber ?: env.BUILD_NUMBER
        def branch = args.branch ?: env.BRANCH_NAME
        def commit = args.commit ?: env.GIT_COMMIT
        def result = args.result ?: currentBuild.currentResult
        def agent = args.agent ?: env.NODE_NAME
        def repository = args.repository
        def pr = args.prNumber ?: env.CHANGE_ID
        def prArg = ''
        if (pr) {
            prArg = "--pr ${shQuote(pr.toString())}"
        }
        def script = """
            pipelineiq report \\
              --server "\$PIPELINEIQ_URL" \\
              --key "\$PIPELINEIQ_INGEST_KEY" \\
              --repository ${shQuote(repository ?: '')} \\
              --job ${shQuote(job ?: '')} \\
              --build ${shQuote(buildNumber?.toString() ?: '')} \\
              --branch ${shQuote(branch ?: '')} \\
              --commit ${shQuote(commit ?: '')} \\
              --result ${shQuote(result ?: '')} \\
              --agent ${shQuote(agent ?: '')} \\
              ${prArg}
        """
        withCredentials([string(credentialsId: 'pipelineiq-ingest-key', variable: 'PIPELINEIQ_INGEST_KEY')]) {
            def status = sh(script: script, returnStatus: true)
            if (status != 0) {
                echo "pipelineiq report exited ${status}; the build is not failed because of that"
            }
        }
    } catch (err) {
        echo "pipelineiq reportBuild swallowed an error: ${err}"
    }
}

def shQuote(String value) {
    return "'" + value.replace("'", "'\\''") + "'"
}
