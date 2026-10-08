# P1 — design notes

Written before the skeleton, then checked against the code that followed. Owner is a Java
developer new to Go. Comparisons below are for reading the diff, not for copying Java style
into Go.

Checked on 2026-10-07 unless a line says otherwise.

## What I am certain of, and what I did not run

Certain, because I read the page or the source:

- Surefire/Failsafe rerun XML element names, from the official plugin pages cited below.
  The page I opened is for plugin version 3.6.0.
- The totals bug, from ASF Jira SUREFIRE-1627, status still Open, no fix version.
- Method filters in `excludesFile` and `-Dtest`, from the Surefire 3.6.0 mojo page and the
  inclusion/exclusion example page.
- `libraryPath` is a relative path inside an SCM checkout, from
  `SCMBasedRetriever.java` on the default branch of
  `jenkinsci/pipeline-groovy-lib-plugin` (cloned 2026-10-07). Latest GitHub release of that
  plugin is `806.v408277b_33d1d` (2026-09-16). I did not check out that tag, so a line of
  the source could differ. The field and the javadoc are what I read.
- Go 1.27.1 exists: tag `go1.27.1` on `github.com/golang/go`, commit message
  `[release-branch.go1.27] go1.27.1`, committer date 2026-09-01.
- Module versions below, from GitHub releases.
- Image tags below, from `docker-library/official-images` or the Grafana download page.

Commands run in this environment on 2026-10-07, after the skeleton was written:

- `go version` printed `go version go1.27.1 linux/amd64`. That binary was not the official
  tarball from go.dev (that host was unreachable) and its checksum was not checked.
- `gofmt -l .` printed nothing and exited 0.
- `go vet ./...` exited 0.
- `go test -race -count=1 -timeout 180s ./...` exited 0. Packages with no tests were
  reported as `[no test files]`. `internal/config`, `internal/db`, and `internal/httpserver`
  passed.
- `TestMigrationsApplyAndTablesExist` logged
  `PostgreSQL 18.6 on x86_64-pc-linux-gnu, compiled by gcc (GCC) 14.2.1 20250110 (Red Hat 14.2.1-11), 64-bit`.
  That server was embedded-postgres 18.6.3, reached through `PIPELINEIQ_TEST_DATABASE_URL`.
  It is not the `postgres:18.6` image, and the testcontainers path in the same test was not
  executed. A second run of the same test also passed.
- `pipelineiq-server` was started on the host against that same database, not in a container.
  `GET /healthz` and `GET /readyz` returned `{"status":"ok"}`. `POST /healthz` returned 405.
  SIGTERM logged `stopped` and the process exited. The log line named the database host and
  did not include the password.

Not verified in this environment:

- I did not `docker pull` any image and did not run `docker compose up`. No Docker daemon
  was available. `docker ps` and `docker stats` were not run. The 704 MB figure below is the
  sum of configured limits, not a measurement.
- I did not run Surefire 3.6.0, so the XML and `-Dtest` notes are from the docs, not from a
  fixture I generated. Fixtures are P2.
- I did not start Jenkins, so the JCasC snippet is a proposal for P2, not a file that loaded.
- golangci-lint v2.14.0 was not executed. `gh release download` of
  `golangci-lint-2.14.0-linux-amd64.tar.gz` failed because
  `release-assets.githubusercontent.com` returned EOF. Building tag `v2.14.0` from source
  also failed: `charm.land`, `mvdan.cc`, `codeberg.org`, `gitlab.com`, `4d63.com`,
  `go-simpler.org`, `go.augendre.info`, and `dev.gaijin.team` were unreachable. The config
  file matches the v2 shape in that tag's `.golangci.reference.yml`. That is a schema check,
  not a lint run.
- Distroless digest was not pinned. The registry was unreachable.

## Surefire and Failsafe XML

Pages checked:

- https://maven.apache.org/surefire/maven-surefire-plugin/examples/rerun-failing-tests.html
- https://maven.apache.org/surefire/maven-failsafe-plugin/examples/rerun-failing-tests.html
- https://maven.apache.org/surefire/maven-surefire-plugin/test-mojo.html
  (full name `org.apache.maven.plugins:maven-surefire-plugin:3.6.0:test`)
- https://issues.apache.org/jira/browse/SUREFIRE-1627

`rerunFailingTestsCount` reruns a failing test until it passes or the count is used up.
Supported for JUnit 4.x (4.12+), JUnit 5, and TestNG. Failsafe uses the same report shape;
the user property is `failsafe.rerunFailingTestsCount` instead of
`surefire.rerunFailingTestsCount`.

