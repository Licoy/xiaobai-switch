
CREATE TABLE IF NOT EXISTS settings (
  id INTEGER PRIMARY KEY CHECK (id = 1),
  json TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS sites (
  id TEXT PRIMARY KEY,
  name TEXT NOT NULL,
  base_url TEXT NOT NULL,
  protocol TEXT NOT NULL DEFAULT 'openai_compatible',
  claude_auth_key_style TEXT NOT NULL DEFAULT 'anthropic_auth_token',
  notes TEXT,
  enabled INTEGER NOT NULL DEFAULT 1,
  sort_order INTEGER NOT NULL DEFAULT 0,
  created_at INTEGER NOT NULL,
  updated_at INTEGER NOT NULL,
  base_urls_json TEXT,
  capabilities_json TEXT
);

CREATE TABLE IF NOT EXISTS site_api_keys (
  id TEXT PRIMARY KEY,
  site_id TEXT NOT NULL REFERENCES sites(id) ON DELETE CASCADE,
  label TEXT NOT NULL,
  api_key_encrypted TEXT NOT NULL,
  key_prefix TEXT NOT NULL,
  is_active INTEGER NOT NULL DEFAULT 0,
  selected_model_id TEXT,
  last_model_fetch_at INTEGER,
  last_model_fetch_latency_ms INTEGER,
  last_model_fetch_error TEXT,
  created_at INTEGER NOT NULL,
  updated_at INTEGER NOT NULL
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_site_api_keys_one_active
  ON site_api_keys(site_id) WHERE is_active = 1;
CREATE UNIQUE INDEX IF NOT EXISTS idx_site_api_keys_label
  ON site_api_keys(site_id, lower(label));

CREATE TABLE IF NOT EXISTS site_models (
  id TEXT PRIMARY KEY,
  site_id TEXT NOT NULL REFERENCES sites(id) ON DELETE CASCADE,
  api_key_id TEXT NOT NULL REFERENCES site_api_keys(id) ON DELETE CASCADE,
  model_id TEXT NOT NULL,
  display_name TEXT NOT NULL,
  owned_by TEXT,
  raw_json TEXT,
  is_manual INTEGER NOT NULL DEFAULT 0,
  UNIQUE(api_key_id, model_id)
);

CREATE TABLE IF NOT EXISTS site_model_exclusions (
  site_id TEXT NOT NULL REFERENCES sites(id) ON DELETE CASCADE,
  api_key_id TEXT NOT NULL REFERENCES site_api_keys(id) ON DELETE CASCADE,
  model_id TEXT NOT NULL,
  PRIMARY KEY (api_key_id, model_id)
);

CREATE TABLE IF NOT EXISTS site_thinking_presets (
  site_id TEXT NOT NULL REFERENCES sites(id) ON DELETE CASCADE,
  target TEXT NOT NULL,
  json TEXT NOT NULL,
  updated_at INTEGER NOT NULL,
  PRIMARY KEY (site_id, target)
);

CREATE TABLE IF NOT EXISTS target_bindings (
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
  site_api_key_id TEXT,
  site_api_key_label_snapshot TEXT,
  site_api_key_prefix_snapshot TEXT,
  FOREIGN KEY (site_id) REFERENCES sites(id) ON DELETE SET NULL,
  FOREIGN KEY (site_api_key_id) REFERENCES site_api_keys(id) ON DELETE SET NULL
);

CREATE TABLE IF NOT EXISTS apply_records (
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
  site_api_key_id TEXT,
  site_api_key_label_snapshot TEXT,
  site_api_key_prefix_snapshot TEXT,
  FOREIGN KEY (site_id) REFERENCES sites(id) ON DELETE SET NULL,
  FOREIGN KEY (site_api_key_id) REFERENCES site_api_keys(id) ON DELETE SET NULL
);

CREATE TABLE IF NOT EXISTS webdav_config (
  id INTEGER PRIMARY KEY CHECK (id = 1),
  base_url TEXT NOT NULL,
  username TEXT NOT NULL,
  password_encrypted TEXT NOT NULL,
  remote_path TEXT NOT NULL DEFAULT 'xiaobai-switch',
  accept_invalid_certs INTEGER NOT NULL DEFAULT 0,
  auto_sync_enabled INTEGER NOT NULL DEFAULT 0,
  sync_interval_minutes INTEGER NOT NULL DEFAULT 60,
  max_remote_backups INTEGER NOT NULL DEFAULT 10,
  updated_at INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS webdav_sync_state (
  id INTEGER PRIMARY KEY CHECK (id = 1),
  last_attempt_at INTEGER,
  last_success_at INTEGER,
  status TEXT NOT NULL DEFAULT 'never',
  error TEXT
);

CREATE INDEX IF NOT EXISTS idx_apply_records_target_time ON apply_records(target, applied_at DESC);
CREATE INDEX IF NOT EXISTS idx_target_bindings_orphan ON target_bindings(orphan);
CREATE INDEX IF NOT EXISTS idx_site_api_keys_site ON site_api_keys(site_id);
CREATE INDEX IF NOT EXISTS idx_site_models_key ON site_models(api_key_id);

CREATE TABLE IF NOT EXISTS target_presets (
 site_id TEXT NOT NULL REFERENCES sites(id) ON DELETE CASCADE,
 api_key_id TEXT NOT NULL REFERENCES site_api_keys(id) ON DELETE CASCADE,
 target TEXT NOT NULL,
 json TEXT NOT NULL,
 updated_at INTEGER NOT NULL,
 PRIMARY KEY(site_id, api_key_id, target)
);
