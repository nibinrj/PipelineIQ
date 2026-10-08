-- PipelineIQ schema, taken from docs/plan.md "Tables (PostgreSQL, SQL migrations)".
-- Times are timestamptz (UTC). Durations are milliseconds.
-- goose applies this file. sqlc reads the same file as the schema.

-- +goose Up

CREATE TABLE repository (
    id              bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    name            text NOT NULL,
    default_branch  text NOT NULL DEFAULT 'main',
    created_at      timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT repository_name_unique UNIQUE (name),
    CONSTRAINT repository_name_not_blank CHECK (length(btrim(name)) > 0),
    CONSTRAINT repository_default_branch_not_blank CHECK (length(btrim(default_branch)) > 0)
);

CREATE TABLE build (
    id                    bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    repository_id         bigint NOT NULL REFERENCES repository (id),
    job_name              text NOT NULL,
    build_number          integer NOT NULL,
    branch                text NOT NULL,
    pr_number             integer,
    commit_sha            text NOT NULL,
    result                text NOT NULL,
    started_at            timestamptz,
    finished_at           timestamptz,
    duration_ms           bigint,
    agent_name            text,
    agent_instance_type   text,
    agent_lifecycle       text NOT NULL DEFAULT 'LOCAL',
    infra_failure         boolean NOT NULL DEFAULT false,
    infra_reason          text,
    cost_usd              numeric(14, 6),
    created_at            timestamptz NOT NULL DEFAULT now(),
    updated_at            timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT build_job_number_unique UNIQUE (job_name, build_number),
    CONSTRAINT build_number_positive CHECK (build_number > 0),
    CONSTRAINT build_pr_number_positive CHECK (pr_number IS NULL OR pr_number > 0),
    CONSTRAINT build_duration_non_negative CHECK (duration_ms IS NULL OR duration_ms >= 0),
    CONSTRAINT build_cost_non_negative CHECK (cost_usd IS NULL OR cost_usd >= 0),
    CONSTRAINT build_lifecycle_known CHECK (agent_lifecycle IN ('LOCAL', 'SPOT', 'ON_DEMAND')),
    CONSTRAINT build_result_known CHECK (result IN ('SUCCESS', 'UNSTABLE', 'FAILURE', 'NOT_BUILT', 'ABORTED')),
    CONSTRAINT build_job_name_not_blank CHECK (length(btrim(job_name)) > 0),
    CONSTRAINT build_branch_not_blank CHECK (length(btrim(branch)) > 0),
    CONSTRAINT build_commit_not_blank CHECK (length(btrim(commit_sha)) > 0)
);

CREATE INDEX build_repository_id_idx ON build (repository_id);
CREATE INDEX build_repository_branch_started_idx ON build (repository_id, branch, started_at);

CREATE TABLE stage_run (
    id          bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    build_id    bigint NOT NULL REFERENCES build (id) ON DELETE CASCADE,
    name        text NOT NULL,
    started_at  timestamptz,
    duration_ms bigint NOT NULL,
    result      text NOT NULL,
    CONSTRAINT stage_run_build_name_unique UNIQUE (build_id, name),
    CONSTRAINT stage_run_duration_non_negative CHECK (duration_ms >= 0),
    CONSTRAINT stage_run_name_not_blank CHECK (length(btrim(name)) > 0)
);

CREATE INDEX stage_run_build_id_idx ON stage_run (build_id);

CREATE TABLE test_case (
    id             bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    repository_id  bigint NOT NULL REFERENCES repository (id),
    module         text NOT NULL DEFAULT '',
    class_name     text NOT NULL,
    method_name    text NOT NULL,
    -- Plan key: unique per repository + class + method. module is stored but is not
    -- part of the key. Two Maven modules with the same fully qualified class name
    -- would collide; ingest must not invent a second identity. See docs/study/P1-design.md.
    CONSTRAINT test_case_identity_unique UNIQUE (repository_id, class_name, method_name),
    CONSTRAINT test_case_class_not_blank CHECK (length(btrim(class_name)) > 0),
    CONSTRAINT test_case_method_not_blank CHECK (length(btrim(method_name)) > 0)
);

CREATE INDEX test_case_repository_id_idx ON test_case (repository_id);

