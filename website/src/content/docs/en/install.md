---
title: Install and migrate
description: Native packages, runtime requirements and the 0.0.12 bridge
order: 2
---

## Native 1.x and migration

XiaoBaiSwitch 1.x uses mygo 0.6.2 and a native Go UI. React / Tauri sources remain for the 0.0.12 bridge only. The native window uses a shared 224px sidebar, #0C76F8, Remix line UI icons and Lobe provider icons.

Upgrade 0.0.11 to [the 0.0.12 bridge](https://github.com/Licoy/xiaobai-switch/releases/tag/v0.0.12) before native 1.x. macOS 11 should stay on the bridge; native builds require macOS 12 or later.

Data stays in `~/.xiaobai-switch/`, including the database and `master.key`. App ZIP backups restore these files without automatically changing Claude Code, Codex, Pi or Prime CLI configurations. Confirm and apply again in Apply Center when you want to synchronize them.

## Download and install

Choose an available platform and architecture from [Downloads](https://xiaobaiswitch.com/en/download/) or [Releases](https://github.com/Licoy/xiaobai-switch/releases).

| Platform | Native package and runtime |
|---|---|
| macOS 12+, Apple Silicon / Intel | `.dmg` |
| Windows 10 / 11, x64 / ARM64 | `-setup.exe` or portable `.zip`, Direct3D 11, no WebView2 |
| Linux, x64 / ARM64 | `.tar.gz`, GTK 3; tray support also needs `libayatana-appindicator3`, no WebKitGTK |

macOS / Windows `.tar.gz`, `.app.tar.gz` and update JSON files belong to update protocols rather than the installers above. Linux distribution compatibility and Windows / Linux signing and installation await release testing on devices.
