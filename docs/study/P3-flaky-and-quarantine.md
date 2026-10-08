# P3 — infra attribution, flaky rules, quarantine

The prompt pasted into this session said `buildlens quarantine`. The product name is PipelineIQ.
The plan's own P3 block says `pipelineiq quarantine`. The CLI subcommand is `pipelineiq quarantine`.

No new Go module. No new Jenkins plugin. No schema change. `quarantine`, `alert`,
`build.infra_failure`, `build.infra_reason`, and `test_run.stage` were already in
`migrations/00001_init.sql`. The log tail is `migrations/00002_build_log_tail.sql`.

## What was checked

Surefire 3.6.0 `SurefireMojo.reportsDirectory` (tag `surefire-3.6.0`, file
`maven-surefire-plugin/.../SurefireMojo.java`) has no `property` attribute. A command-line
`-Dsurefire.reportsDirectory=...` does not move the reports. `excludesFile` does have
`property = "surefire.excludesFile"`. That matches the P1 note.

`surefire.excludesFile` is resolved from each module directory. The library passes the
absolute workspace path from `pwd()`.

So the library does not pass a reports-directory flag. It runs the blocking Maven command,
moves `target/surefire-reports` to `target/blocking-reports`, runs the quarantine command,
and moves the new reports to `target/quarantine-reports`. The CLI uploads both. The stage
is the directory name, not a second HTTP field. I have not executed Surefire against a
generated excludes file. The syntax is the one P1 checked.

`timedStage` parses `stages.json` in an `@NonCPS` method. `JsonSlurper` is not
serializable, and a library step that calls `timedStage` otherwise cannot record
the stage. The file is also reset when `BUILD_NUMBER` changes. The agent workspace
is reused, and a leftover file makes the next build upload two rows with the same
stage name. That unique violation rolls the whole ingest back, so the flaky runs
never reach R1.

`catchError` is called with `stageResult: 'UNSTABLE'` and `buildResult` set to
`currentBuild.currentResult`. That keeps the build result where it was before the
quarantine stage and marks only the stage unstable. I did not re-read the step source
in this session. The unit test checks that those arguments are passed.

Spot interruption is Phase 2 (`infra_event`). A log line that only says "Spot interruption"
does not match a P3 rule. That is tested.

## Order: attribute, then detect

Infrastructure attribution runs inside the ingest transaction, after the rows are written
and before any rule reads them. A failure on an OOM-killed agent is not a flake. If the
rule ran first, R2 would see PASSED on a healthy build and FAILED on the killed agent and
quarantine a real test.

The current build is included. Re-uploading the same job and build number replaces its
test runs and recomputes infra and the quarantine streak from the stored rows. It does
not increment a counter, so a retry cannot release a test by accident.

## Infra rules

First match wins. The reason is a stable token stored in `infra_reason`.

| Token | Matches | Near-miss that must not match |
| --- | --- | --- |
| `AGENT_LOST` | `hudson.remoting.ChannelClosedException`, `Unexpected termination of the channel`, `Agent went offline` | `ClosedChannelException` from the application, or the lowercase sentence "unexpected termination of the channel" |
| `OUT_OF_MEMORY` | `OOMKilled`, `java.lang.OutOfMemoryError`, `exit code 137` not followed by a digit | `exit code 1370` |
| `DISK_FULL` | `No space left on device` | `no space left in the device table` |
| `DOCKER_UNAVAILABLE` | `Cannot connect to the Docker daemon`, `Is the docker daemon running` | `Cannot connect to the Docker registry` |
| `DEPENDENCY_FETCH` | A Maven resolve or transfer failure and a timeout in the same tail | A resolve failure with no timeout, or a timeout with no Maven resolve line |

A missing artifact is a build break. A timeout in a test is a test failure. Neither is
`DEPENDENCY_FETCH` unless both signals are in the tail. That is the false positive the
rule is built to avoid.

## R1, R2, R3

Thresholds default to the plan: R1 is 2 FLAKY builds in the last 30 non-infra builds of
the repo, R2 is one commit, R3 is 3 flips in 30 runs. They can be overridden by a flat
`key: value` file at `PIPELINEIQ_RULES_FILE`. No YAML library was added.

R1 does not require 30 builds to exist. Two FLAKY builds are enough. Infra builds are
left out of the window. One FLAKY build does not fire. FLAKY means a rerun passed. A
test that fails every rerun is FAILED and does not satisfy R1.

R2 fires when the same commit has PASSED and FAILED, or PASSED and ERROR, in non-infra
builds. ERROR is treated as a failure. The P1 note said that would be stated here. SKIPPED
is neither side. Two failures and no pass are a break, not a flake. A pass on a healthy
agent and a fail on an infra build do not fire. The same SHA on two branches still counts:
the code is the same.

