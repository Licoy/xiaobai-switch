---
title: 安装与迁移
description: 原生六平台安装包、系统要求与 0.0.12 过渡迁移
order: 2
---

## 原生 1.x 与迁移

XiaoBaiSwitch 1.x 使用 mygo 0.6.2 与纯 Go 原生 UI，旧 React / Tauri 源仅用于 0.0.12 过渡版；主窗采用统一 224px 侧栏，品牌色 #0C76F8，通用图标使用 Remix line，目标品牌使用 Lobe 图标。

从 0.0.11 升级时，先安装 [0.0.12 过渡版](https://github.com/Licoy/xiaobai-switch/releases/tag/v0.0.12)，再升级至原生 1.x，macOS 11 请停留在过渡版，原生版需要 macOS 12 或更高版本。

应用数据继续使用 `~/.xiaobai-switch/`，包含数据库与 `master.key`，应用 ZIP 备份恢复这两者，不会自动修改 Claude Code、Codex、Pi、Prime 的 CLI 配置；恢复应用数据后如需同步，请在应用中心确认后重新应用。

## 下载与安装

从 [下载页](https://xiaobaiswitch.com/download/) 或 [Releases](https://github.com/Licoy/xiaobai-switch/releases) 选择当前版本实际提供的平台与架构文件。

| 平台 | 原生安装包与运行要求 |
|---|---|
| macOS 12+，Apple Silicon / Intel | `.dmg` |
| Windows 10 / 11，x64 / ARM64 | `-setup.exe` 或便携 `.zip`，使用 Direct3D 11，无需 WebView2 |
| Linux，x64 / ARM64 | `.tar.gz`，需要 GTK 3，托盘另需 `libayatana-appindicator3`，无需 WebKitGTK |

macOS / Windows 的 `.tar.gz`、`.app.tar.gz` 与更新 JSON 是更新协议资产，不是此处的安装器，Linux 发行版兼容性以及 Windows / Linux 的签名和安装仍待发布实机验证。
