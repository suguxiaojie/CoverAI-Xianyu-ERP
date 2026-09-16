-- +goose Up
ALTER TABLE keywords ADD COLUMN group_id TEXT NOT NULL DEFAULT '';
ALTER TABLE keywords ADD COLUMN match_type TEXT NOT NULL DEFAULT 'contains';
ALTER TABLE keywords ADD COLUMN message_scope TEXT NOT NULL DEFAULT 'customer';
ALTER TABLE keywords ADD COLUMN system_types TEXT NOT NULL DEFAULT '';
UPDATE keywords SET group_id='legacy-' || id WHERE group_id='';
CREATE INDEX idx_keywords_cookie_group ON keywords(cookie_id,group_id,id);

-- +goose Down
DROP INDEX IF EXISTS idx_keywords_cookie_group;
ALTER TABLE keywords DROP COLUMN group_id;
ALTER TABLE keywords DROP COLUMN match_type;
ALTER TABLE keywords DROP COLUMN message_scope;
ALTER TABLE keywords DROP COLUMN system_types;