R3 looks only at the repository's `default_branch` (the column, default `main`). Infra
builds are excluded. SKIPPED runs are omitted. They are not a flip and they do not break
the chain. A change is PASSED against FAILED/ERROR, or into or out of FLAKY. FAILED to
ERROR is not a change. Fewer than 30 kept runs do not fire. A break then a fix is one or
two changes, not three, so it does not fire at F = 3.

First matching rule wins, in order R1, R2, R3. The evidence JSON records that rule, the
build ids, and the outcomes.

## Quarantine and the cap

One active row per test. The partial unique index already enforces that. Release sets
`state = RELEASED` and `released_at`. It does not delete the row.

Automatic quarantine is refused when the number of active rows is already at the cap.
The refusal writes an `alert` of type `FLAKY` and does not insert a quarantine row.
Manual quarantine is allowed over the cap. A person asked for it. It still counts toward
the cap for later automatic rules. The plan's step 6 is separate from the cap step.

The cap is `min(10, tests * 5 / 100)`, integer division. The plan says whichever is
smaller. Truncation makes that 0 for any repo under 20 tests, and the smaller-of-zero-and-10
reading would disable quarantine on the lab. The done criteria require a quarantine on
that lab. P1 said to ask before inventing a floor. This prompt's done criteria cannot be
met with a floor of 0, so the floor is 1 when the repo has at least one test. A repo with
no tests still has a cap of 0. This is a rule decision, not a schema column.

Release looks at `test_run.stage = QUARANTINE` with `build.id` greater than the build
that created the quarantine (`since_build_id` in the evidence). K consecutive PASSED
releases the row. Default K = 10. A FAILED, ERROR, FLAKY, or SKIPPED run breaks the
streak. A pass in the blocking stage does not count: the test was not supposed to run
there. The streak is recomputed from the rows on every ingest.

`GET /api/v1/quarantine?repo=` returns the active list. It uses the same bearer key as
ingest. An unknown repo returns an empty list, not 404, so the CLI excludes nothing.
`POST /api/v1/quarantine` and `POST /api/v1/quarantine/release` are the manual endpoints.

## CLI

`pipelineiq quarantine --server --key --repo --format surefire-excludes|surefire-only --out`.

`surefire-excludes` is the P1 file: a comment line, then `class#method` lines. Blank lines
and `#` lines are ignored by Surefire. `surefire-only` is the `-Dtest` value:
`Class#m1+m2,Other#m3`. Methods are sorted. Classes are sorted.

If the server cannot be reached, or it returns 5xx, the CLI writes an empty file, warns,
and exits 0. The build then runs every test. A 401 is not that case. The CLI exits 2 and
does not write a file that would hide a bad key.

## Library

`quarantineAwareTests` fetches both files, runs the blocking stage with
`surefire.excludesFile` and `failsafe.excludesFile`, then the quarantine stage with
`-Dtest` only when the only-file has a pattern. That stage also sets
`surefire.failIfNoSpecifiedTests=false`, because `-Dtest=` is applied to every
module and core/api do not contain the quarantined method. The quarantine stage is inside
`catchError`. `reportBuild` is still the upload, in `post { always }` of the lab
Jenkinsfile. The library echoes the excludes file so the Jenkins console shows the
exclusion without a UI click.

## Lab

`../pipelineiq-lab` is recreated here. It was not on GitHub. The three injected tests are
labelled in the source and the README:

- random, about 20 percent: build number modulo 5 equals 0. The first attempt fails, the
  rerun passes, so Surefire records FLAKY. Two such builds satisfy R1.
- time-based: fails when the UTC minute modulo 5 equals 0. `-Dpipelineiq.flaky.epochSecond`
  replays a chosen second.
- order-dependent: `dependsOnPartner` fails unless `partner` already ran. The order comes
  from `-Dpipelineiq.flaky.orderSeed`. Seed 1 runs `partner` first. Seed 2147483648 runs
  the dependent method first.

The verification job pins the time and order seeds so only the random test flakes. The
cap for this lab is 1. A second flake would take that slot and the demo would depend on
which rule fired first.

The verification job copies `.github/lab-fixture/flaky-lab` onto the generated lab. The
lab repository itself is not in this git repo and was not pushed.

## Not verified here

This workspace has no Go toolchain and no Docker. `gofmt`, `go vet`, golangci-lint, the
race tests, JenkinsPipelineUnit, and the 20-build loop are verified in GitHub Actions if
that run is green, and nowhere else. Surefire was not executed against a generated
excludes file on this machine.
