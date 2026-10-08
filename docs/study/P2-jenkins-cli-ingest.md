# P2 — Jenkins, CLI, ingest

Written before the code, then checked against it. Owner is a Java developer new to Go.
Comparisons are for reading the diff, not for copying Java style into Go.

Checked on 2026-10-07 unless a line says otherwise.

## What I am certain of, and what I did not run

Certain, because I read the page, the tag, or the source:

- Jenkins LTS is 2.580.1, published 2026-09-30. Source: https://www.jenkins.io/changelog-stable/#v2.580.1
  and the GitHub release `jenkins-2.580.1` on `jenkinsci/jenkins`. Weekly 2.585 (2026-10-06)
  is newer and was not used. The controller image tag `jenkins/jenkins:2.580.1-jdk21` exists
  on Docker Hub. The `jenkinsci/docker` release `2.580.1` says that image line uses JDK 21.0.12.1+1.
- That Jenkins war bundles remoting `3386.v353e57a_1b_ea_0` (`<remoting.version>` in
  `jenkinsci/jenkins` `pom.xml` at tag `jenkins-2.580.1`). The agent image is
  `jenkins/inbound-agent:3386.v353e57a_1b_ea_0-3-jdk21`, the newest image revision of that
  same remoting. The tag exists on Docker Hub. A newer remoting (`3391`) was not used,
  because the agent should match the controller.
- Maven 3.9.16, released 2026-05-17 (Apache release catalog). Image tag
  `maven:3.9.16-eclipse-temurin-21` exists on Docker Hub. It is a copy source for the agent
  image, not the agent's base image.
- Go on the agent is 1.27.1, the same pin as `go.mod`, copied from `golang:1.27.1-bookworm`.
- Plugin versions below are GitHub release tags dated on or before 2026-09-02, the day the
  2.580 weekly shipped. I did not load them in Jenkins, so a dependency conflict is possible.
- `libraryPath` is still a relative path inside the SCM checkout, not a host path. Confirmed
  again on `pipeline-groovy-lib` tag `805.va_fc79344957d` (`help-libraryPath.html` is present).
  The P1 note read a later tag's Java. The field is the same idea.
- Git plugin 5.10.1 README: a `file:` remote is refused unless
  `-Dhudson.plugins.git.GitSCM.ALLOW_LOCAL_CHECKOUT=true`. That property is set. It is a
  local-only risk, documented by SECURITY-2478.
- JCasC shapes for a local user, an inbound WebSocket node, a global library, string
  credentials, and a Job DSL `jobs:` block come from the configuration-as-code demos
  (`demos/embedded-userdatabase`, `demos/build_agents`, `demos/pipeline-groovy-lib`,
  `demos/credentials`, `demos/jobs`). The inbound and library samples were read from the
  plugin's default branch, not from tag `2121.v86fe99d4b_b_a_b_`. The inbound `webSocket`
  key is also in that tag's `demos/build_agents/README.md`. The file was not loaded by Jenkins.
- Surefire element names are the P1 rules. The XML fixtures in `testdata/surefire/` are
  hand-written to that shape. Surefire 3.6.0 was not executed, so these are not captured reports.
- JenkinsPipelineUnit 1.31 (GitHub release 2026-07-01) tests shared libraries with
  `ProjectSource.projectSource(path)`. Its own build uses Groovy 2.4.21 and JUnit 6.1.1.
  The README says it is not compatible with Groovy 4.

Not verified here:

- No Docker daemon. `docker compose up`, `docker ps`, and `docker stats` were not run.
  Jenkins did not start. JCasC was not loaded. No lab build was ingested through Jenkins.
- `golangci-lint run` was not executed. Same blocker as P1: the release asset host and
  several module hosts are unreachable from this environment.
- The JenkinsPipelineUnit tests were not executed. `java` and `mvn` are not installed here.
- Plugin compatibility with Jenkins 2.580.1 was not proven by starting the controller.

## Memory before the new containers

P1 compose limits: server 64 + Postgres 384 + Grafana 256 = 704 MB. The plan's Phase 1
budget for the two new containers is Jenkins controller 1 GB and agent 1.5 GB. Compose sets
those: controller `1024m`, agent `1536m`. Expected limit sum: 1024 + 1536 + 64 + 384 + 256
= 3264 MB. That is a sum of limits, not a measurement. `docker stats` was not run. Both new
services set `mem_limit` and `memswap_limit` to the same value, so Docker does not add swap
on top of the limit. The Docker socket is not mounted on the agent. The plan mentions it for
Testcontainers, and the P2 lab does not start sibling containers. A missing socket path would
stop `compose up`.

