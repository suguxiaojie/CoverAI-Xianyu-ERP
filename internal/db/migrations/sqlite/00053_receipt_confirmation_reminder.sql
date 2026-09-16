-- +goose Up
ALTER TABLE account_task_settings ADD COLUMN auto_receipt_reminder_enabled INTEGER NOT NULL DEFAULT 0;
ALTER TABLE account_task_settings ADD COLUMN receipt_reminder_after_days INTEGER NOT NULL DEFAULT 2;
ALTER TABLE account_task_settings ADD COLUMN receipt_reminder_time TEXT NOT NULL DEFAULT '10:00';
ALTER TABLE account_task_settings ADD COLUMN receipt_reminder_message TEXT NOT NULL DEFAULT '您好，订单已经发货一段时间。确认商品信息无误后，麻烦在闲鱼确认收货，谢谢。';
ALTER TABLE account_task_settings ADD COLUMN receipt_reminder_enabled_at INTEGER NOT NULL DEFAULT 0;

-- +goose Down
ALTER TABLE account_task_settings DROP COLUMN receipt_reminder_enabled_at;
ALTER TABLE account_task_settings DROP COLUMN receipt_reminder_message;
ALTER TABLE account_task_settings DROP COLUMN receipt_reminder_time;
ALTER TABLE account_task_settings DROP COLUMN receipt_reminder_after_days;
ALTER TABLE account_task_settings DROP COLUMN auto_receipt_reminder_enabled;
