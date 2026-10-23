---
title: FAQ
description: macOS will not open, config does not take effect, key display, and import behavior.
order: 14
---

## macOS says the app is damaged

See [Install](../install/). Run `xattr -cr /Applications/XiaoBaiSwitch.app`, then right-click Open.

## Apply succeeded, but the CLI still uses the old URL

Restart the terminal, or fully quit and reopen Claude Code / Codex / Pi / Prime. Apply only writes config files; it does not hot-reload a running process. In Pi, open `/model` to reload the model list.

## How can I view the full API key?

Site lists and details show only a prefix. Open the site edit dialog, wait for the complete key to load, then use the eye control on the password field. The key remains encrypted in the app database.

## Does an import link change Claude / Codex immediately?

No. Import only creates or updates a site. You still apply in Apply Center.

## Is Linux supported?

Native 1.x includes Linux x64 / ARM64 `.tar.gz` packages, requiring GTK 3 and libayatana-appindicator3 for tray support. Check the actual assets on Downloads; distribution compatibility awaits device testing.

## What is the official site URL?

[https://xiaobaiswitch.com](https://xiaobaiswitch.com) (GitHub Pages custom domain). Docs live at `/docs/`; English at `/en/`.