A `testcase` is one test method. Child elements say what happened:

| Element | Meaning for PipelineIQ |
| --- | --- |
| none of the failure elements | PASSED |
| `failure` | FAILED. Assertion failure. First failing run, if every rerun also failed. |
| `error` | ERROR. Unexpected exception. Same "first run" rule as `failure`. |
| `skipped` | SKIPPED |
| `flakyFailure` or `flakyError`, and no `failure`/`error` | FLAKY. A rerun passed. `rerun_failures` is the count of those elements. |
| `rerunFailure` or `rerunError` | Extra failing reruns after the first `failure`/`error`. The outcome stays FAILED or ERROR. Do not count these as separate tests. |

The docs are explicit about times, and the parser must not "fix" them:

- If a rerun passes, `testcase time` is the last successful run, not the sum.
- If every run fails, `testcase time` is the first failing run, not the sum.

`system-out` and `system-err` may appear both at the top of `testcase` and inside each
flaky or rerun element. The top-level ones are the last successful run when the test is
flaky, and the first failing run when it never passes.

### The totals bug

SUREFIRE-1627 (Open, affects 2.19.1, also reported on 2.22.1 and 3.0.0-M3, no fix version
on the page I read): the `testsuite` attributes `tests` and `failures` count each rerun as
another test. The console summary does not. A suite with one failing method and five reruns
can say `tests="6"` while the body has one `testcase`.

So the parser reads `testcase` elements and ignores suite totals. A fixture with wrong
totals and one `testcase` must produce one `test_run`. That fixture is P2; the rule is
fixed now so P2 does not get to rediscover it.

## How the CLI will select tests

Certain, from the Surefire 3.6.0 mojo page and the inclusion/exclusion page.

### Exclude specific methods — this is the quarantine format

`excludesFile` (user property `surefire.excludesFile`, since 2.13) is a file of patterns.
Blank lines and lines starting with `#` are ignored. If `<excludes>` is also set, the file
is appended.

Since 3.0.0-M6 the file also filters methods:

```text
# pipelineiq quarantine — generated, do not edit
com.example.lab.FlakyTest#testFlips
com.example.lab.FlakyTest#testTimeBound
```

Failsafe has the same feature on `failsafe.excludesFile`. The CLI will write one file and
the pipeline will pass it to both plugins when both run. P3 generates this format. I am
certain of the syntax. I have not executed Surefire against a generated file.

### Run only specific methods — the quarantine stage

Certain: `-Dtest=com.example.lab.FlakyTest#testFlips+testTimeBound` runs those two methods.
The mojo page shows `#method` and `+` between methods. Several classes are comma-separated.

The same page says `-Dtest` **overrides** `includes` and `excludes`. A leading `!` excludes
a pattern (`-Dtest=!Unstable*`). That is documented. It is still the wrong tool for
quarantine:

- `-Dtest=!com.example.lab.FlakyTest#testFlips` replaces the include set. It does not mean
  "run the default suite except this method". Depending on the provider, the `!` form can
  select nothing useful or select too much. I am certain it overrides includes/excludes. I
  am not certain of every JUnit 5 provider's handling of method-level `!`, because I did
  not run it. That is why the plan says prefer the excludes file.
- The blocking stage uses `surefire.excludesFile` so the default includes stay in place.
- The quarantine stage uses `-Dtest=Class#m1+m2` because that stage wants only those
  methods. Overriding includes is what we want there.

Class-level `<exclude>**/FlakyTest.java</exclude>` is the wrong grain. Quarantine is a
method. Excluding the class would also skip the stable methods in it.

## Stage timings, recorded by the library

Do not call the Stage View REST API (`/job/.../wfapi/describe`, or whatever the current
path is). Two reasons, one of them checked:

1. The plan said Pipeline Stage View was up for adoption. I opened
   https://plugins.jenkins.io/pipeline-stage-view/ on 2026-10-07. The page documents the
   view and its REST limits (`com.cloudbees.workflow.rest.external.*`). It does not say the
   plugin is up for adoption. A search snippet for the health page said it is not marked up
   for adoption and that version 2.41 was released about three months before that index.
   I am not treating "up for adoption" as a current fact. The API still has hardcoded
   limits (10 runs per job unless a system property overrides it, and that override needs
   a Jenkins restart). That is enough to refuse it as the source of timings.
2. Timings have to exist even if the plugin is not installed. The architecture uploads
   from the agent. Scraping the controller inverts that.

