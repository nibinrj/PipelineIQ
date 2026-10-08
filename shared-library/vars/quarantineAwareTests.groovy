// quarantineAwareTests fetches the active list, excludes those methods from the
// blocking stage, then runs only those methods in a non-blocking stage.
// PIPELINEIQ_URL must already be in the environment. The key comes from the
// pipelineiq-ingest-key credential, so the shell text does not contain it.
def call(Map args = [:]) {
    def repository = args.repository
    if (!repository) {
        echo 'pipelineiq quarantineAwareTests: repository is required'
        return
    }
    def excludes = 'pipelineiq-excludes.txt'
    def only = 'pipelineiq-only.txt'
    // Surefire resolves excludesFile from each module directory, not the reactor root.
    def excludesPath = "${pwd()}/${excludes}"
    def seed = env.BUILD_NUMBER ?: '1'
    def extra = args.mavenArgs ?: ''
    try {
        withCredentials([string(credentialsId: 'pipelineiq-ingest-key', variable: 'PIPELINEIQ_INGEST_KEY')]) {
            sh """
                pipelineiq quarantine --server "\$PIPELINEIQ_URL" --key "\$PIPELINEIQ_INGEST_KEY" --repo ${shQuote(repository)} --format surefire-excludes --out ${excludes}
                pipelineiq quarantine --server "\$PIPELINEIQ_URL" --key "\$PIPELINEIQ_INGEST_KEY" --repo ${shQuote(repository)} --format surefire-only --out ${only}
            """
        }
    } catch (err) {
        echo "pipelineiq quarantine list failed: ${err}"
        writeFile file: excludes, text: ''
        writeFile file: only, text: ''
    }
    def excludeText = ''
    if (fileExists(excludes)) {
        excludeText = readFile(excludes)
    }
    echo "pipelineiq excludes:\n${excludeText}"
    timedStage('Blocking tests') {
        try {
            sh "mvn -B -Dpipelineiq.flaky.seed=${seed} ${extra} -Dsurefire.excludesFile=${excludesPath} -Dfailsafe.excludesFile=${excludesPath} test"
        } finally {
            moveReports('blocking-reports')
        }
    }
    def patterns = quarantinePatterns(fileExists(only) ? readFile(only) : '')
    if (patterns) {
        echo "pipelineiq quarantine stage: ${patterns}"
        def keep = currentBuild.currentResult ?: 'SUCCESS'
        // -Dtest= is applied to every module. Modules that do not contain the
        // method must not fail the reactor before flaky-lab runs.
        catchError(buildResult: keep, stageResult: 'UNSTABLE') {
            timedStage('Quarantine') {
                try {
                    sh "mvn -B -Dpipelineiq.flaky.seed=${seed} ${extra} -Dsurefire.failIfNoSpecifiedTests=false -Dtest=${patterns} test"
                } finally {
                    moveReports('quarantine-reports')
                }
            }
        }
    } else {
        echo 'pipelineiq quarantine stage skipped: no quarantined tests'
    }
}

// Surefire 3.6.0 reportsDirectory has no command-line property. Move the
// directory so the quarantine run cannot overwrite the blocking reports.
def moveReports(String destName) {
    sh """
        find . -type d -name surefire-reports -path '*/target/surefire-reports' | while read -r dir; do
            parent=\$(dirname "\$dir")
            rm -rf "\$parent/${destName}"
            mv "\$dir" "\$parent/${destName}"
        done
    """
}

def quarantinePatterns(String text) {
    if (!text) {
        return ''
    }
    def lines = []
    for (line in text.split('\n')) {
        def trimmed = line.trim()
        if (trimmed && !trimmed.startsWith('#')) {
            lines.add(trimmed)
        }
    }
    return lines.join(',')
}

def shQuote(String value) {
    return "'" + value.replace("'", "'\\''") + "'"
}
