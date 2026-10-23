# XiaoBaiSwitch v1 原生重构开发要求

本清单记录已批准的六阶段范围，勾选与验收以 `v1-progress.md` 为准，阶段标题不代表已经实现

## 基线与技术方案

- 工作分支为 `v1`，旧实现基线 main `5a6e4bc` / v0.0.11，保留旧 Tauri / React / Rust 和 website，未经要求不删除旧实现
- 新实现使用 Go 1.27.1、github.com/egoist/mygo v0.6.2 与同版 CLI，mygo/ui 从 Go 直接绘制，不采用 WebView
- 产品名 XiaoBaiSwitch，bundle id `com.github.licoy.xiaobai-switch.app`，品牌色 `#0C76F8`，复用 `assets/brand/app-icon-artwork.svg`
- Quosparr 参考版本为 `823bf89ca06bbcb79eb54f3529edf9fea7b70f02`，独立实现，不复制 AGPL 业务代码
- 站点优先 SSOT：Base URL + API key → 模型 → 目标私有能力预设 → 应用到目标
- 应用根目录保持 `~/.xiaobai-switch`，SQLite 数据库与 AES-256-GCM 主密钥，Unix 主密钥权限 0600
- 通用 UI 图标统一使用官方 RemixIcon，优先 `-line` 空心版；品牌/服务商图标使用官方 Lobe Icons，保留版本、来源与许可证；Prime 当前 Lobe 无专用图标，明确使用 Remix 通用图标，不自造品牌；不用 Lucide 或字符代替图标
- 所有用户可见文案提供 zh-CN / en-US，系统字体，浅深色跟随系统，不硬编码状态色

## 阶段一：骨架、工具链与隔离

- 固定并真实验证 Go / mygo / CLI 版本，依赖校验不得关闭，不静默降级
- 主窗口 1100×720、最小 960×640，224 宽统一图文侧栏，系统窗口拖拽，12 圆角分组卡片
- 导航：站点中心、技能管理；应用到分组 Claude Code / Codex / Pi / Prime；通用分组外观 / 网络 / 路径环境 / 备份 / 关于更新
- 未接服务的页面明确说明尚未接入，禁止伪造成功，禁用写入
- 日常启动、调试与手动验收默认使用真实 HOME 下的正式应用与四目标 CLI 目录，沿用已有设置和目标环境变量覆盖；脚本、开发二进制与开发 `.app` 均不自动创建隔离目录
- 仅用户明确要求隔离时使用 `--dev-home` / `XIAOBAI_DEV_HOME`，统一验证隔离路径边界，不持久修改系统 HOME；自动化测试使用临时 HOME / 测试数据目录，不写真实配置
- 开发模式禁用真实登录项、正式更新与 URL 注册，WebDAV 沿用已有设置；开发重启须在资源完整收口后显式保留当前目录选择与环境，不能依赖 macOS LaunchServices 继承环境
- 主动打开显示主窗口，开机可仅托盘，关闭默认留托盘，托盘不可用则显示错误并允许关闭退出
- 窗口路由、翻译目录、隔离逻辑测试，真实原生窗口启动与布局验收

## 阶段二：存储与安全

- SQLite 结构、迁移、事务、站点/模型/预设/设置存储，兼容旧数据，迁移前可靠备份
- 密钥加密、按需解密、全链路脱敏，不在日志或长期 UI 状态保留明文
- 应用目录、目标目录和开发路径策略统一，显式隔离时路径覆盖不得逃离开发 home
- 原子写入、备份、权限、锁、失败回滚，不能吞掉错误
- 备份列表、恢复、保留策略和导入导出，确保用户数据完整

## 阶段三：四目标适配

- Claude Code：鉴权 key 风格、默认模型、opus/sonnet/haiku 别名、effort
- Codex：responses wire API、由站点 id 派生 provider、默认模型、全部模型目录、reasoning effort、compact / vision / imagegen / search 站点默认与覆盖
- Pi：合并 models/auth/settings JSON，保留注释、未知字段、其他 provider 与 OAuth，只管理 `xiaobai_` 命名空间；逐模型思考能力与默认等级按 `(site_id,target)` 记忆
- Prime：与 Pi 同等级能力，默认 `~/.prime/agent`，环境变量 `PRIME_AGENT_CODING_AGENT_DIR`，严禁写到 Pi 路径
- 配置路径按设置 → 对应环境变量 → 默认目录解析，环境注入 shell rc / user env / file_only 与设置一致
- 应用前备份，跨文件失败回滚，auth.json 0600，保留不归属本应用的数据
- 提示应用后 API key 可能明文出现在 CLI 文件、codex.env 和 shell rc

