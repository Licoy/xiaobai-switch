# Native 1.x 发布与旧版过渡

发布链固定为 Go 1.27.1、mygo 0.6.2、macOS/Windows/Linux 的 amd64 和 arm64；开发配置不含正式更新公钥，开发模式与 `XIAOBAI_DEV_HOME` 均禁止真实更新

## 两套更新协议

- `latest.json` 永远指向不可变的 `v0.0.12` 四平台 Tauri 安装包，原 0.0.11 先升级为桥接版本
- bridge 单独发布时同时上传与完整 `latest.json` 同内容的 `native-transition.json`，版本仍为 0.0.12，支持系统检查会得到当前版本且无更新，不会因 1.x 尚未发布而请求 404；两份清单必须都上传成功才公开 bridge
- 0.0.12 在 macOS 12+、Windows 10+ 使用 `native-transition.json`，macOS 11 留在 0.0.12；桥接仍由已有 Tauri plugin 验证原 minisign 公钥，macOS 使用 `.app.tar.gz`，Windows 使用兼容 `/UPDATE /ARGS /P /S /R` 的 NSIS 安装器
- 原生应用使用六份 `update-{darwin,windows,linux}-{amd64,arm64}.json`，签名为原始 Ed25519 私钥对压缩包 SHA-256 摘要签名，base64 公钥 32 字节、私钥 64 字节，与 Tauri minisign 格式不同
- 每个未来原生版本都复制并验证 `v0.0.12/latest.json`，不会让仍使用旧更新源的客户端断更；桥接资产必须先正式发布，不能用空清单代替

## 产物

`XiaoBaiSwitch_{version}_{os}-{arch}.tar.gz` 是原生更新包，macOS 另有 `.dmg` 与 `.app.tar.gz`，Windows 另有 `.zip` 与 `-setup.exe`，Linux 附 mygo 生成的 `install.sh`，可执行 `sh install.sh ARCHIVE` 安装到当前用户目录；四个桥接包附 `.sig`，六个原生更新清单分别指向该版本的不可变 GitHub release 资产

`Native Release` 仅处理 `v[1-9]*.*.*`，旧 `Release` 仅处理 `v0.*.*`；RELEASE_VERSION 必须为无 v 前缀且 major ≥ 1 的 semver，临时 mygo 配置位于 `.build/native-release/{target}`，不改动仓库 mygo.json，也不把原生版本写进桥接 package/Cargo/Tauri 的 0.0.12

MyGo CLI 自行传入 `-ldflags`，因此发布使用 `scripts/native-release-go` 保留其全部编译参数并追加 `internal/buildinfo.Version`，以保证原生 JSON、可执行文件和 archive/feed 版本一致；打包始终 `-skip-build-command`，绑定生成不会初始化 Store 或 UI

## 必须人工配置的 CI 参数

仓库变量：`MYGO_UPDATER_PUBLIC_KEY`、`APPLE_SIGNING_IDENTITY`

仓库 secrets：`MYGO_UPDATER_PRIVATE_KEY`、现有 `TAURI_SIGNING_PRIVATE_KEY`、`TAURI_SIGNING_PRIVATE_KEY_PASSWORD`、`APPLE_CERTIFICATE_BASE64`、`APPLE_CERTIFICATE_PASSWORD`、`WINDOWS_CERTIFICATE_BASE64`、`WINDOWS_SIGN_CERT_PASSWORD`

原生 Ed25519 key 已在本地生成并通过签名、验签和篡改拒绝检查，公钥文件为 `~/.config/xiaobai-switch-signing/mygo-update.pub`，私钥文件为 `~/.config/xiaobai-switch-signing/mygo-update.key`，目录权限 `0700`、私钥 `0600`、公钥 `0644`，私钥是 raw 64 字节的 base64 文本，无独立密码；公钥为 raw 32 字节的 base64 文本，不能放入旧 Tauri minisign 配置

