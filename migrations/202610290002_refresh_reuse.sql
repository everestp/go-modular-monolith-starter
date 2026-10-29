-- +goose Up
ALTER TABLE refresh_tokens ADD COLUMN replaced_by_hash TEXT;

-- +goose Down
ALTER TABLE refresh_tokens DROP COLUMN replaced_by_hash;
