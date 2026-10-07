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
        credentialsId('github-token')
        traits {
          gitHubBranchDiscovery {
            // 3 = all branches. PRs are not indexed in P2.
            strategyId(3)
          }
        }
      }
    }
    orphanedItemStrategy {
      discardOldItems {
        numToKeep(10)
      }
    }
    triggers {
      periodicFolderTrigger {
        interval('5')
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