P2's `timedStage` will wrap `stage` and append a JSON object itself:

```text
{ "name": "Test", "started_at": "<RFC3339 UTC>", "duration_ms": 1200, "result": "SUCCESS" }
```

Sketch, not shipped in P1 (the prompt does not ask for the shared library yet):

```groovy
def call(String name, Closure body) {
    def started = System.currentTimeMillis()
    def result = 'SUCCESS'
    try {
        stage(name) { body() }
    } catch (err) {
        result = 'FAILURE'
        throw err
    } finally {
        def duration = System.currentTimeMillis() - started
        // append name, started, duration, result to stages.json in the workspace
    }
}
```

`currentBuild.currentResult` is the build, not the stage. A stage can fail and be caught
by `catchError` later; the wrapper must record the stage's own result. Time is milliseconds,
UTC when it is turned into a timestamp. The CLI uploads the file. PipelineIQ does not ask
Jenkins what the stage took.

## JCasC and a library that lives in this repo

Pages and source checked:

- https://www.jenkins.io/doc/book/pipeline/shared-libraries/ (Retrieval method: Modern SCM
  or Legacy SCM only; no "folder on disk" option on that page)
- `jenkinsci/pipeline-groovy-lib-plugin` `SCMBasedRetriever.java` and
  `SCMSourceRetriever.java` (`@Symbol("modernSCM")`)

`libraryPath` is **not** a host path. The javadoc says: null means the library is at the
repository root; otherwise it is a relative path inside the checkout, and the setter adds a
trailing slash. `..` is rejected. An absolute path is rejected (`isRelativePath`).

So this does not work, and I will not propose it:

```yaml
# wrong: libraryPath is not a directory on the Jenkins machine
libraryPath: /var/jenkins_home/pipelineiq/shared-library
```

What does work, and what P2 should use:

1. Mount this repo into the controller, for example at `/var/jenkins_home/pipelineiq`.
2. Point Modern SCM Git at that checkout with a `file://` remote (Jenkins on localhost
   cannot be reached by GitHub webhooks; a file remote does not need GitHub either).
3. Set `libraryPath: shared-library` so the library root is that subdirectory, not the
   Go module root. `vars/` and `src/` must be directly under `shared-library/`.

Proposed shape for P2, not applied now:

```yaml
unclassified:
  globalLibraries:
    libraries:
      - name: pipelineiq
        defaultVersion: main
        implicit: true
        retriever:
          modernSCM:
            libraryPath: shared-library
            scm:
              git:
                remote: file:///var/jenkins_home/pipelineiq
```

`implicit: true` means Jenkinsfiles do not need `@Library('pipelineiq')`. The lab Jenkinsfile
in the plan still uses the annotation. Either is fine; implicit plus the annotation is
harmless if the version matches. I have not loaded this YAML in Jenkins. If `file://` Git
refuses a non-bare repo or a detached worktree, the fallback is a tiny bare clone of
`shared-library/` inside the controller image, still JCasC, still no UI click. I would
rather find that out in P2 than add a filesystem-retriever plugin now. The plan says ask
before adding a plugin. Stock retrievers are Modern SCM and Legacy SCM only.

## Go choices

### sqlc + pgx, not plain pgx

pgx v5.11.0 is the driver (GitHub release 2026-09-07). sqlc v1.31.1 (GitHub release
2026-04-22) reads `queries/` and `migrations/` and writes `internal/store`. A renamed
column fails `sqlc generate` or the Go build, instead of failing the first request that
hits that query.

Plain pgx would be fewer tools. Every query would be a string in a Go file, checked only
when a test runs it. That is worse once ingest has a dozen statements. sqlc is not an ORM.
The SQL is still the source.

P1 queries: `Ping`, `UpsertRepository`, `GetRepositoryByName`, `GetBuildByJobNumber`.
The server calls `Ping` for `/readyz`. The others are the first repository and build
lookups P2 will need, generated now so the store package is not empty.

### goose, not golang-migrate

See `docs/adr/001-scope-and-stack.md`. goose v3.28.0. The server calls `goose.UpContext`
on startup with the SQL embedded via `//go:embed`. The distroless image has no shell, so
a separate migrate CLI would be a second thing to copy in. Startup migration is not in the
prompt's file list as its own bullet; it is how `/readyz` can mean "schema is there" after
`docker compose up` on an empty volume. A probe that applies schema as a side effect would
be worse, so migration is once in `main`, not inside the handler.

