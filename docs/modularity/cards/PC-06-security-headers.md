# 卡片 PC-06：安全响应头

| 调度头 | 值 |
| --- | --- |
| 波次 | **W13** |
| 前置依赖 | P6-02 |
| 独占文件 | `internal/httpapi/middleware/security.go` |
| 可与谁并发 | 全部 PC-* + P6-04（只新增文件） |
| 风险 | 🟡 低 |

---

- 目标：统一安全响应头，收敛 SPA 的 script/连接源。
- 源（现状）：有基础 CORS；缺安全响应头/HSTS；`/login-assets` SVG 有独立 CSP sandbox。
- 目标（Go）：`internal/httpapi/middleware/security.go`。
- 实现要点：
  - 统一注入 `X-Content-Type-Options:nosniff`、`X-Frame-Options:DENY`、`Referrer-Policy`、`Content-Security-Policy`（对 SPA 收敛 script/连接源）、生产启用 `HSTS`。
  - 保留现状对 `/login-assets` SVG 的 CSP sandbox 加固，统一到该中间件。
  - CSP 需允许 WS/SSE 连接源（对齐 P6-03 实时通道）与内嵌前端资源；生产/开发可分档。
- 不要做：不设过严 CSP 打断前端或实时通道（先测再收紧）。
- 验收：响应头就位；前端 + 实时通道在 CSP 下正常；SVG sandbox 保留。
- 完成判据：☐ 五类安全头 ☐ HSTS（生产）☐ CSP 兼容 SPA+WS ☐ SVG sandbox 统一。
