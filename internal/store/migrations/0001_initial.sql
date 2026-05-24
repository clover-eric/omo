-- +goose Up
PRAGMA foreign_keys = ON;

CREATE TABLE IF NOT EXISTS admins (
  id TEXT PRIMARY KEY,
  username TEXT NOT NULL UNIQUE,
  password_hash TEXT NOT NULL,
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS sessions (
  id TEXT PRIMARY KEY,
  admin_id TEXT NOT NULL,
  token_hash TEXT NOT NULL UNIQUE,
  expires_at TEXT NOT NULL,
  created_at TEXT NOT NULL,
  revoked_at TEXT,
  FOREIGN KEY (admin_id) REFERENCES admins(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS settings (
  key TEXT PRIMARY KEY,
  value TEXT NOT NULL,
  updated_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS domains (
  id TEXT PRIMARY KEY,
  domain TEXT NOT NULL UNIQUE,
  status TEXT NOT NULL,
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS certificates (
  id TEXT PRIMARY KEY,
  domain_id TEXT NOT NULL,
  issuer TEXT NOT NULL,
  not_before TEXT,
  not_after TEXT,
  status TEXT NOT NULL,
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL,
  FOREIGN KEY (domain_id) REFERENCES domains(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS service_modules (
  id TEXT PRIMARY KEY,
  name TEXT NOT NULL UNIQUE,
  version TEXT NOT NULL,
  status TEXT NOT NULL,
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS service_profiles (
  id TEXT PRIMARY KEY,
  module_id TEXT NOT NULL,
  profile_key TEXT NOT NULL,
  version TEXT NOT NULL,
  display_name TEXT NOT NULL,
  expert_protocol TEXT,
  status TEXT NOT NULL,
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL,
  UNIQUE (profile_key, version),
  FOREIGN KEY (module_id) REFERENCES service_modules(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS service_instances (
  id TEXT PRIMARY KEY,
  profile_id TEXT NOT NULL,
  listen_port INTEGER NOT NULL,
  status TEXT NOT NULL,
  config_version TEXT NOT NULL,
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL,
  FOREIGN KEY (profile_id) REFERENCES service_profiles(id) ON DELETE RESTRICT
);

CREATE TABLE IF NOT EXISTS distribution_tokens (
  id TEXT PRIMARY KEY,
  token_hash TEXT NOT NULL UNIQUE,
  name TEXT NOT NULL,
  status TEXT NOT NULL,
  expires_at TEXT,
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS subscription_requests (
  id TEXT PRIMARY KEY,
  distribution_token_id TEXT NOT NULL,
  client_hint TEXT,
  remote_addr_hash TEXT,
  created_at TEXT NOT NULL,
  FOREIGN KEY (distribution_token_id) REFERENCES distribution_tokens(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS clients (
  id TEXT PRIMARY KEY,
  name TEXT NOT NULL,
  status TEXT NOT NULL,
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS traffic_samples (
  id TEXT PRIMARY KEY,
  service_instance_id TEXT,
  client_id TEXT,
  rx_bytes INTEGER NOT NULL DEFAULT 0,
  tx_bytes INTEGER NOT NULL DEFAULT 0,
  sampled_at TEXT NOT NULL,
  FOREIGN KEY (service_instance_id) REFERENCES service_instances(id) ON DELETE SET NULL,
  FOREIGN KEY (client_id) REFERENCES clients(id) ON DELETE SET NULL
);

CREATE TABLE IF NOT EXISTS health_samples (
  id TEXT PRIMARY KEY,
  service_instance_id TEXT,
  status TEXT NOT NULL,
  latency_ms INTEGER,
  error_code TEXT,
  sampled_at TEXT NOT NULL,
  FOREIGN KEY (service_instance_id) REFERENCES service_instances(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS diagnostic_reports (
  id TEXT PRIMARY KEY,
  status TEXT NOT NULL,
  summary TEXT NOT NULL,
  report_json TEXT NOT NULL,
  created_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS cascade_nodes (
  id TEXT PRIMARY KEY,
  name TEXT NOT NULL,
  domain TEXT NOT NULL,
  status TEXT NOT NULL,
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS cascade_pairs (
  id TEXT PRIMARY KEY,
  source_node_id TEXT NOT NULL,
  target_node_id TEXT NOT NULL,
  status TEXT NOT NULL,
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL,
  FOREIGN KEY (source_node_id) REFERENCES cascade_nodes(id) ON DELETE CASCADE,
  FOREIGN KEY (target_node_id) REFERENCES cascade_nodes(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS jobs (
  id TEXT PRIMARY KEY,
  kind TEXT NOT NULL,
  state TEXT NOT NULL,
  status TEXT NOT NULL,
  progress INTEGER NOT NULL DEFAULT 0,
  user_message TEXT,
  internal_error_code TEXT,
  started_at TEXT,
  finished_at TEXT,
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS audit_logs (
  id TEXT PRIMARY KEY,
  actor_admin_id TEXT,
  action TEXT NOT NULL,
  resource_type TEXT NOT NULL,
  resource_id TEXT,
  details_json TEXT NOT NULL,
  created_at TEXT NOT NULL,
  FOREIGN KEY (actor_admin_id) REFERENCES admins(id) ON DELETE SET NULL
);

CREATE TABLE IF NOT EXISTS backup_records (
  id TEXT PRIMARY KEY,
  status TEXT NOT NULL,
  path TEXT NOT NULL,
  checksum TEXT,
  created_at TEXT NOT NULL,
  completed_at TEXT
);

CREATE TABLE IF NOT EXISTS update_history (
  id TEXT PRIMARY KEY,
  from_version TEXT NOT NULL,
  to_version TEXT NOT NULL,
  status TEXT NOT NULL,
  created_at TEXT NOT NULL,
  completed_at TEXT
);

CREATE INDEX IF NOT EXISTS idx_sessions_admin_id ON sessions(admin_id);
CREATE INDEX IF NOT EXISTS idx_service_instances_profile_id ON service_instances(profile_id);
CREATE INDEX IF NOT EXISTS idx_traffic_samples_sampled_at ON traffic_samples(sampled_at);
CREATE INDEX IF NOT EXISTS idx_health_samples_instance_sampled ON health_samples(service_instance_id, sampled_at);
CREATE INDEX IF NOT EXISTS idx_jobs_status ON jobs(status);
CREATE INDEX IF NOT EXISTS idx_audit_logs_created_at ON audit_logs(created_at);

-- +goose Down
DROP TABLE IF EXISTS update_history;
DROP TABLE IF EXISTS backup_records;
DROP TABLE IF EXISTS audit_logs;
DROP TABLE IF EXISTS jobs;
DROP TABLE IF EXISTS cascade_pairs;
DROP TABLE IF EXISTS cascade_nodes;
DROP TABLE IF EXISTS diagnostic_reports;
DROP TABLE IF EXISTS health_samples;
DROP TABLE IF EXISTS traffic_samples;
DROP TABLE IF EXISTS clients;
DROP TABLE IF EXISTS subscription_requests;
DROP TABLE IF EXISTS distribution_tokens;
DROP TABLE IF EXISTS service_instances;
DROP TABLE IF EXISTS service_profiles;
DROP TABLE IF EXISTS service_modules;
DROP TABLE IF EXISTS certificates;
DROP TABLE IF EXISTS domains;
DROP TABLE IF EXISTS settings;
DROP TABLE IF EXISTS sessions;
DROP TABLE IF EXISTS admins;