`goose.SetDialect` is process-global. One service, one schema, so that is acceptable. It
is the same constraint as a static field in Java: tests in this process must not point
goose at a second dialect.

### Layout

```text
cmd/pipelineiq-server     main only: config, signals, listen
internal/config           environment
internal/db               pool, goose, sqlc wrapper
internal/httpserver       /healthz, /readyz, shutdown
internal/store            sqlc output, do not edit
migrations                SQL, embedded
queries                   SQL that sqlc compiles
```

`cmd` is the binary. `internal` cannot be imported by another module. That is the Go rule
that keeps Jenkins Groovy and a future lab repo from depending on the service's packages.

## Go idioms, next to Java

### Errors are values

Java throws. The caller can ignore a checked exception only by declaring it. Go returns
`error` as the last result. Ignoring it is a compile-legal bug, which is why `errcheck` is
in the linter set.

Wrapping keeps the cause:

```go
return fmt.Errorf("migrate: %w", err)
```

`%w` is like `throw new IllegalStateException("migrate", cause)` except the caller branches
with `errors.Is` / `errors.As` instead of `catch`. The readiness handler logs the wrapped
error and returns a generic 503. Same as a Java controller that logs the exception and
returns a problem JSON without the SQL text.

### context is the deadline

`context.Context` is the first argument, like a request-scoped timeout you thread by hand
because Go has no thread-local request. Cancel the context and a database call returns.
`/readyz` uses `context.WithTimeout` so a stuck Postgres cannot hold the probe open past
`PIPELINEIQ_READY_TIMEOUT` (default 2s).

Java's closest thing is a request-scoped `Executor` plus a timeout on the JDBC statement.
You do not pass that timeout through every method. In this code you do. If a function does
I/O and does not take a context, that is a bug.

### Interfaces for fakes

`httpserver.Pinger` exists because the handler tests must fail `/readyz` without Postgres.
The interface is on the consumer side (the HTTP package), not on the database package.
That is the opposite of a Java `*Repository` interface declared next to the implementation
"for Spring". Do not add an interface until a test needs a fake. pgx's pool is used
directly everywhere else.

### Struct embedding

Not used in P1. When it shows up it will look like a Java subclass field that was promoted:
`Queries` embedded in a wrapper would make `store.Ping` callable as `s.Ping` without
forwarding. I did not embed it. `Store.Ping` is a real method so it can check the result
is 1 and wrap the error. Embedding would have hidden that.

### defer

`defer sqlDB.Close()` runs when the function returns, including on panic. It is the `finally`
you cannot forget, attached to the function rather than to a block. `defer cancel()` on a
timeout context is the same idea: the timer must be released. `defer pool.Close()` in
`main` runs after `Serve` returns, which is after shutdown. Closing the pool first would
fail in-flight readiness checks. Order is listen, serve, then close the pool on the way out.

pgx documents that `stdlib.OpenDBFromPool(...).Close()` does **not** close the pool. I read
that comment in pgx v5.11.0 `stdlib/sql.go`. The migrate function relies on it.

### Table-driven tests

One slice of cases, one `t.Run` per case. Same as a JUnit `@ParameterizedTest`, with the
table in the test function instead of a method source. Handler cases cover live, ready,
database down, wrong method, and unknown path. Config cases cover missing URL, defaults,
bad duration, and bad log level.

## Rules, restated, with the edges

Defaults are starting points from the plan, not measured facts. They will live in
`config.yaml` later. Nothing here quarantines a test yet.

### Parse, then attribute, then detect

Order matters. Infrastructure attribution runs before flaky detection. A test that failed
because the agent was OOM-killed is not flaky. If attribution is late, R2 and R3 learn the
wrong thing and quarantine a real test.

### R1 — Surefire rerun

Fires when the test's outcome is FLAKY in at least N builds in the window. Default N = 2
in the last 30 builds.

Edges:

- One FLAKY build is not enough. A single bad agent should not quarantine.
- FLAKY means a rerun passed. A test that fails every rerun is FAILED, not FLAKY, and does
  not satisfy R1. It might still satisfy R2 or R3.
- Builds marked `infra_failure` do not count. The plan says failures from infra builds are
  excluded from flakiness statistics.
- The window is builds, not days. A quiet repo does not age the rule out by the clock.

### R2 — same commit

Fires when the same test PASSED and FAILED on the same `commit_sha`, in non-infra builds.
Once is enough.

Edges:

- PASSED and ERROR is the same idea as PASSED and FAILED. The plan says PASSED and FAILED.
  I will treat ERROR like FAILED when P3 is written, and say so there, because an
  exception flake is still a flake. SKIPPED does not count as either side.
