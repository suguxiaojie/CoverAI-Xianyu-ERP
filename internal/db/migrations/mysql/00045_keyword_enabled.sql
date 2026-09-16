-- +goose Up
ALTER TABLE keywords ADD COLUMN enabled TINYINT(1) NOT NULL DEFAULT 1;
CREATE INDEX idx_keywords_cookie_enabled_group ON keywords(cookie_id,enabled,group_id,id);

-- +goose Down
DROP INDEX idx_keywords_cookie_enabled_group ON keywords;
ALTER TABLE keywords DROP COLUMN enabled;
