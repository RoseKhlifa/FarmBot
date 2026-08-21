# 卡片 P7-02：跨平台发布构建

| 调度头 | 值 |
| --- | --- |
| 波次 | **W13** |
| 前置依赖 | P7-01、P0-04 |
| 独占文件 | `Makefile`（release 目标；+ 可选 `.goreleaser.yaml`） |
| 可与谁并发 | P7-03（都改构建，但文件不同：Makefile vs Dockerfile） |
| 风险 | 🟡 中 |

---

- 目标：交叉编译出 Windows/Linux/macOS 二进制，替代 pkg。
- 源（现状）：`package:release`(pkg)、`start.bat`/`start.sh` 源码启动脚本。
- 目标（Go）：`Makefile` `release` 目标（+ 可选 GoReleaser）。
- 实现要点：
  - `make build-web`（`pnpm -C web build`）→ `GOOS/GOARCH` 矩阵 `go build -ldflags "-s -w -X main.version=..."`。
  - **纯 Go 依赖保证交叉编译**：wazero + modernc.org/sqlite 均无 CGO，`CGO_ENABLED=0` 可静态交叉编译到各平台（这是选型的关键回报）。
  - 版本号注入（对齐现状 `__APP_VERSION__` 来自 core/package.json）。
- 不要做：不引入 CGO 依赖（会破坏交叉编译与单文件目标）。
- 验收：`make release` 产出至少 win-amd64 / linux-amd64 两个可运行二进制。
- 完成判据：☐ 交叉编译成功 ☐ CGO_ENABLED=0 ☐ 版本注入 ☐ 二进制自带 yyb。

> 商用备注：二进制发布虽非首选部署方式，但交叉编译能力是「纯 Go 无 CGO」选型的验证点，务必保留。首选仍是 Docker（P7-03）。
