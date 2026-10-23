<p align="left">
  <a href="./README.md">中文</a> · <strong>English</strong>
</p>

<p align="center">
  <img src="assets/brand/app-icon-1024.png" alt="XiaoBaiSwitch" width="160" height="160">
</p>

# XiaoBaiSwitch

Website: [https://xiaobaiswitch.com](https://xiaobaiswitch.com)

A beginner-friendly, site-driven desktop app for wiring Claude Code, Codex, Pi, and Prime to an upstream API.

Everything starts from an upstream site: enter a Base URL and API key, fetch or type model ids, then apply to Claude Code, Codex, Pi, or Prime in one click.

## Features

- **Sites**: manage multiple upstream relays, with extra routes, speed tests, and one-click switching
- **Models**: fetch models from a site, or add and test them yourself
- **Apply Center**: write Claude Code, Codex, Pi, and Prime separately, with their own site, model, and target options
- **Route switching**: after a switch, already-applied tool URLs can be updated to match
- **Backups**: configs are backed up before apply, and can be restored in Apply Center
- **Link import**: import a site with a `xiaobaiswitch://` link; it is not applied to tools automatically
- **Desktop extras**: tray, launch at login, light / dark theme, Simplified Chinese and English

## Screenshots

These screenshots show the 0.x bridge; native 1.x uses a shared sidebar interface.

| Welcome | Model test |
|:---:|:---:|
| <img src="assets/screenshot/1.webp" alt="Welcome"> | <img src="assets/screenshot/2.webp" alt="Model test"> |
| Sites | Apply Center |
| <img src="assets/screenshot/3.webp" alt="Sites"> | <img src="assets/screenshot/4.webp" alt="Apply Center"> |

## Quick start

1. Add an upstream site with a name, Base URL, and API key
2. Fetch models, or type the model ids you need
3. Open Apply Center, pick Claude Code, Codex, Pi, or Prime, confirm the model and options, then apply
4. Restart the terminal or reopen the matching CLI so the change takes effect

A site can have multiple routes; the first one is the current default, and you can probe and switch at any time.

## Import a site from a link

After the desktop app is installed, a browser or another app can open a `xiaobaiswitch://` link to launch XiaoBaiSwitch and import an upstream site; import does not apply to Claude Code / Codex / Pi / Prime automatically, so you still confirm in Apply Center.

1. Install and open the desktop app
2. Click the import link; the app switches to Sites and shows a confirm dialog
3. Check the name, routes, protocol, notes, and API key prefix, then confirm
4. If the link has no `apikey`, confirm opens a prefilled add-site form so you can finish the key and save

The same protocol plus the same set of routes (order does not matter) counts as the same site: a matching key is reused, a different key updates the stored key; adding or removing a route creates a new site instead of merging.

### Link format

```text
xiaobaiswitch://sites?name=<name>&baseurls=<url>[&baseurls=<url>…][&apikey=<key>][&protocol=openai_compatible|anthropic][&notes=<notes>][&codex-compact=1][&codex-vision=1][&codex-imagegen=1][&codex-search=1]
```

| Parameter | Required | Description |
|-----------|----------|-------------|
| `name` | Yes | Site name, up to 128 characters |
| `baseurls` | Yes | Route Base URL; repeatable, up to 20 |
| `apikey` | No | API key; if omitted, confirm opens a prefilled form for the user to finish |
| `protocol` | No | `openai_compatible` (default) or `anthropic` |
| `notes` | No | Notes, up to 2000 characters |
| `codex-compact` | No | Codex remote compaction preset; `1` / `true` / `on` / `yes` turns it on |
| `codex-vision` | No | Codex vision preset |
| `codex-imagegen` | No | Codex image-generation preset |
| `codex-search` | No | Codex built-in search preset |

Aliases: `baseurl` = `baseurls`, `type=openai` / `type=anthropic` = `protocol`; other `platform-capability` kebab keys are stored as-is, and the current UI only shows the four Codex ones; if the link includes any capability parameter, it is treated as a full Codex preset (omitted known keys are off); older links without these parameters will not overwrite presets already on the site.

### Multiple routes

The first item is the current / default route; prefer repeating `baseurls` so a URL that contains a comma is not split by mistake:

```text
xiaobaiswitch://sites?name=Example%20Relay&baseurls=https://a.example.com/v1&baseurls=https://b.example.com/v1&apikey=sk-xxx&protocol=openai_compatible
```

A single parameter also works, separated by commas or `|`:

```text
xiaobaiswitch://sites?name=Example&baseurls=https://a.example.com/v1,https://b.example.com/v1
xiaobaiswitch://sites?name=Example&baseurls=https://a.example.com/v1|https://b.example.com/v1
```

`baseurl` and `baseurls` can be mixed, and are merged in query-string order:

```text
xiaobaiswitch://sites?name=Mix&baseurl=https://first.example.com/v1&baseurls=https://second.example.com/v1
```

Putting an API key in a URL can leave it in browser history, extensions, or system logs, so do not put a real key on a public page; generate the link from a private, signed-in console, or omit `apikey` and let the user finish it in the app.

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

## Native development

Run `./scripts/native-dev.sh` for a development instance. Everyday launches, debugging and manual acceptance use `~/.xiaobai-switch` and the four CLI configuration directories under your real HOME by default, honoring existing settings and target environment overrides. Development binaries and `.app` bundles use the same defaults; specify `--dev-home` or `XIAOBAI_DEV_HOME` only when isolation is explicitly needed.

```sh
./scripts/native-go.sh test ./internal/...
./scripts/native-go.sh vet ./internal/...
./scripts/native-go.sh build -o .build/XiaoBaiSwitch .
```

See [native development](docs/v1-development.md) for toolchain setup, packaging, default directories and optional isolation. Legacy `pnpm dev` / Tauri commands apply to the bridge only.


## Automatic updates

Official desktop builds can check for updates and install them in-app; check manually in **Settings → About**, or turn on automatic checks.

## Security

API keys are encrypted inside the app; after apply they may be written in plaintext to Claude Code config, Codex environment files, `~/.pi/agent/auth.json`, or `~/.prime/agent/auth.json`, so do not sync those configs to untrusted cloud storage.

Pi and Prime use their official `models.json`, `auth.json`, and `settings.json` interfaces. XiaoBaiSwitch manages only `xiaobai_` providers and preserves other custom providers, OAuth logins, and unknown settings. You can write only the default model or the whole site catalog. Prime writes `~/.prime/agent` by default, not `~/.pi/agent`.

## License

MIT
