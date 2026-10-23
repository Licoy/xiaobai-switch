# v1 实际开发状态

更新日期：2026-10-23，业务源码已冻结，本记录区分真实窗口操作、单元测试、跨编译和正式发布；发布批详情见 `v1-release.md`，完整需求见 `v1-development.md`

## 当前交付

- Go 1.27.1 + mygo runtime/CLI v0.6.2，纯 Go mygo/ui，无 WebView；主窗 1100×720、最小 960×640、224 宽平面导航
- 中英文、系统/浅/深主题、自定义品牌色和原生标题栏同步；通用图标 RemixIcon 4.9.0 line，ClaudeCode/Codex/Pi 使用 Lobe 5.15.0，Prime 使用 Remix 通用图标，两份官方许可嵌入包并可在关于页查看
- 真实 Store/Service 接线：站点 CRUD、搜索、排序、启停、多 key、按需隐藏解密、多线路、协议/备注和 Codex 四能力；删除支持保留 CLI 的孤立绑定或逐目标撤销后删除，线路支持仅修改站点或同步目标
- 模型缓存、手动添加、默认模型、删除/清空、真实获取/探测与可取消批量探测；URL 写入预览、线路延迟探测时间与 TTL 过期标识
- 余额按站点/key/线路/密钥版本隔离，缺值显示未知、无限额明确显示，保留最近成功与失败信息、更新时间和过期日期
- 四目标当前配置与草稿区分，独立模型与私有能力表单；Claude auth/别名/effort/[1m]，Codex catalog/reasoning/能力来源，Pi/Prime 独立 thinking 和全局/逐模型映射；预览取消不写站点默认或目标配置
- 真实预览/应用/撤销/官方恢复/孤立清理/目录打开与目标备份；部分失败展示逐目标结果，提交后清理失败刷新成功状态并显示维护警告，错误提供折叠的脱敏技术详情
- 技能本地列表/筛选/详情/启停/删除/目录、skills.sh/GitHub 搜索安装和基于来源清单的更新；应用本地备份与 WebDAV 配置/连接测试/上传/列表/下载恢复/删除/取消/调度摘要均接真实业务
- 恢复仅替换应用 DB/主密钥，不自动恢复 CLI；排队后停调度、暂停写入并显示重启入口，Store.Open 前应用待恢复；后台通知只刷新本地摘要，不自动重复网络列表
- 界面改为 CPA-Bar 式多彩布局：隐藏标题栏、红绿灯悬于侧栏，侧栏为实色图标块+计数/状态点，内容为无边框柔和面板、着色标题、统计卡与胶囊按钮；新增总览首页（统计、四目标卡、站点与备份摘要），站点中心改为左侧可筛选列表+右侧详情（概要、余额/线路、密钥、模型、应用到目标），设置/目标/备份/技能/关于页统一为设置行与面板
- 站点图标沿用 Tauri 版逻辑：取当前 Base URL 的 origin，首页 HTML 的 icon/apple-touch-icon 按同样优先级逐个尝试，再回退 `/favicon.ico`，走应用代理设置、8 秒超时、512KB 截断；按站点 ID+origin 缓存于 `~/.xiaobai-switch/cache/site-icons/`，编辑站点即失效、删除站点清除；原生补 ICO（PNG/1-32 位位图）与 SVG 解码，无法解码的候选继续尝试下一个，全部请求无响应时不写入「无图标」缓存
- 全部结果、警告、错误与启动提示统一由弹窗呈现（托盘可见时在托盘内弹出），即时保存的设置与常规增删改静默成功，持续状态（差异、密钥明文提示、最近获取失败）保留为行内横幅；`XBS_SHOTS=<目录>` 可离屏渲染全部页面、托盘与弹窗的浅/深色截图
- 420 宽托盘视图：四目标切换卡+单目标快速表单、固定拖动、未固定失焦隐藏、位置延迟持久化和工作区 85% 限高；Linux AppIndicator 使用菜单入口；这些快速面板交互尚无系统级完整验收
- 手动启动显示主窗、登录隐藏与关闭留托盘遵循设置，无托盘显示错误并允许正常退出；退出取消任务并等 manager/Service/DB/锁收口
- mygo 单实例加应用根文件锁，macOS 旧 Tauri socket 激活/深链转发与同用户 stale socket 回收，Windows 旧隐藏窗口先于 mutex 建立；Linux 无已发布旧 Tauri 包，仅启用原生单实例
- 深链确认只显示 key 前缀，无 key 时打开已填表单；生产集成登录项与协议注册，开发禁用；About 接真实更新管理器，开发强制禁用更新
- 日常启动、调试与手动验收默认使用正式 `~/.xiaobai-switch` 与四目标 CLI 目录，沿用已有设置和目标环境变量覆盖；脚本、开发二进制与开发 `.app` 均不自动隔离，重启保持当前目录选择，正式构建使用系统 Relaunch
- 仅用户明确要求时使用 `--dev-home` / `XIAOBAI_DEV_HOME`，仍验证隔离边界；`--use-real-home` 兼容旧调用但默认无需指定，仍与非空隔离参数互斥；自动化测试使用临时测试数据，不写真实配置
- native-dev.sh 使用稳定 `.build/native-dev-XiaoBaiSwitch`，避免 go run 清除临时 exe 破坏二次重启；smoke/bindings 在配置与 DB 初始化前返回，.build 被忽略

