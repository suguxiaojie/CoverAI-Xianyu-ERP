-- +goose Up
ALTER TABLE account_task_settings ADD COLUMN request_flower_after_seconds INT NOT NULL DEFAULT 10;
UPDATE account_task_settings
SET request_flower_after_seconds = CASE
    WHEN auto_request_flower_enabled = 1 THEN request_flower_after_hours * 3600
    ELSE 10
END;

-- +goose Down
ALTER TABLE account_task_settings DROP COLUMN request_flower_after_seconds;
