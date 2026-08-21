# 卡片 P3-01：yyb 协议/扫码子包平移

| 调度头 | 值 |
| --- | --- |
| 波次 | **W5** |
| 前置依赖 | P0-01 |
| 独占文件 | `internal/yyb/protocol/**` `internal/yyb/qr/**` `internal/yyb/store.go`(初版) |
| 可与谁并发 | P1-05, P2-05 |
| 风险 | 🟡 中（平移为主，改 import 路径） |

---

- 目标：把独立 `yyb-go` 进程的协议与扫码实现原样搬进主 module，编译通过。这是消除「需分别部署两个项目」痛点的核心阶段。
- 源（现状）：`yyb-go/internal/protocol/*`（mmtls/ilink/shortlink/transport/newdns/loginbuffer/bytes）、`yyb-go/internal/qr/*`、`yyb-go/internal/store/*`。
- 目标（Go）：`internal/yyb/protocol/`、`internal/yyb/qr/`、`internal/yyb/store.go`。
- 实现要点：
  - 直接复制源码，改包路径与 import 到 `github.com/<owner>/farmbot/internal/yyb/...`。这些子包本就是纯 Go 且相对自洽，改动主要是 import 路径与 package 名。
  - 合并依赖到根 `go.mod`（P0-01 已合并 gin/modernc 等；此处补齐 protocol 用到的加密/编码库）。
  - 保留 OAuth 常量（appid `wxd44977328b36e647`、回调 `yybadaccess.3g.qq.com`、小程序 `wx5306c5978fdb76e4`）。
- 不要做：不重写微信协议实现；不改协议常量。
- 验收：`go build ./internal/yyb/...` 通过；yyb-go 原有单测（若有）迁移后通过。
- 完成判据：☐ 三子包平移 ☐ import 路径修正 ☐ 编译通过。