## Versions pinned in P2

| Thing | Pin | Source |
| --- | --- | --- |
| Jenkins controller | `jenkins/jenkins:2.580.1-jdk21` | LTS changelog 2026-09-30; Docker Hub tag |
| Inbound agent | `jenkins/inbound-agent:3386.v353e57a_1b_ea_0-3-jdk21` | remoting version in Jenkins 2.580.1 pom; Docker Hub tag |
| Maven on the agent | 3.9.16, copied from `maven:3.9.16-eclipse-temurin-21` | Apache release catalog; Docker Hub tag |
| Go on the agent | 1.27.1, copied from `golang:1.27.1-bookworm` | same pin as `go.mod` |
| configuration-as-code | 2121.v86fe99d4b_b_a_b_ | GitHub release 2026-08-30 |
| job-dsl | 3732.v9a_c49a_61a_313 | GitHub release 2026-08-12 |
| workflow-aggregator | 608.v67378e9d3db_1 | GitHub release 2025-03-19, latest tag |
| pipeline-graph-view | 1013.v9f83fd83c063 | GitHub release 2026-08-31 |
| git | 5.10.1 | GitHub release 2026-03-25 |
| github-branch-source | 1983.vfa_27ed961853 | GitHub release 2026-08-05 |
| credentials-binding | 728.v902a_273b_8947 | GitHub release 2026-07-09 |
| junit | 1425.v9c7318dca_96d | GitHub release 2026-09-02 |
| timestamper | 1.30 | GitHub release 2025-07-02 |
| pipeline-groovy-lib | 805.va_fc79344957d | GitHub release 2026-09-02 |
| JenkinsPipelineUnit | 1.31 | GitHub release 2026-07-01 |
| JUnit Jupiter (library tests) | 6.1.1 | matches the JenkinsPipelineUnit 1.31 build file |
| Groovy (library tests) | 2.4.21 | JenkinsPipelineUnit 1.31 `build.gradle` |
| Surefire in the lab | 3.6.0 | the page cited in the P1 note |

`pipeline-groovy-lib` is not in the prompt's minimum plugin list. The prompt requires a
global library named `pipelineiq`. That feature is this plugin. `workflow-aggregator` does
not include it. `credentials-binding` already depends on `plain-credentials`, so string
credentials do not need another direct line. No other plugin was added.

## JCasC, the library, and the agent

`libraryPath` is still not a directory on the Jenkins machine. P2 mounts this repo at
`/pipelineiq` on the controller and the agent, and points Modern SCM Git at
`file:///pipelineiq` with `libraryPath: shared-library`. The agent has the mount because
library checkout can run there. Both images mark `/pipelineiq` as a Git safe directory and
allow local checkout. `defaultVersion` is `${PIPELINEIQ_LIBRARY_REF}`, which compose
defaults to `main`. This branch is not `main`. `.env.example` sets the ref to this branch
so the controller can load `shared-library/` before merge. The verification job points that
ref at a local branch named `ci-library`, because a slash in the branch name is a bad
library version. Git checkout sees that commit, not later uncommitted edits.

The controller image runs `git config --system --add safe.directory /pipelineiq` and
`safe.directory '*'`. `--global` would write `~/.gitconfig` under `/var/jenkins_home`, and
the named volume hides that file at runtime. `/etc/gitconfig` survives the volume. The bind
mount is owned by the host user, not the `jenkins` user. That is a local-lab setting.

The agent is inbound over WebSocket. JCasC creates the node. It does not let us pin the
agent secret. The agent entrypoint waits for the controller, authenticates as the local
admin, and reads the secret from `computer/<name>/jenkins-agent.jnlp`. That is a process,
not a UI click. The admin password is in the agent environment for that fetch. It comes
from `.env`, not from git. The image's `jenkins-agent` already turns `JENKINS_URL` and
`JENKINS_AGENT_NAME` into flags. The entrypoint only exports the secret and
`JENKINS_WEB_SOCKET=true`. Passing `-url` again makes remoting reject the launch. The TCP
agent port is disabled (`JENKINS_SLAVE_AGENT_PORT=-1`); WebSocket uses the HTTP port.

