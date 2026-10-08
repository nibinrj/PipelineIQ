// Job DSL. JCasC loads this at startup. The seed job re-applies it.
// Periodic scan only. GitHub cannot reach a localhost Jenkins, so there is no webhook.

def repos = [
  [name: 'pipelineiq-lab', owner: 'nibinrj', repository: 'pipelineiq-lab'],
  [name: 'PipelineIQ', owner: 'nibinrj', repository: 'PipelineIQ'],
]

repos.each { spec ->
  multibranchPipelineJob(spec.name) {
    displayName(spec.owner + '/' + spec.repository)
    branchSources {
      github {
        id(spec.name)
        repoOwner(spec.owner)
        repository(spec.repository)
        // This Job DSL context has scanCredentialsId, not credentialsId.
        scanCredentialsId('github-token')
        // Origin branches only. PR discovery stays off.
        buildOriginBranch(true)
        buildOriginBranchWithPR(true)
        buildOriginPRMerge(false)
        buildOriginPRHead(false)
        buildForkPRMerge(false)
        buildForkPRHead(false)
      }
    }
    orphanedItemStrategy {
      discardOldItems {
        numToKeep(10)
      }
    }
    triggers {
      periodicFolderTrigger {
        interval('5m')
      }
    }
  }
}

freeStyleJob('seed') {
  description('Re-applies jenkins/jobs/seed.groovy. tasks.ps1 seed triggers this job.')
  steps {
    dsl {
      external('/var/jenkins_jobs/seed.groovy')
      removeAction('IGNORE')
    }
  }
}
