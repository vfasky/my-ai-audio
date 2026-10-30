<p align="right">
  <strong>简体中文</strong> · <a href="updates.md">English</a>
</p>

# 更新、签名与公证

`go-voice` 通过 MyGo 的签名更新器自动更新，并以 Developer ID 签名、经过公证的应用发布。两套机制用途不同，缺一不可：

| 机制 | 保证 | 密钥 |
| --- | --- | --- |
| Developer ID 签名与 hardened runtime | Gatekeeper 与 TCC 身份：代码来自团队 `92ZWDWJJNB` 且未被修改 | 登录钥匙串中的 Developer ID Application 证书 |
| Apple 公证并 staple | Apple 已扫描这一构建；Gatekeeper 可离线放行 | 钥匙串 profile `NOTARY` 中的 `notarytool` 凭据 |
| Ed25519 更新签名 | 已安装的 `go-voice` 只接受发布密钥签名的归档 | `mygo-update.key`，绝不进入仓库 |

固定的 Developer ID 还让 designated requirement 在更新前后保持不变，因此蓝牙、语音识别、辅助功能与自动化授权在更新后仍然有效。ad hoc 与 Apple Development 构建做不到这一点。

## 更新如何工作

正式版构建会嵌入 `mygo.json` 中的更新源与公钥：

```json
"updates": {
  "publicKey": "kf9mFvBQMctgyCFHkeCemCVejf31Xx2+gNkoFuWFqr4=",
  "github": "vfasky/my-ai-audio",
  "tagPrefix": "go-voice-v",
  "changelog": "docs/CHANGELOG.zh_CN.md"
}
```

- 仓库的 latest release 属于固件，所以桌面版使用独立的 tag 前缀。应用通过 GitHub API 查找最新一个已公开、非草稿、非预发布的 `go-voice-v*` release（未认证时每个地址每小时 60 次请求），读取 `update-darwin-arm64.json`，再下载其中列出的完整归档或差分包。
- manifest 包含版本、发布说明、日期，以及归档和每个差分包的 URL、大小和 Ed25519 签名。应用先用内置公钥校验签名，再替换 `go-voice.app`；差分更新失败时改用完整归档。
- MyGo 原生更新窗口（简体中文）在每次启动约 10 秒后进行后台检查，之后每天检查一次，展示发布说明，并提供“安装更新”“跳过这个版本”“稍后提醒我”。菜单栏 ›“检查更新…”会立即打开它。安装时先暂存已校验的更新。“重新启动”走正常的 `OnBeforeQuit` 退出流程，各 helper 与固件安装进程停止、应用退出后才替换 bundle，再通过 LaunchServices 启动新版本。
- 只有正式版会更新。`build.sh` 和 `dev.sh` 生成的调试版从不读取更新密钥、不生成更新文件，使用“检查更新…”时会说明这一点。正式版必须能写入自身安装位置：管理员的 `/Applications` 或 `~/Applications`；否则菜单会说明它无法在该位置自行更新。
- 应用未启用 App Sandbox，更新写出的 bundle 不带沙盒隔离标记，不需要修复用的 helper。

## 一次性准备

1. **Developer ID。** `security find-identity -v -p codesigning` 必须列出 `Developer ID Application: … (92ZWDWJJNB)`。Git 忽略的 `gui/.local/signing.env` 设置 `MYGO_SIGNING_IDENTITY` 与 `MYGO_TEAM_ID`；已有环境变量优先。
2. **公证凭据。** 按 `macos.notarize.keychainProfile` 指定的名称（`NOTARY`）保存一次：

   ```sh
   xcrun notarytool store-credentials NOTARY
   ```

3. **更新密钥。** 密钥对已用固定版本的 MyGo 生成在独立目录中，避免覆盖其他应用的默认密钥：

   ```sh
   cd gui
   go run github.com/egoist/mygo/cmd/mygo keygen -o "$HOME/Library/Application Support/mygo/update-keys/go-voice"
   ```

   其中 `mygo-update.pub` 即 `updates.publicKey`。请把 `mygo-update.key` 备份到密码管理器，或备份到登录钥匙串（发布脚本也会从这里读取）：

   ```sh
   security add-generic-password -s 'go-voice update signing key' -a mygo-update.key -w
   ```

   （`-w` 不带值时会提示输入，密钥不会出现在进程列表中。）绝不提交、打印或记录私钥。丢失私钥会让所有已安装副本无法再更新；持有私钥的人可以向用户推送代码。已发布的应用绝不要执行 `keygen -force`；换新密钥意味着所有用户都必须手动重装。

## 发布新版本