CREATE TABLE test_run (
    id              bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    build_id        bigint NOT NULL REFERENCES build (id) ON DELETE CASCADE,
    test_case_id    bigint NOT NULL REFERENCES test_case (id),
    outcome         text NOT NULL,
    duration_ms     bigint NOT NULL,
    rerun_failures  integer NOT NULL DEFAULT 0,
    failure_type    text,
    failure_hash    text,
    stage           text NOT NULL DEFAULT 'BLOCKING',
    CONSTRAINT test_run_build_case_stage_unique UNIQUE (build_id, test_case_id, stage),
    CONSTRAINT test_run_outcome_known CHECK (outcome IN ('PASSED', 'FAILED', 'ERROR', 'SKIPPED', 'FLAKY')),
    CONSTRAINT test_run_stage_known CHECK (stage IN ('BLOCKING', 'QUARANTINE')),
    CONSTRAINT test_run_duration_non_negative CHECK (duration_ms >= 0),
    CONSTRAINT test_run_reruns_non_negative CHECK (rerun_failures >= 0)
);

CREATE INDEX test_run_test_case_id_idx ON test_run (test_case_id);
CREATE INDEX test_run_build_id_idx ON test_run (build_id);

CREATE TABLE quarantine (
    id                  bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    test_case_id        bigint NOT NULL REFERENCES test_case (id),
    state               text NOT NULL,
    reason_rule         text NOT NULL,
    evidence            jsonb NOT NULL DEFAULT '{}'::jsonb,
    quarantined_at      timestamptz NOT NULL,
    released_at         timestamptz,
    consecutive_passes  integer NOT NULL DEFAULT 0,
    manual              boolean NOT NULL DEFAULT false,
    CONSTRAINT quarantine_state_known CHECK (state IN ('QUARANTINED', 'RELEASED')),
    CONSTRAINT quarantine_passes_non_negative CHECK (consecutive_passes >= 0),
    CONSTRAINT quarantine_reason_not_blank CHECK (length(btrim(reason_rule)) > 0),
    CONSTRAINT quarantine_released_after_start CHECK (released_at IS NULL OR released_at >= quarantined_at)
);

-- History is kept. One active row per test.
CREATE UNIQUE INDEX quarantine_one_active_idx
    ON quarantine (test_case_id)
    WHERE state = 'QUARANTINED';

CREATE INDEX quarantine_test_case_id_idx ON quarantine (test_case_id);

CREATE TABLE alert (
    id              bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    type            text NOT NULL,
    build_id        bigint REFERENCES build (id),
    repository_id   bigint NOT NULL REFERENCES repository (id),
    payload         jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at      timestamptz NOT NULL DEFAULT now(),
    pr_comment_id   bigint,
    CONSTRAINT alert_type_known CHECK (type IN ('FLAKY', 'REGRESSION', 'COST'))
);

CREATE INDEX alert_repository_id_idx ON alert (repository_id);
CREATE INDEX alert_build_id_idx ON alert (build_id);

CREATE TABLE price (
    id              bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    instance_type   text NOT NULL,
    lifecycle       text NOT NULL,
    region          text NOT NULL,
    usd_per_hour    numeric(14, 6) NOT NULL,
    effective_from  timestamptz NOT NULL,
    source          text NOT NULL,
    CONSTRAINT price_lifecycle_known CHECK (lifecycle IN ('LOCAL', 'SPOT', 'ON_DEMAND')),
    CONSTRAINT price_usd_non_negative CHECK (usd_per_hour >= 0),
    CONSTRAINT price_instance_not_blank CHECK (length(btrim(instance_type)) > 0),
    CONSTRAINT price_region_not_blank CHECK (length(btrim(region)) > 0),
    CONSTRAINT price_source_not_blank CHECK (length(btrim(source)) > 0)
);

CREATE INDEX price_lookup_idx ON price (instance_type, lifecycle, region, effective_from);

CREATE TABLE infra_event (
    id           bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    instance_id  text NOT NULL,
    event_type   text NOT NULL,
    event_time   timestamptz NOT NULL,
    raw          jsonb NOT NULL DEFAULT '{}'::jsonb,
    CONSTRAINT infra_event_instance_not_blank CHECK (length(btrim(instance_id)) > 0),
    CONSTRAINT infra_event_type_not_blank CHECK (length(btrim(event_type)) > 0)
);

CREATE INDEX infra_event_instance_time_idx ON infra_event (instance_id, event_time);

-- +goose Down

DROP TABLE IF EXISTS infra_event;
DROP TABLE IF EXISTS price;
DROP TABLE IF EXISTS alert;
DROP TABLE IF EXISTS quarantine;
DROP TABLE IF EXISTS test_run;
DROP TABLE IF EXISTS test_case;
DROP TABLE IF EXISTS stage_run;
DROP TABLE IF EXISTS build;
DROP TABLE IF EXISTS repository;
