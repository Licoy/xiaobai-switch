
PRAGMA foreign_keys = ON;
CREATE TABLE settings (
  id INTEGER PRIMARY KEY CHECK (id = 1),
  json TEXT NOT NULL
);
CREATE TABLE sites (
  id TEXT PRIMARY KEY,
  name TEXT NOT NULL,
  base_url TEXT NOT NULL,
  api_key_encrypted TEXT NOT NULL,
  key_prefix TEXT NOT NULL,
  protocol TEXT NOT NULL DEFAULT 'openai_compatible',
  claude_auth_key_style TEXT NOT NULL DEFAULT 'anthropic_auth_token',
  notes TEXT,
  enabled INTEGER NOT NULL DEFAULT 1,
  sort_order INTEGER NOT NULL DEFAULT 0,
  selected_model_id TEXT,
  last_model_fetch_at INTEGER,
  last_model_fetch_latency_ms INTEGER,
  last_model_fetch_error TEXT,
  created_at INTEGER NOT NULL,
  updated_at INTEGER NOT NULL,
  base_urls_json TEXT,
  capabilities_json TEXT
);
CREATE TABLE site_models (
  id TEXT PRIMARY KEY,
  site_id TEXT NOT NULL REFERENCES sites(id) ON DELETE CASCADE,
  model_id TEXT NOT NULL,
  display_name TEXT NOT NULL,
  owned_by TEXT,
  raw_json TEXT,
  is_manual INTEGER NOT NULL DEFAULT 0,
  UNIQUE(site_id, model_id)
);
CREATE TABLE site_model_exclusions (
  site_id TEXT NOT NULL REFERENCES sites(id) ON DELETE CASCADE,
  model_id TEXT NOT NULL,
  PRIMARY KEY (site_id, model_id)
);
CREATE TABLE target_bindings (
  target TEXT PRIMARY KEY,
  site_id TEXT,
  site_name_snapshot TEXT NOT NULL,
  model_id TEXT NOT NULL,
  provider_id TEXT,
  key_fingerprint TEXT NOT NULL,
  managed_paths_json TEXT NOT NULL,
  managed_env_keys_json TEXT NOT NULL,
  expected_fields_json TEXT NOT NULL,
  orphan INTEGER NOT NULL DEFAULT 0,
  apply_record_id TEXT,
  applied_at INTEGER NOT NULL,
  FOREIGN KEY (site_id) REFERENCES sites(id) ON DELETE SET NULL
);
CREATE TABLE apply_records (
  id TEXT PRIMARY KEY,
  site_id TEXT,
  site_name_snapshot TEXT NOT NULL,
  target TEXT NOT NULL,
  model_id TEXT NOT NULL,
  provider_id TEXT,
  status TEXT NOT NULL,
  backup_dir TEXT,
  touched_keys_json TEXT NOT NULL,
  config_snapshot_hash TEXT,
  error TEXT,
  applied_at INTEGER NOT NULL,
  FOREIGN KEY (site_id) REFERENCES sites(id) ON DELETE SET NULL
);
