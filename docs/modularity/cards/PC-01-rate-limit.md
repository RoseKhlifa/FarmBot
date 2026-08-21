# 卡片 PC-01：速率限制中间件

| 调度头 | 值 |
| --- | --- |
| 波次 | **W13** |
| 前置依赖 | P6-02 |
| 独占文件 | `internal/httpapi/middleware/ratelimit.go` |
| 可与谁并发 | 全部 PC-* + P6-04（只新增文件，不改 P6 既有文件） |
| 风险 | 🟡 低中 |

---

- 目标：防滥用限流，敏感端点更严。
- 源（现状缺口）：无 API 速率限制；登录仅 IP 计数（`user-store.js` 登录锁定逻辑）。
- 目标（Go）：`internal/httpapi/middleware/ratelimit.go`。
- 实现要点：
  - 每 IP + 每 token 双维度令牌桶；登录/短信/领卡等敏感端点更严阈值。
  - 用 `golang.org/x/time/rate` 或内存滑窗；多实例场景预留 Redis 后端接口（默认单实例内存即可，符合单容器部署）。
  - 命中返回 429 + `Retry-After`；豁免内部健康检查路径。
- 不要做：不默认强依赖 Redis（单容器优先）；不限流健康探针。
- 验收：`go test`——正常放行、超限 429+Retry-After、敏感端点更严。
- 完成判据：☐ 双维度令牌桶 ☐ 敏感端点分级 ☐ 429+Retry-After ☐ 健康路径豁免。