## 主进程独立验收（历史记录）

以下验收记录对应当时默认隔离且缺少目录选择时拒绝启动的开发版本，该默认已由上面的正式目录默认规则替代；保留历史结果，不作为本次目录默认变更的验证证据

| 检查 | 实际结果 |
| --- | --- |
| 前端 | pnpm test:run：45 files、333 tests，17.10s；pnpm typecheck exit0 |
| Rust | cargo test --manifest-path src-tauri/Cargo.toml --offline：293 passed，1.14s；初次沙箱 4 个 HTTP 监听 PermissionDenied，审批允许后全通过，存在既有 dead-code warnings |
| Go | test -race -count=1 ./...、vet ./... 全部 exit0；app4.032s、service6.962s、adapters8.258s、update1.229s |
| 发布工具/网站 | Node 6 tests 通过，57.4ms；网站 fixture 构建 37 pages |
| 六平台编译 | Go1.27.1，darwin/windows/linux × amd64/arm64 全部 built，exit0；不等同于六平台运行 |
| 开发入口 | invalid devhome 的 smoke 与 MYGO_GENERATE 均 exit0；无隔离 home 的开发正常启动 exit1，明确 refusing production home |
| 真实 Claude | 预览→确认→隔离 settings.json，权限0600、模型/baseURL正确、UI key redacted；再次预览取消→撤销通过，保留空配置并清 managed fields |
| 真实 Codex/Pi/Prime | 各自 Preview→Confirm→Applied，隔离文件正确；Pi/Prime 不同目录且 auth0600，Codex env0600、responses、隔离 zshrc |
| 真实主题/语言 | CLI 检测结束，中英即时切换、自定义品牌色、dark 内容与 dark 原生标题栏通过，About 可见更新禁用说明 |
| 真实备份 | 本地 ZIP 含3项、DB118784字节/key32字节；恢复→重启后保持 Isolated development 与 Native UI fixture，显示 Application data restored，恢复语言/主题 |
| 真实技能 | fixture 详情可读；禁用生成 SKILL.md.disabled，重新启用恢复 SKILL.md |

宿主补充测试覆盖 ui.NewTester 导航/翻译/浅深、Store CRUD、密钥生命周期、设置平台动作失败回滚、预览取消零写入、thinking映射、私有 auth/live model 优先、CLI完成通知合并、余额 nil/unlimited/密钥版本/过期、深链无key/缺省协议、恢复写入锁定、显式隔离重启与屏幕约束；Darwin 临时 socket 夹具验证 stale 恢复、活跃 listener 不删除与深链转发，需允许本地 socket 绑定

