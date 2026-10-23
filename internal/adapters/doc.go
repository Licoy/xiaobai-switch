// Package adapters builds and executes native CLI configuration changes.
//
// Build, Revert, Rewrite and RestoreOfficial are read-only constructors; Plan JSON
// omits credentials and file content. Execute acquires legacy-compatible target/site
// locks, checks preview snapshots, and delegates backups, durable recovery journals,
// atomic replacement, rollback, and read-after-write verification to fileops.
// The service must persist Plan.Binding only after Execute succeeds. All target paths
// are explicit: callers resolve overrides/environment/home with package paths.
//
// Pi/Prime models.json accepts line comments and trailing commas; auth/settings and
// Claude settings require strict JSON, matching the CLIs and previous Rust adapter.
// Codex EnvMode defaults to file_only; shell_rc requires ShellHome, while user_env
// requires Windows. Registry recovery is envinject.Recover(backupDir), which also
// recovers its file transaction; call it under the corresponding target/site locks.
// Backup directory parents must already exist; every transaction directory is new.
package adapters
