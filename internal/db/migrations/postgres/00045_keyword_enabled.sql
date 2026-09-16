-- +goose Up
ALTER TABLE keywords ADD COLUMN enabled BOOLEAN NOT NULL DEFAULT TRUE;
CREATE INDEX idx_keywords_cookie_enabled_group ON keywords(cookie_id,enabled,group_id,id);

-- +goose Down
DROP INDEX IF EXISTS idx_keywords_cookie_enabled_group;
ALTER TABLE keywords DROP COLUMN enabled;
