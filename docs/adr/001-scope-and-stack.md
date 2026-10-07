# ADR 001 — Scope and stack

Date: 2026-10-07. Status: accepted for Phase 1.

## Context

PipelineIQ has to sit next to Jenkins, remember every build, and later tell the next build
which tests to skip. The owner is a Java developer. The plan forbids Kafka, microservices,
ML, and a web framework. P1 has to pick the database migration tool and record why the
other boundaries stay where the plan put them.

## Decision

One Go process, `pipelineiq-server`, plus a later static CLI. Postgres is the only datastore.
Jenkins is the only CI server. Maven Surefire and Failsafe are the only test reports.
Detection rules are deterministic SQL and Go, not a model. Schema changes go through goose
SQL migrations. Queries are SQL in `queries/`, compiled by sqlc against pgx v5.

## Why one service

The loop is small: an agent uploads a build, the service writes rows, a later agent reads
the quarantine list. Splitting that into ingest, rules, and comments services would add a
network hop and a second failure mode for no traffic this project has. One binary also fits
the memory budget: the container limit is 64 MB. A Spring Boot service of the same shape
was the thing the original plan was trying to get away from; that comparison was not
remeasured here and is not a PipelineIQ result.

Grafana is a dashboard, not a second application we write. Jenkins is the CI server, not a
library we embed. Those are the other processes. They are not microservices of PipelineIQ.

## Why Go

The agent CLI has to be a static binary Jenkins agents can run with no JVM. The service
should be the same language so the report format and the parser are not translated twice.
`net/http` and `log/slog` are in the standard library, so P1 adds no web framework.

Java would be the familiar choice. It is the wrong one for the agent: a JRE on every
inbound agent costs memory the plan already gave to Maven (1.5 GB). One Go toolchain
covers the service, the CLI, and later the MCP server.

## Why Jenkins and Maven only

The lab subject is a multi-module Maven build. Surefire and Failsafe XML is the report
format the parser will trust. Supporting Gradle or GitHub Actions in Phase 1 would mean a
second report format before the first one has a fixture test. The plan's non-goals say no
multi-CI support. P2 will not add a plugin or a parser outside that boundary without asking.

## Why no ML

A flaky test here is a rule with evidence: a Surefire rerun, the same commit passing and
failing, or a flip rate on the default branch. Those rules can be wrong, and the wrongness
is a row you can read. A model would hide the evidence and would need a training set this
repo does not have. IDoFT is a benchmark for the rules, not a training set. If a rule is
noisy, the threshold changes in config. That is the tuning loop, not a model.

## Why goose, not golang-migrate

Both can apply SQL files. goose v3.28.0 (GitHub release 2026-09-02) is a library the
server calls on startup (`goose.UpContext`) with the SQL embedded in the binary. The
distroless image then needs no shell and no second migrate binary. golang-migrate can also
be embedded; it was not selected, and its version was not pinned, because the service
does not ship it. The choice is recorded here so a later prompt does not add the other tool
beside goose.

## Why sqlc + pgx, not plain pgx and not an ORM

pgx v5.11.0 (GitHub release 2026-09-07) is the driver. sqlc v1.31.1 (GitHub release
2026-04-22) compiles the SQL in `queries/` into Go functions. A wrong column name fails
the build, which is the closest thing in this stack to a checked Spring Data repository
without becoming an ORM. Plain pgx would leave every query as a string until a test hits
it. An ORM would hide the SQL the plan wants written down.

## Consequences

- P2 ingest must be idempotent on `(job_name, build_number)` because that unique key is
  already in the first migration.
- A schema change is a new goose file, not an edit of `00001_init.sql`, once this migration
  has been applied anywhere that matters.
- Interfaces stay rare. The HTTP package has a `Pinger` because the handler tests need a
  fake. The pool is not hidden behind a repository interface in P1.
