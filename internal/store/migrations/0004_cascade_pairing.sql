-- +goose Up
ALTER TABLE cascade_nodes ADD COLUMN role TEXT NOT NULL DEFAULT 'remote';
ALTER TABLE cascade_nodes ADD COLUMN trust_key_fingerprint TEXT NOT NULL DEFAULT '';
ALTER TABLE cascade_nodes ADD COLUMN online INTEGER NOT NULL DEFAULT 0;
ALTER TABLE cascade_nodes ADD COLUMN latency_ms INTEGER NOT NULL DEFAULT 0;
ALTER TABLE cascade_nodes ADD COLUMN throughput_mbps REAL NOT NULL DEFAULT 0;
ALTER TABLE cascade_nodes ADD COLUMN last_error TEXT NOT NULL DEFAULT '';

ALTER TABLE cascade_pairs ADD COLUMN config_state TEXT NOT NULL DEFAULT 'pending_apply';

CREATE TABLE IF NOT EXISTS pairing_codes (
  id TEXT PRIMARY KEY,
  node_id TEXT NOT NULL,
  node_name TEXT NOT NULL,
  domain TEXT NOT NULL,
  code_hash TEXT NOT NULL UNIQUE,
  public_key TEXT NOT NULL,
  signature TEXT NOT NULL,
  status TEXT NOT NULL,
  expires_at TEXT NOT NULL,
  used_at TEXT,
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_pairing_codes_status_expires ON pairing_codes(status, expires_at);
CREATE INDEX IF NOT EXISTS idx_cascade_pairs_source_target ON cascade_pairs(source_node_id, target_node_id);

-- +goose Down
DROP TABLE IF EXISTS pairing_codes;
