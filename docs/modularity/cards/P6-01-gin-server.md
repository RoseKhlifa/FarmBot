# 卡片 P6-01：Gin 服务器组装 + 静态托管 + SPA fallback

| 调度头 | 值 |
| --- | --- |
| 波次 | **W9** |
| 前置依赖 | P0-02、P4-03 |
| 独占文件 | `internal/httpapi/server.go` |
| 可与谁并发 | P5-01（不同目录树） |
| 风险 | 🟠 中 |

---

- 目标：立起 HTTP 服务器骨架，托管前端，优雅退出。
- 源（现状）：`admin.js` startAdminServer 主干、`configureStaticAssets`(:168)、`registerSpaFallback`(:227)、`/game-config`(:368)、`/login-assets`(:409)、`/api/game-asset` CDN 代理(:391)。
- 目标（Go）：`internal/httpapi/server.go`。
- 实现要点：
  - Gin 引擎；`embed.FS` 托管 `web/dist`（P7 内嵌），SPA fallback：非 `/api`、非 `/game-config` 的 GET 回 `index.html`，`/api` 未匹配返回 404 JSON。
  - `/game-config`、`/login-assets`（保留 `X-Content-Type-Options:nosniff` + SVG 的 CSP sandbox 加固）、`/api/game-asset` CDN 图片代理。
  - 监听 `ADMIN_PORT`（默认 3007）绑定 `0.0.0.0`；`http.Server` + `context` 优雅退出（SIGINT/SIGTERM）。
  - 装配唯一入口：从 `internal/app.Application` 取依赖，handler 只依赖 Application 暴露的服务接口。
- 不要做：不手工逐个 register（改用分组路由 + handler 结构体）；不在 server.go 写业务。
- 验收：`go run` 起服务，`/api/health` 200，SPA fallback 返回 index，静态资源可取。
- 完成判据：☐ Gin 骨架 ☐ 内嵌前端托管 ☐ SPA fallback ☐ 优雅退出。