## 最终开发包（历史记录）

- `.build/native/darwin-arm64/XiaoBaiSwitch.app`，17.8 MB，mygo build 与 ad hoc 签名 exit0
- codesign --verify --deep --strict 输出 valid on disk、satisfies its Designated Requirement；CFBundleIdentifier 为 com.github.licoy.xiaobai-switch.app
- 最终包使用 invalid-relative-home 执行 --native-smoke 返回 `{"version":"1.0.0-dev","os":"darwin","arch":"arm64","native":true}`、exit0，未再次启动窗口
- 独立验收完成后隔离应用已正常退出，限定项目可执行路径的进程检查无匹配；未发布、未提交推送
- `mod verify` 输出 all modules verified，负责范围 diff --check 与三个宿主脚本 sh -n 通过；native-dev.sh --native-smoke 构建稳定 exe 并 exit0
- 工具链官方 SHA256：ee215d57e0ec269c60cc9ceca68e6bda321ba9ee5afe24f4b0988703c2d87d12；官方对象存储曾有 EOF/TLS，官方 tar 校验安装，依赖镜像仍保留 Go checksum 验证
- 历史截图 `.build/native-shell-macos.png` 与 `.build/native-stage2-claude.png` 均被 Git 忽略，不能替代最终交互证据

## 开发重启隔离事故（历史记录）

以下记录保留当时显式隔离丢失的事故与修复证据，日常开发现已默认使用正式目录，显式隔离仍须在重启后保持隔离边界

- 旧第二批包在 macOS 恢复→Restart now 时使用系统 Relaunch 丢失 XIAOBAI_DEV_HOME，新窗口显示真实站点；主进程立即 CmdQ，没有点击真实站点或目标操作，不能宣称本轮未触碰真实用户数据
- 真实 DB 从 schema1 升至 schema2，仅新增 target_presets；迁移前快照：`/Users/lmini/.xiaobai-switch/backups/app/pre_migration_v1_5f966fbe-78b9-4544-a876-27d1cce91f0c`
- 旧表 settings、site_api_keys、site_models、sites、target_bindings、apply_records、site_model_exclusions、site_thinking_presets、webdav_config 逐行比较均不变
- webdav_sync_state 的 last_attempt_at/last_success_at 从1791556007585变为1792724124617，status=success，说明生产自动上传和 Cleanup 返回nil；是否删除远端旧备份未知，不自动回滚 DB 或操作远端
- 旧 dev Info.plist 没有 CFBundleURLTypes，上游 RegisterURLScheme 缺声明时返回错误，没有改默认 URL handler 的证据；登录项影响仍未知
- 修复后真实恢复重启保持开发标记/fixture；验收前后生产 DB SHA256 均为8c3ae9e41531b5d382ec5aefe4d1abc9d98be6ef9c5013b4fa6c3d24ff6f3bc3，mtime_ns 均为1792724178545915439，证明这次修复验收未再改生产 DB；不能抵消之前事故

## 未验收边界

- 新界面仅经离屏渲染截图与单测核对；隐藏标题栏下红绿灯位置、Windows/Linux 窗口按钮避让未在真实窗口截图验收（当前环境无屏幕录制权限，开发实例可正常启动）
- 快速面板只有视图与几何单测；macOS AX 不暴露状态栏图标、SystemUIServer 获取超时，未实际验证托盘内应用、固定、失焦、跨显示器
- Windows/Linux 仅跨编译，未真机运行；Windows 旧版 mutex/window 协议与 Linux AppIndicator 需要对应平台验收
- 公网 marketplace、供应商余额、实际 WebDAV 连通性和正式签名更新未以用户凭据做功能验收；事故中的真实自动上传不作为受控测试通过
- 正式签名、公证、安装包真实安装/升级、Tauri过渡版本→v1的生产发布尚未完成；仅开发 ad hoc 包，不能称全发布完成
- 少数生命周期与 UI 组合函数超过默认50行，保留流程以核对共享窗口状态、任务收口与完整目标表单，未为缩行引入抽象