Jenkins is published on host port 8081. Port 8080 stays the PipelineIQ service.
`PIPELINEIQ_URL` inside the agent network is `http://pipelineiq-server:8080`.

The seed file is Job DSL. JCasC loads it at startup (`jobs: - file:`), and a `seed` job
re-applies it. `tasks.ps1 seed` triggers that job. The GitHub branch source method in this
Job DSL version is `scanCredentialsId`, not `credentialsId`. Origin branches are indexed;
pull requests are not. Branch indexing is a periodic scan (`5m`). There is no webhook:
GitHub cannot reach localhost. The GitHub credential is a placeholder until a fine-grained
token is set. Scan of a missing or private repo will fail until that token is real and the
lab repo is pushed. That is expected.

The controller has zero executors. Builds run on the agent label `linux`.

## CLI

No extra Go module. `flag` is the standard library, the same idea as parsing `args` in a
Java `main` without bringing in a CLI framework. Subcommands are the first argument:
`report` and `quarantine`. `quarantine` prints that P3 implements it and exits 0, so a
pipeline that calls it early does not go red.

`report` flags are the ones the prompt lists, plus `--repository`. The data model requires
a repository row, and a Jenkins job name is not a repository name
(`pipelineiq-lab/main` versus `nibinrj/pipelineiq-lab`). `--strict` is also required by the
prompt: if the server is down, the default is a warning and exit 0.

Retries: connection errors and HTTP 5xx, three attempts. The waits between them are 200ms
then 400ms.
HTTP 401 is not retried. A wrong key will not become right. 401 exits 1. "Server down" means
the TCP connection failed after the retries.

The upload is `multipart/form-data`:

| Part | Meaning |
| --- | --- |
| `metadata` | JSON: repository, job, build number, branch, optional PR, commit, result, agent |
| `stages` | JSON array from `stages.json` |
| `log_tail` | text file, last 500 lines |
| `report` | one part per XML file; the filename parameter is the relative path |

Default report search, if `--report` is omitted: `target/surefire-reports` and
`target/failsafe-reports` at the workspace root and one module directory down. A passed
`--report` replaces that list. A missing directory is not an error. A build can upload
stages and no tests.

The key is `Authorization: Bearer`. The library does not put the key in the shell script
text. `reportBuild` expands `$PIPELINEIQ_INGEST_KEY`, which `withCredentials` injects.
Jenkins then echoes the variable name, not the secret.

## Ingest

`POST /api/v1/builds` requires the bearer key. Comparison uses `crypto/subtle` so the check
does not stop at the first different byte. A missing key and a wrong key both return 401
with the same body. The key is required at process start (`PIPELINEIQ_INGEST_KEY`). An empty
key would make the check pass for an empty header, which is an open endpoint. That is the
same reason a Java filter rejects a blank configured secret instead of treating it as
"auth disabled".

The body is limited to 8 MB. The log tail stored in Postgres is capped at 256 KB. The plan's
table list has no log column. P2 uploads the tail, and P3's infra rules need it, so
`migrations/00002_build_log_tail.sql` adds `build.log_tail`. That is a new migration, not an
edit of `00001`.

Idempotency is the unique key `(job_name, build_number)`. The second POST updates that row,
deletes its `stage_run` and `test_run` children, and inserts the new ones. `test_case` rows
are identities and are not deleted. A repeated upload does not add a second build.

`GET /api/v1/builds/{id}` returns the build, its stages, and its test runs. It uses the same
key. It is for debugging, not a public API.

Every test run in P2 is `stage = BLOCKING`. The quarantine stage does not exist until P3.
`agent_lifecycle` is `LOCAL`. `infra_failure` stays false. Infra rules are P3.

## Parser

The parser walks `testcase` elements and ignores suite attributes. A file whose `tests`
attribute is 6 and whose body has one `testcase` produces one result. That is the
SUREFIRE-1627 fixture.

| Children of `testcase` | Outcome | `rerun_failures` |
| --- | --- | --- |
| none | PASSED | 0 |
| `skipped`, and no failure/error | SKIPPED | 0 |
| `failure`, no `error` | FAILED | count of `rerunFailure` + `rerunError` |
| `error` | ERROR | count of `rerunFailure` + `rerunError` |
| `flakyFailure` or `flakyError`, and no `failure`/`error` | FLAKY | count of those elements |

