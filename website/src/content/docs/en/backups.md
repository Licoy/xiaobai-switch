---
title: Backups
description: Target configs are backed up before apply and can be previewed, restored, or deleted in Apply Center.
order: 11
---

Before writing Claude Code, Codex, Pi, or Prime, the app copies the current target files into `~/.xiaobai-switch/backups/`. Each target has a separate directory; Pi and Prime back up `models.json`, `auth.json`, and `settings.json` together.

## In Apply Center

Each target form lists backup records: preview a summary, reveal the source file, restore, or delete.

Restore **overwrites the current config file** and cannot be undone. Deletion is permanent.

## Retention

**Settings → Backup**: max copies per target, default 30. Older copies for that target are removed. You can open the backup directory from Settings.

## App data backups and WebDAV

Native 1.x app ZIP backups live in `~/.xiaobai-switch/app-backups/`, with a consistent database snapshot, `master.key` and a verification manifest. They contain the material needed to decrypt stored keys, so keep them private.

Restore first creates a pre-restore snapshot, then queues replacement for the next launch before the database opens. Restoring app data does not change the four target CLI configurations; confirm and apply again in Apply Center if needed.

WebDAV supports connection testing, upload, download, deletion and scheduled synchronization. Automatic retention only cleans backups from this device. Passwords are encrypted in the application database; cleanup warnings after successful uploads are shown separately.