- Two failures and no pass on that commit is a real break, not a flake.
- Same commit on two branches is still the same SHA. That is what we want: the code is the
  same. A branch name change must not hide it.
- Infra builds are excluded on both sides. A pass on a healthy agent and a fail on an
  OOM-killed agent is not R2.

### R3 — flip rate

On the default branch, the outcome changed between consecutive runs at least F times in W
runs. Default F = 3, W = 30.

Edges:

- Only the default branch. A PR that fails once and is fixed is not a flip.
- "Consecutive runs" means consecutive builds of that test on the default branch, not
  consecutive build numbers if the test was skipped. A SKIPPED run is not a flip. P3
  should define whether SKIPPED breaks the consecutive chain. My reading: skip it, do not
  count it as a change. A change is PASSED↔FAILED/ERROR, or into/out of FLAKY.
- A genuine break then a fix is one change, not three. F = 3 is what stops that being
  called flaky.
- Fewer than W runs: do not fire. Insufficient history is not a flake.

### Quarantine

1. A rule fires → one `quarantine` row, state QUARANTINED, with the rule and evidence
   (build ids, outcomes) in `evidence` jsonb.
2. The pipeline calls `GET /api/v1/quarantine?repo=` before tests and writes the excludes
   file from P1's syntax.
3. A non-blocking stage runs only those methods. Failure marks the stage UNSTABLE, not the
   build FAILED. That is `catchError` in the library, P3.
4. Release after K consecutive passes in the quarantine stage. Default K = 10. Passes in
   the blocking stage do not count: the test was not run there.
5. Safety cap: never quarantine more than 5% of a repo's tests or 10 tests, whichever is
   smaller. Above the cap, write an alert, do not insert another active quarantine row.
   "Whichever is smaller" means a repo with 100 tests caps at 5, and a repo with 1000
   tests caps at 10. A repo with 10 tests caps at 1 (5% of 10, and 10 is larger). Integer
   percent: 5% of 19 is 0 if truncated. P3 must define rounding. I would use integer
   ceiling only if the plan said so; it does not. Truncating toward zero means a repo
   under 20 tests has a cap of 0 from the percent, so the cap is 0, and the "10" does not
   save it because the smaller one wins. That would disable quarantine for the lab repo.
   **This is an edge the plan does not resolve.** P3 should ask before picking a floor of 1.
   I am not inventing a floor in the schema.
6. Manual quarantine and release set `manual = true`. A manual row is still one active row
   per test. The partial unique index `quarantine_one_active_idx` enforces that. History
   stays: releasing sets state RELEASED, it does not delete the row.

### Regression

For each stage on the default branch, baseline is the median and MAD of the last 20
successful builds. Fewer than 10 samples → insufficient data, no alert.

```text
threshold = median + max(3 × 1.4826 × MAD, 0.2 × median)
```

1.4826 makes MAD comparable to a standard deviation for a normal distribution. It is a
constant in the formula, not a measurement from this repo.

Edges:

- Default branch: alert when 2 of the last 3 builds exceed the threshold. One spike does
  not alert. A slow agent once is a spike.
- Pull request: alert when the latest build exceeds the threshold by 50% or more, or the
  last 2 PR builds both exceed it. A PR with one build can still alert on the 50% rule.
- The same check runs on the 20 slowest tests. "Slowest" needs a definition in P4: slowest
  by median on the default branch, not slowest in the PR build (that would chase noise).
- Unsuccessful builds are not in the baseline. A failed stage's duration is often a
  timeout, which would drag the median up and hide the next regression.
- A stage that was renamed is a new series. Do not stitch names. Insufficient data until
  10 samples exist under the new name.

### Cost

Cost = agent busy seconds / 3600 × `usd_per_hour` for the instance type and lifecycle at
build time. Phase 1 uses a reference price, labelled as a reference, not a cloud bill.
The `price` table is empty in this migration. P4 seeds a reference row. I am not putting
a made-up dollar figure in the schema.

Edges:

- `agent_lifecycle` LOCAL means the number is an equivalent cost. The API and the Grafana
  label must say so. A local build with a null price is cost unknown, not zero.
- Busy seconds are the build duration, not the sum of stages, unless P4 decides stages
  overlap. Parallel stages would double-count if summed. I would use build `duration_ms`
  for the build cost and stage `duration_ms` only for a stage split that is labelled as a
  split, not as an extra bill. P4 owns that choice.