`failure` wins over `skipped`. `error` wins over `failure` if both appear, which Surefire
does not normally do. `testcase time` is seconds; the stored duration is that number times
1000, rounded to milliseconds. The parser does not add rerun times. The P1 note says the
docs already put the right time on `testcase`.

`failure_hash` is SHA-256 of the exception type plus the first stack frame whose class is
not a framework class (`java.`, `javax.`, `jdk.`, `sun.`, `com.sun.`, `org.junit.`, `junit.`,
`org.hamcrest.`, `org.opentest4j.`, `org.testng.`, `org.apache.maven.`, `org.gradle.`). If every frame is
a framework frame, the hash is the type plus the failure message. The subject's package is
not configured in P2. A denylist is the honest stand-in until a repo declares its packages.
Two failures with the same type and the same first application frame hash the same.

The Maven module is the directory before `/target/` in the uploaded filename
(`core/target/surefire-reports/TEST-Foo.xml` → `core`). It is stored and is not part of the
test identity. That matches the P1 schema note. Go's `ParseMultipartForm` keeps only the
base name, so the handler reads the original `filename` parameter from
`Content-Disposition` instead of `FileHeader.Filename`.

## Shared library

`timedStage(name) { body }` opens `stage(name)`, times the body, and appends one object to
`stages.json`. A thrown error is recorded as `FAILURE` and rethrown, so the build still
fails. A failure to write the file is printed and does not hide the original error.
`started_at` is UTC, RFC3339, from `java.time.Instant`. Duration is milliseconds.
`currentBuild.currentResult` is not used here: that is the build, not the stage.

`reportBuild` runs in `post { always }` via a `finally` in the lab Jenkinsfile. It writes
the last 500 log lines and runs the CLI. `sh` uses `returnStatus: true`. Any exception is
caught and printed. The step does not fail the build. `currentBuild.rawBuild.getLog` is not
a sandboxed call. Global libraries from `globalLibraries` are trusted (`isTrusted` returns
true in `GlobalLibraries.ForJob` on tag `805.va_fc79344957d`), so that call does not need a
script-approval click. The Jenkinsfile itself only calls `timedStage`, `sh`, and `reportBuild`.

The lab Jenkinsfile is scripted, not declarative. `timedStage` already opens a stage. A
declarative `stage` around it would record two stages for one piece of work.

JenkinsPipelineUnit tests live in `shared-library/test` and are Maven, not Gradle. The
prompt said to ask which. Maven is the lab's build, and the only JVM tool this repo needs
for tests. Gradle would be a second build tool for one test module. The tests set
`scriptRoots` to `../vars` and `loadScript` the step. That is the direct JenkinsPipelineUnit
path. `projectSource('../')` is the production-like retriever and is not what these two
tests call.

## Go, next to Java

- The CLI's `flag.FlagSet` is a local parser. It does not touch `flag.CommandLine`, so a
  test can parse args without affecting another test. A Java program would new up its own
  parser for the same reason.
- `context.Context` on the upload is the timeout. The HTTP client does not use the default
  client with no timeout. A Java call would set connect and read timeouts on the request.
- The handler depends on an `Ingester` interface so the HTTP tests do not open Postgres.
  That is a fake, not a layer of repositories. The pool is still concrete in `internal/ingest`.
- Errors from the database are wrapped and logged. The HTTP body says `internal error` or
  `bad request`. The driver text can contain a statement. It does not go to the client.
- The ingest write is one transaction. If a test insert fails, the build update rolls back.
  That is the same boundary as one Spring `@Transactional` method, without the proxy.

## Lab repo

`../pipelineiq-lab` is a three-module Maven build: `core` (stable), `api` (depends on core,
one labelled slow test that sleeps one second), `flaky-lab` (a stable placeholder). P2 said
no flaky tests yet. Injected flakiness is not in this tree. The README says so.

The repo is created locally. It is not pushed. Commands to push are in that README and in
the prompt close-out.

## Checked after the code

`gofmt -l` was empty. `go test -race -count=1 -timeout 240s ./...` passed, including the
Surefire fixtures, the CLI httptest cases, the handler tests, and the idempotent ingest
test. That ingest test used the already-running embedded Postgres 18.6 at
`/tmp/pgdata-pipelineiq`, not the `postgres:18.6` image.

Still not run: `golangci-lint run`, `docker compose up`, `docker stats`, a Jenkins start,
JCasC load, JenkinsPipelineUnit (`java` and `mvn` are not installed here), and a lab build
appearing in the API.
