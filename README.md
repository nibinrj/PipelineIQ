# PipelineIQ

CI intelligence for Jenkins. It will learn from every build and act on flaky tests, slow stages,
and build cost. Phase 1 runs locally. This repository is at prompt P1: design notes and a Go
skeleton. Ingest, Jenkins, and the rules are not built yet.

The product name is PipelineIQ. The service binary is `pipelineiq-server`. The module path is
`github.com/nibinrj/PipelineIQ`.

## P1 stack

| Piece | Pin | Where the pin came from |
| --- | --- | --- |
| Go | 1.27.1 | `github.com/golang/go` tag `go1.27.1` (2026-09-01) |
| Postgres image | `postgres:18.6` | `docker-library/official-images` `library/postgres` |
| Grafana image | `grafana/grafana:13.2.3` | Grafana download page, 2026-09-29; Docker Hub tag `13.2.3` |
| Runtime image | `gcr.io/distroless/static-debian13:nonroot` | distroless README; digest not pinned, see the study note |
| pgx | v5.11.0 | GitHub release 2026-09-07 |
| goose | v3.28.0 | GitHub release 2026-09-02 |
| sqlc | v1.31.1 | GitHub release 2026-04-22 |

## Run locally

Windows, PowerShell 7, Docker Desktop:

```powershell
Copy-Item .env.example .env
.\tasks.ps1 up
.\tasks.ps1 test
.\tasks.ps1 lint
```

`up` starts Postgres (`:5432`), Grafana (`:3000`), and pipelineiq-server (`:8080`).
Configured memory limits for those three are 384 + 256 + 64 = 704 MB. That is a limit, not a measurement.

- `GET /healthz` is liveness and does not touch the database.
- `GET /readyz` runs `SELECT 1`. It returns 503 if the database is down.
- Migrations run once at startup. A second run is a no-op.

## What is not in P1

No Jenkins, no CLI, no Surefire parser, no flaky rules, no quarantine HTTP API.
Those are P2 and P3. The study note for this prompt is `docs/study/P1-design.md`.
