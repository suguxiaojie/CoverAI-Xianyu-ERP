-- +goose Up
ALTER TABLE keywords ADD COLUMN group_id VARCHAR(255) NOT NULL DEFAULT '';
ALTER TABLE keywords ADD COLUMN match_type VARCHAR(32) NOT NULL DEFAULT 'contains';
ALTER TABLE keywords ADD COLUMN message_scope VARCHAR(32) NOT NULL DEFAULT 'customer';
ALTER TABLE keywords ADD COLUMN system_types VARCHAR(255) NOT NULL DEFAULT '';
UPDATE keywords SET group_id=CONCAT('legacy-',id) WHERE group_id='';
CREATE INDEX idx_keywords_cookie_group ON keywords(cookie_id,group_id,id);

-- +goose Down
DROP INDEX idx_keywords_cookie_group ON keywords;
ALTER TABLE keywords DROP COLUMN group_id;
ALTER TABLE keywords DROP COLUMN match_type;
ALTER TABLE keywords DROP COLUMN message_scope;
ALTER TABLE keywords DROP COLUMN system_types;
