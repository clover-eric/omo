-- +goose Up
ALTER TABLE service_instances ADD COLUMN display_name TEXT NOT NULL DEFAULT '';

-- +goose Down
-- SQLite does not support dropping columns in older target versions.
