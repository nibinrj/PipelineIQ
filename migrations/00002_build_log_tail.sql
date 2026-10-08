-- P2 uploads the last 500 log lines. The plan's table list has no column for them.
-- P3 infra rules read this tail. A new migration, not an edit of 00001.

-- +goose Up

ALTER TABLE build ADD COLUMN log_tail text;

-- +goose Down

ALTER TABLE build DROP COLUMN log_tail;