1. 修改 `mygo.json` 的版本，以及 `resources/FirmwareHelper-Info.plist` 的 `CFBundleShortVersionString`，并递增其 `CFBundleVersion`。
2. 在 [`CHANGELOG.md`](CHANGELOG.md) 与 [`CHANGELOG.zh_CN.md`](CHANGELOG.zh_CN.md) 中都新增 `## <版本>` 小节。中文小节会签入 manifest 并显示在更新窗口中。
3. 提交后，在 `main` 分支、工作区干净时于 `gui/` 中运行：

   ```sh
   ./scripts/release.sh              # 推送 tag 和上传前会询问
   ./scripts/release.sh --no-upload  # 只构建并验证
   ```

构建会将 `MACOSX_DEPLOYMENT_TARGET` 和 C/C++ 编译、链接参数固定为 `mygo.json` 中的 `macos.minimumSystemVersion`。CGO 使用较新 SDK 时，仅设置 Info.plist 不够；显式参数还会使 Go 缓存的旧 C/C++ 对象失效。每次调试与正式构建都会运行 `bash scripts/verify-macos-compatibility.sh <app>`，检查 arm64 支持、主应用及 helper 的 plist 声明、实际 Mach-O 最低系统版本。SDK 版本可以高于编译目标。发布脚本还会检查从更新归档解出的应用。这些检查不能代替在最低支持的 macOS 上进行安装与听写测试。

脚本依次：

1. 检查版本、两份 changelog 小节、分支、工作区、tag，以及该 release 是否已公开。还会检查签名身份、公证 profile 和更新密钥；密钥依次取自 `MYGO_UPDATER_PRIVATE_KEY`、钥匙串条目 `go-voice update signing key`、`~/Library/Application Support/mygo/update-keys/go-voice/mygo-update.key`。
2. 运行 `./scripts/validate.sh`（`--skip-tests` 跳过）。
3. 运行 `./scripts/build.sh --release`：以 release 模式构建各 helper，再执行 `mygo build -platform darwin/arm64 -o build/release -sign <Developer ID>`。MyGo 由内向外签名嵌套的 `BluetoothHelper`、`SpeechBridge`（使用 `entitlements.plist`）和 `FirmwareHelper`，最后签名应用，均启用 hardened runtime 和安全时间戳。随后它制作 `go-voice-<版本>-arm64.dmg` 并签名，提交 `notarytool` 等待结果，再把公证票据 staple 到磁盘映像和应用。最后它归档已 staple 的应用并签名归档，为最多三个已公开版本生成差分包，并写出 `update-darwin-arm64.json`。
4. 验证：macOS 兼容性；`codesign --verify --deep --strict`；应用和每个 helper 的 Developer ID 签发者、团队、hardened runtime 和时间戳；应用与磁盘映像的 `stapler validate` 和 `spctl`；从更新归档解出的应用的兼容性、签名与 staple。
5. 把 `go-voice-<版本>-macOS-AppleSilicon.dmg`、归档、差分包、`SHA256SUMS.txt`、`notes.md` 和 manifest 复制到 `build/updates/<版本>/`。
6. 确认后（`--yes` 跳过询问）推送附注 tag `go-voice-v<版本>`，创建不标记为 latest 的草稿 release，先上传其他文件，最后上传 manifest。

草稿不会推送给已安装的应用。检查说明和文件后再公开：

```sh
gh release edit go-voice-v<版本> --repo vfasky/my-ai-audio --draft=false --latest=false
```

`--publish` 会在上传后立即公开。版本号含 `-` 时作为预发布公开，不会作为更新推送。固件工作流会忽略 `go-voice-v*` tag。

失败后重跑会重新生成文件、覆盖草稿中的文件，并复用指向当前提交的 tag。已公开的 release 不会被修改，应提高版本号。绝不要移动或删除已推送的 tag。

## 常见失败

| 现象 | 处理 |
| --- | --- |
| `Missing signing identity` | 在登录钥匙串安装 Developer ID 证书及其私钥。 |
| `No notary credentials` | 执行 `xcrun notarytool store-credentials NOTARY`。 |
| `notarization failed (Invalid …)` | 用 `xcrun notarytool log <submission id> --keychain-profile NOTARY` 查看被拒文件。 |
| `No update signing key` | 恢复备份的密钥，不要生成新密钥。 |
| `has no "## <版本>" section` | 在两份 changelog 中补上该小节并提交。 |
| `Tag … exists on another commit` / `already published` | 提高版本号。 |

## 验收

单元测试和成功的发布构建不能证明已安装副本能够更新。首次公开发布前，以及签名或更新路径变化时，请在干净的 Mac 账户上检查：

- 下载磁盘映像，把应用拖入 `/Applications`，确认打开时没有 Gatekeeper 警告。
- 授予权限，发布一个更高的测试版本，执行“检查更新…”并安装、重新启动，确认版本已变化、授权仍然有效，蓝牙听写与固件安装仍可使用。
- 对更新后的应用执行 `codesign --verify --deep --strict` 和 `spctl --assess --type execute`。
