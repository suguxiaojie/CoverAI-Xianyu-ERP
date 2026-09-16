-- +goose Up
ALTER TABLE account_task_settings ADD COLUMN auto_request_flower_enabled INTEGER NOT NULL DEFAULT 0;
ALTER TABLE account_task_settings ADD COLUMN request_flower_after_hours INTEGER NOT NULL DEFAULT 24;
ALTER TABLE account_task_settings ADD COLUMN auto_receive_flower_enabled INTEGER NOT NULL DEFAULT 0;
ALTER TABLE account_task_settings ADD COLUMN receive_flower_show_browser INTEGER NOT NULL DEFAULT 1;
ALTER TABLE account_task_settings ADD COLUMN receive_flower_timeout_seconds INTEGER NOT NULL DEFAULT 120;

-- +goose Down
ALTER TABLE account_task_settings DROP COLUMN receive_flower_timeout_seconds;
ALTER TABLE account_task_settings DROP COLUMN receive_flower_show_browser;
ALTER TABLE account_task_settings DROP COLUMN auto_receive_flower_enabled;
ALTER TABLE account_task_settings DROP COLUMN request_flower_after_hours;
ALTER TABLE account_task_settings DROP COLUMN auto_request_flower_enabled;
