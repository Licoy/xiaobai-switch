---
title: 设置与更新
description: 语言、主题、托盘、代理、路径、备份策略、Codex 注入方式和自动更新。
order: 12
---

设置页从标题栏进入，Esc 返回。多数开关会立即保存。

## 通用

- **语言**：简体中文 / English
- **主题**：跟随系统、浅色、深色
- **窗口置顶**
- **开机启动**
- **关闭窗口时最小化到托盘**
- **开机启动行为**：旧 startInTray 只映射开机启动，主动打开应用始终显示主窗

## 网络

- 代理：系统代理 / 不使用 / 自定义。系统代理不解析 PAC；若仅配置了 PAC，会回退环境变量 `HTTP_PROXY` / `HTTPS_PROXY`
- 线路测速结果有效期，默认 10 分钟

## 路径

可覆盖 Claude、Codex、Pi Agent 与 Prime Agent 配置目录。Pi 留空时先读取 `PI_CODING_AGENT_DIR`，再使用 `~/.pi/agent`。Prime 留空时先读取 `PRIME_AGENT_CODING_AGENT_DIR`，再使用 `~/.prime/agent`。

## 应用行为

- **Codex 密钥注入方式**：自动（按平台）、仅 Shell rc、用户环境变量、仅写入 `codex.env`
- **Claude 强制独占鉴权键**：写入选定键时删除另一鉴权字段

## 备份

见 [备份与还原](../backups/)。

## 关于与更新

显示版本。可手动检查更新，或开启自动检查并设置间隔。正式构建从 GitHub Releases 的 `update-{darwin,windows,linux}-{amd64,arm64}.json` 拉取更新清单。
