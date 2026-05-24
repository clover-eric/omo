-- +goose Up
CREATE TABLE IF NOT EXISTS login_rate_limits (
  username TEXT PRIMARY KEY,
  failure_count INTEGER NOT NULL DEFAULT 0,
  locked_until TEXT,
  updated_at TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_login_rate_limits_locked_until ON login_rate_limits(locked_until);

-- +goose Down
DROP TABLE IF EXISTS login_rate_limits;
