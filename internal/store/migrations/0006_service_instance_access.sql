-- +goose Up
ALTER TABLE service_instances ADD COLUMN access_username TEXT NOT NULL DEFAULT '';
ALTER TABLE service_instances ADD COLUMN access_password TEXT NOT NULL DEFAULT '';
ALTER TABLE service_instances ADD COLUMN access_path TEXT NOT NULL DEFAULT '';

-- +goose Down
-- SQLite does not support dropping columns in older target versions.