## 阶段四：全功能 UI

- 接真实存储和应用服务，站点增删改查、搜索筛选、排序、启停、模型发现、连通性/协议配置、能力预设、导入导出
- 模型探测、配额查询与网络代理使用真实服务和明确错误，不以空结果伪装成功
- 站点高级配置与 Codex 私有能力默认收起，deep link 使用同一套 kebab 键
- 四目标各自表单、模型选择、私有能力配置、应用预览与结果反馈
- 技能管理：接实际技能目录和导入来源，展示与管理目标安装状态
- 外观、网络、路径环境、备份、关于更新各页接真实设置，适合即时保存的字段即时保存
- 补充键盘操作、可访问性、错误/空/加载状态，中英文和浅深色验收

## 阶段五：托盘与生命周期

- 420 宽托盘原生面板，四 Agent 卡片，选择站点/模型后点击应用，可固定
- macOS / Windows 点击托盘显示面板；Linux AppIndicator 仅菜单入口已接受
- 主动启动显示主窗口，登录启动可仅托盘，关闭留托盘，显式退出释放资源
- 单实例、激活、开机启动与状态持久化，开发环境禁止真实注册登录项
- 托盘验收需覆盖实际草稿/当前配置、失焦收起、固定移动与屏幕变化，不能只以基础菜单代替完整面板

## 阶段六：三端发布与升级

- macOS / Windows / Linux 构建、图标、签名、公证/安装包和发布验收
- 旧 Tauri v0.0.12 作为过渡版本，引导并验证升级至 v1.0.0，保留既有自动更新签名协议
- 新旧签名格式和升级包兼容性需要明确实现与测试，mygo 自身更新能力不能自动替代旧 Tauri 协议
- 正式 URL 注册、更新源、日志、迁移恢复、降级/失败恢复、发布文档和支持边界
- 未经明确授权不发布、不提交推送；日常开发验收默认使用正式目录，仅用户明确要求时隔离

## 开发命令

```sh
./scripts/native-toolchain.sh # 初次安装，下载固定版本并校验官方 SHA256
./scripts/native-go.sh version
./scripts/native-go.sh test ./internal/...
./scripts/native-go.sh vet ./internal/...
./scripts/native-go.sh build -o .build/XiaoBaiSwitch .
./scripts/native-go.sh tool mygo version
./scripts/native-go.sh tool mygo build -skip-dmg -skip-build-command
./scripts/native-dev.sh
```

`native-go.sh` 强制 Go1.27.1，优先 `.build/toolchain/go`，显式设置 GOROOT、GOMODCACHE、GOCACHE，避免继承本机 Go1.25 的 GOROOT；依赖下载受官方代理对象存储 TLS 阻断时可设置 `GOPROXY=https://goproxy.cn`，仍保留 sumdb 验证

`native-dev.sh` 构建到 `.build/native-dev-XiaoBaiSwitch` 的稳定可执行路径再启动，避免 go run 临时可执行文件被清除导致二次重启失败；默认使用真实 HOME 下的 `~/.xiaobai-switch` 和正常四目标 CLI 路径，保留现有设置与目标环境变量覆盖规则，不自动创建临时目录；`.build/` 已忽略，包含缓存、工具链、打包产物和测试 home

## 特殊需求：显式隔离

日常启动、调试与手动验收直接使用上面的默认命令；仅用户明确要求隔离时，指定独立绝对路径，脚本不会自动创建临时隔离目录

```sh
./scripts/native-dev.sh --dev-home /absolute/path/to/isolated-home
# 或 XIAOBAI_DEV_HOME=/absolute/path/to/isolated-home ./scripts/native-dev.sh
```

显式隔离仍验证应用与目标路径边界；开发二进制和开发 `.app` 未指定隔离参数或环境时默认使用正式目录，重启保持当前目录选择

`--use-real-home` 仅为兼容旧调用保留，默认启动已无需该参数，仍与 `--dev-home` / 非空 `XIAOBAI_DEV_HOME` 互斥；无论目录选择如何，开发构建仍禁用登录项、URL 注册与正式更新，WebDAV 沿用已有设置