既有 `.updater-private.key` 与 `.updater-signing.env` 已实际签名并匹配 Tauri 内置公钥；没有上传或读取 GitHub secrets 值，CI 仍要求人工配置上述变量与 secrets，缺失、不匹配、缺 NSIS 或签名失败均阻止发布，MyGo 的“跳过安装器”不能算成功

当前发布采用两个平台自签，不需要 `APPLE_ID`、`APPLE_PASSWORD`、`APPLE_TEAM_ID` 或 notary profile；macOS 的 `AQBot Release` 自签证书已定位，subject/issuer 为 `CN=AQBot Release,O=AQBot`，本轮尚未导出或配置 Actions，Windows 证书尚未定位

Apple 身份导入 runner 临时 keychain，仅在临时 CI runner 添加此证书的管理员域 codeSign 信任，实际 codesign 验签要求叶证书 SHA1 与导入证书相同；MyGo 生成的自签 DMG 验签后重新编码为 unsigned DMG，校验容器完整性且确认无签名，内部 app 保留原签名，不调用公证或 stapler；处理方式参考 [CPA-Bar 发布工作流](https://github.com/Licoy/CPA-Bar/blob/main/.github/workflows/release.yml)，未复制业务代码

Windows 从 PFX 导出公共证书，仅导入 runner `CurrentUser\Root` 与 `CurrentUser\TrustedPublisher`，不把私钥导入用户证书库；对 payload、uninstaller、setup 均做 Authenticode，要求 `signtool verify /pa` 和签名证书 thumbprint 校验通过，再生成 Ed25519 与 minisign 更新签名，CI 最后删除信任记录与证书文件

自签不等于 Apple notarization 或 Windows SmartScreen reputation，首次启动仍可能要求系统确认；unsigned DMG 只移除容器额外的 Gatekeeper 拒绝，不承诺 app 自动受信任，依据 [Apple TN2206](https://developer.apple.com/library/archive/technotes/tn2206/_index.html) 与 [Microsoft Import-Certificate](https://learn.microsoft.com/en-us/powershell/module/pki/import-certificate)

## Windows 升级边界

NSIS 使用原 currentUser 路径 `$LOCALAPPDATA\XiaoBaiSwitch`、原产品名 uninstall key、`Software\github\XiaoBaiSwitch` manufacturer key、bundle id 与 `xiaobaiswitch` 协议，已有 NSIS 安装路径从注册表恢复；替换前暂存 payload，旧目录重命名为 `.previous`，复制失败恢复原目录，回滚失败明确保留恢复路径

旧 MSI 在 HKCU/HKLM 和 32/64 位卸载注册表中按产品名 `XiaoBaiSwitch`、publisher `github`、WindowsInstaller 标记识别，迁移先选可写用户目录，完整安装并运行 `--native-smoke` 验证版本后才调用 msiexec 卸旧 MSI，避免先卸载后因 Program Files 权限失败丢失可运行程序；旧 MSI 卸载失败保留新旧程序并显式失败，注册失败保留 `.previous`，不会伪报成功，所有安装失败回调设置非零退出码

`/ARGS` 原样传递给新程序，只有 `/R` 或交互完成页的启动选择才重启；installer 等待旧进程释放文件最多 30 秒，用户数据根目录和目标 CLI 配置不属于 installer 卸载范围

## 验证与未验证项

`node scripts/native-check.mjs` 实际交叉编译六目标，`node --test scripts/native-release-lib.test.mjs scripts/native-release-sign.test.mjs scripts/native-transition.test.mjs` 验证格式、签名、缺产物、路径与过渡协议，`go test -race ./internal/update ./internal/service` 验证调度、取消、状态与开发隔离

`Native Check` 在三种宿主系统执行测试、六目标构建和无状态 smoke，Windows 另执行 NSIS 的 `/UPDATE /ARGS` 与复制失败恢复 fixture；`Native Release` 对宿主架构解包运行 smoke，Linux 在临时 HOME 中运行真实 install.sh 后验证可执行文件，Windows ARM64 当前由 Windows x64 交叉构建，明确报告无法执行 ARM64 runtime smoke

当前本机已验证 macOS ARM64 `--native-smoke` 和六目标交叉编译，Intel smoke 因本机无 Rosetta 返回 bad CPU type，未执行成功，本轮未在 Actions 配置或验证平台自签证书，未实机执行 Windows NSIS/MSI 迁移、Windows ARM64、Linux 桌面与完整 UI 升级；这些必须在具备凭证和对应系统的 CI/人工验收中完成，解包 smoke 仅证明可执行文件及版本，不等于 UI、安装器或实际应用更新验收

更新服务通过单次安装的稳定 HTTP transport 读取当前代理设置，HTTPS 禁止降级，检查 15 秒、单请求 30 秒，下载可取消，关闭等待在途任务；OnChange 必须快速把状态排到主线程，不能同步调用 Close，队列中的 UI 回调必须检查窗口关闭状态，Close 后不再访问 Store

## 正式发布顺序与回退

1. 用户配置旧 Tauri minisign 私钥及密码，保留 v0.0.11 内置公钥；新原生 Ed25519 公钥放 `MYGO_UPDATER_PUBLIC_KEY` 仓库 variable，私钥放 `MYGO_UPDATER_PRIVATE_KEY` secret，不能交换两种格式或重新生成旧密钥
2. 导出并配置现有 macOS 自签证书 P12 和 Windows 自签代码签名证书 PFX，证书与密码只放 Actions secrets，Apple identity 使用前述 variable；新原生 key 已生成，旧 Tauri key 保留，本轮未上传 secrets 或触发发布
3. 先发布 `v0.0.12` bridge，验证四平台安装器、minisign `.sig`、`latest.json` 与同版本 `native-transition.json` 已公开可下载；在此之前 native release 必须失败，不能跳过 bridge 或上传占位清单
4. 再发布首个 `v1.0.0`，核对六个原生 feed、四个 signed bridge installer、`native-transition.json` 与复制的 `latest.json`；每次后续 1.x 都必须保留 bridge feed，旧客户端分两次更新
5. 在 macOS 11/12+ 与 Windows NSIS/MSI 实机验证数据、更新和失败恢复后再标记无缝升级验收通过；macOS 11 始终留在 0.0.12

截至 2026-10-23，远端最新正式版仍为 v0.0.11，v0.0.12 尚未发布；上述发布顺序尚未在真实 release 上执行，当前检查与 fixture 通过不代表真实旧版升级成功

首启保留 `~/.xiaobai-switch/{xiaobai-switch.db,master.key}`，迁移前生成一致性数据库与配套密钥备份，密钥缺失/密文损坏和迁移外键失败显式阻止启动；旧 Rust 与新 Go 同为 AES-256-GCM、12 字节 nonce 前置的 base64 格式，不重新加密或替换原主密钥

回退应用前先退出应用，用对应迁移前备份的数据库与 master.key 成对恢复；原生 schema 2 不能直接交给旧版 schema 1 应用，禁止只降级二进制或只恢复数据库；应用备份恢复不会重新应用四个 CLI 配置，CLI 如需回退应使用其独立应用前备份

Tauri 未配置 publisher 时实际取 bundle identifier 第二段，本项目是 `github`，Cargo authors 不改变 Windows manufacturer；NSIS/MSI 迁移沿用此身份以恢复自定义安装路径并识别旧 MSI，依据 [Tauri NSIS bundler](https://github.com/tauri-apps/tauri/blob/tauri-bundler-v2.8.0/crates/tauri-bundler/src/bundle/windows/nsis/mod.rs) 与 [Tauri MSI bundler](https://github.com/tauri-apps/tauri/blob/tauri-bundler-v2.8.0/crates/tauri-bundler/src/bundle/windows/msi/mod.rs) 独立实现

Website 在 Release / Native Release 成功后重新读取最新已发布 stable release，手动执行 Native Release 的 branch 名不充当 release tag；网站手动 workflow_dispatch 的 release_tag 仍可显式选版，发布 prerelease 不会把网站默认下载切到预览版

## 本轮定向复核记录

- `node --test scripts/native-release-lib.test.mjs scripts/native-release-metadata.test.mjs scripts/native-transition.test.mjs`：9 passed，0 failed；测试只生成临时 Ed25519/minisign fixture，不读正式私钥
- `./scripts/native-go.sh test ./internal/store ./internal/secure`：两包通过，涵盖 legacy 数据迁移、配套密钥 snapshot、外键失败事务回滚和密钥安全；测试仅使用临时目录
- 主进程运行 `cargo test --manifest-path src-tauri/Cargo.toml`：293 passed，0 failed，仅现有 dead_code warnings；`pnpm typecheck` 通过，`pnpm test:run`：45 files / 333 tests passed
- 最终主进程 `./scripts/native-go.sh test -race ./...` 与 `./scripts/native-go.sh vet ./...` 全通过，六目标 cross build 全通过；新版 actionlint 通过，56 项受影响 TypeScript 测试通过（三份：generateUpdaterManifest / packagingWorkflow / websiteReleases）
- bridge alias 定向 `pnpm exec vitest run src/lib/generateUpdaterManifest.test.ts src/lib/packagingWorkflow.test.ts`：20 tests / 2 files passed，验证仅 0.0.12 双清单、完整签名平台内容一致、alias 上传失败传播与公开前上传顺序；`node --check scripts/generate-updater-manifest.mjs` 通过
- 原生更新定向 `test` 与 `vet` 覆盖 `internal/update`、`internal/domain`、`internal/store`、`internal/app`、`locales`；`test -race` 覆盖 update/store，均通过，未用真实 feed 或正式安装

原生自动检查固定一小时，保留启动时与关闭→开启时立即检查；普通设置保存不会额外检查或重置计时，旧 interval 字段读写规范为 60 并保留关闭状态与未知字段；安装成功等待重启时不再后台覆盖更新状态

关于更新页完整显示 release notes 并允许复制，移除 12 行截断，依赖页面原有滚动；完整内容来自 release workflow 共用 notes，页面完整排版仍需实机验收

## Actions 发布日志与完整性

Native Release preflight 先按 tag checkout 并输出 commit SHA，提前要求公开的 `v0.0.12/latest.json`；所有 build/publish 都固定同一 SHA，手动 dispatch 的工作流 branch 不代替 release 内容

复用 `cliff.toml` 与固定 git-cliff v2.14.2 从明确 previous SHA..tag SHA 收集真实 conventional/direct commits，生成共用 `release-notes.md`，剥离旧 ad-hoc/WebView 下载 footer，附固定 Go 1.27.1、mygo v0.6.2 与官方 mygo changelog 链接；构建时要求 RELEASE_NOTES 非空，六份原生 feed、native-transition 和 GitHub Release 正文使用同一正文，发布前再次核对版本与 notes 一致性

每个目标的 test/build/smoke stdout/stderr 经脱敏写 `.log`，成功或失败均上传 diagnostics artifacts，成功发布时日志也附 release；日志命令保留真实退出状态，失败不继续发布；证书/签名私钥不作为诊断材料上传

`SHA256SUMS` 覆盖所有 release 文件但不包含自身；流程先创建 draft 并上传，再完整下载所有 assets 校验 SHA256，全部一致后才公开；prerelease 使用 latest=false，稳定版才设置 latest；preflight/build 为 contents:read，只有 publish 为 contents:write

Windows payload、uninstaller、setup 在 Authenticode 签名后各自执行 `signtool verify /pa`；Apple 临时 keychain 解锁有效期为 21600 秒，完成后移除 runner 管理员域证书信任并删除 keychain，仍需后续导出、配置并在 Actions 验证平台自签证书与更新签名配置

更新日志范围排除当前同点 tag，stable 以上一 stable 为边界以包含 prerelease 中的改动，prerelease 可以上一 prerelease 为边界，未来 HEAD/tag 不混入；脚本用临时 git repo 测试这些边界，多行 secret 的日志脱敏亦有实际进程输出测试

发布途中失败保留未公开 draft 和资产，不自动 clobber；检查失败原因和现有资产后，可人工删除该未发布 draft/资产再重跑同一 tag，已公开版本保持不可变，修复使用新版本继续发布

四份 release / native-release / native-check / website 工作流经 actionlint v1.7.12 检查 exit 0；本机 git-cliff 2.12.0 按工作流同参数在真实 `v0.0.10..v0.0.11` 生成 changelog，确认包含直接 feat 提交且剥离后不含 ad-hoc/WebView2 footer；CI 固定 git-cliff 2.14.2，本机未执行此同版工具

真实网络网站构建两次均因连接 api.github.com 超时未通过，其中第二次已启用 Node 官方 `--use-env-proxy`；不能据此声称真实下载数据构建成功，未修改业务错误处理或添加兜底

此前一轮协作审计：`/root/release_pipeline` 负责发布流水线与日志，`/root/update_schedule` 负责更新调度与日志显示，`/root/upgrade_bridge` 负责旧版过渡及统一发布文档；spawn 工具接受 `gpt-6.1-sol` / `medium` 参数，底层执行模型未独立认证；三项分工结束状态为 completed，真实签名/安装器实机验收和发布仍按上述边界待后续执行

使用仓库现有显式 V1 fixture 执行 `NODE_OPTIONS="--import=$PWD/website/scripts/fixture-fetch.mjs" pnpm --dir website build && node website/scripts/check-download-build.mjs`，exit 0，实际生成 37 页；双语下载链接、Linux 检测、Tab 键盘操作与六平台检查全部通过，这是离线测试数据的页面构建验收，不能代表真实 GitHub 网络构建成功

## 自签流程定向验证

`node --test scripts/native-release-lib.test.mjs scripts/native-release-metadata.test.mjs scripts/native-release-sign.test.mjs scripts/native-transition.test.mjs`：12 passed，0 failed，覆盖无 Apple 公证账号的自签 preflight、缺证书和 ad hoc 拒绝、证书 SHA1 绑定、DMG 非预期验签错误拒绝及 workflow 临时 runner 信任清理合约；测试使用临时签名 fixture，没有访问本机 keychain 或修改本机证书信任

这组检查与 actionlint 不代表 macOS 实际自签打包或 Windows 签名信任执行通过，两个平台仍需在临时 CI runner 使用实际证书验收；本轮不提交、不推送、不发布

主进程独立复核上述四份 Node 测试：12/12 passed，两个 native workflow 的 actionlint 与 `git diff --check` 通过；新 Ed25519 key 的独立 sign/verify/tamper 拒绝和文件权限核验均通过

本轮另用 `/usr/bin/true` 的临时副本验证真实 `AQBot Release` 身份，精确证书 SHA1 为 `319F9217A02978A18033419A6023046CEB2825CA`，`codesign --force --sign SHA1 --timestamp=none` 成功，`codesign --verify --strict -R '=certificate leaf = H"319F9217A02978A18033419A6023046CEB2825CA"'` 成功；fixture 已清理，没有弹窗、证书导出、信任或 ACL 修改，这只证明现有本机身份可签，不证明完整 DMG、公证、其他机器信任或 Windows 实际签名已验证

本轮协作审计：`/root/signing_credentials` 负责凭证定位与新 key 验证，`/root/self_signed_release` 负责自签发布流程与文档；工具接受 `gpt-6.1-sol` / `medium` 参数，底层执行模型未独立认证，两项分工最终状态为 completed；未提交、推送、上传 GitHub secrets 或发布