- Spot interruption (Phase 2) still has a cost: the instance was busy until it died.
  `infra_failure` does not zero the cost.

### Test impact (stretch, not P1)

Changed files against the merge base map to Maven modules, then
`mvn -pl <modules> -amd`. Full run when the root POM, `.mvn/`, the Jenkinsfile, or the
shared library changes, or when the mapping is uncertain. Uncertain means full run, not
"skip tests". A nightly full run on the default branch is what catches an escaped failure:
a test that failed on the next full run after being skipped. Report selected vs total,
time saved, and those escapes. No numbers until a run produces them.

## Data model notes that are not obvious from the table list

- Unique build key is `(job_name, build_number)`, as the plan says. `job_name` must be the
  full Jenkins name (`folder/job`), or two repos with the same short name collide. The
  schema cannot see that mistake. Ingest has to send the full name.
- `test_case` is unique on `(repository_id, class_name, method_name)`. `module` is stored
  and is not part of the key, matching the plan. Two modules with the same fully qualified
  class name would collapse into one test. Maven FQCNs are usually unique in a reactor.
  If the lab ever duplicates one, ingest must fail loudly rather than merge the rows.
- `test_run` is unique on `(build_id, test_case_id, stage)` so a blocking run and a
  quarantine run of the same method in the same build are both kept.
- Times are `timestamptz`. Durations are `bigint` milliseconds. A negative duration is
  rejected. A null duration on a build is allowed, because a crashed build may not have
  one; a `stage_run` and a `test_run` must have one, because the library and the XML both
  have a time.
- `result` is constrained to Jenkins' `SUCCESS`, `UNSTABLE`, `FAILURE`, `NOT_BUILT`,
  `ABORTED`. An unknown value fails the insert. That is deliberate: a typo should not
  become a new outcome.
- No price row is seeded. Seeding a dollar amount I did not measure would break rule 5.

## Memory budget

Phase 1 target from the plan: about 3.5 GB. Configured limits:

| Container | Limit | In P1 compose? |
| --- | --- | --- |
| pipelineiq-server | 64 MB | yes |
| Postgres | 384 MB | yes |
| Grafana | 256 MB | yes |
| Jenkins controller | 1 GB | no, P2 |
| Jenkins agent | 1.5 GB | no, P2 |

P1 compose limit sum: 64 + 384 + 256 = 704 MB.
Full Phase 1 limit sum: 704 MB + 1024 MB + 1536 MB = 3264 MB, under 3.5 GB by 236 MB
before Docker's own overhead. I have not measured resident set size. `docker stats` was
not run. The plan's "about 450 MB saved versus Spring Boot" is not a PipelineIQ number.

The server image sets `GOMEMLIMIT=48MiB` so the garbage collector aims under the 64 MB
cgroup limit instead of at it. That is a configuration choice, not a measured saving.

## Versions pinned in P1

| Thing | Pin | Source |
| --- | --- | --- |
| Go toolchain | go1.27.1 | github.com/golang/go tag go1.27.1, 2026-09-01 |
| Build image | golang:1.27.1-bookworm | docker-library/official-images library/golang |
| Runtime image | gcr.io/distroless/static-debian13:nonroot | distroless README; digest not pinned |
| Postgres image | postgres:18.6 | docker-library/official-images library/postgres (`18.6`, `18`, `latest`) |
| Grafana image | grafana/grafana:13.2.3 | grafana.com download, version 13.2.3, 2026-09-29; Docker Hub tag 13.2.3 |
| pgx | v5.11.0 | GitHub release 2026-09-07 |
| goose | v3.28.0 | GitHub release 2026-09-02 |
| sqlc | v1.31.1 | GitHub release 2026-04-22 |
| testcontainers-go | v0.44.0 | GitHub release 2026-08-07 |
| golangci-lint config | v2 schema, release v2.14.0 | GitHub release 2026-09-24; `golangci-lint run` not executed |

Postgres 19 was only a beta tag (`19beta4`) in the official-images file. Not used.
The Postgres 18 image stores data in `/var/lib/postgresql/18/docker` and declares its
volume at `/var/lib/postgresql`. Compose mounts the named volume there. Mounting the
older `/var/lib/postgresql/data` path makes the 18 entrypoint refuse to start
(https://hub.docker.com/_/postgres, "PGDATA", checked 2026-10-07). That page was read;
the container was not started.
Distroless debian12 tags are documented as no longer updated. The static debian13 image
is the current one, and a CGO-disabled Go binary does not need the base image's libc.
