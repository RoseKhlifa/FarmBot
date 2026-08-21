# 卡片 P6-02：中间件链（鉴权/账号/CORS/超时/角色）

| 调度头 | 值 |
| --- | --- |
| 波次 | **W10** |
| 前置依赖 | P6-01、P1-04 |
| 独占文件 | `internal/httpapi/middleware/{cors,auth,session,account,timeout,role}.go` |
| 可与谁并发 | P5 各域（不同目录树） |
| 风险 | 🟠 中高（鉴权门是安全边界，商用尤重） |

---

- 目标：复刻现状显式中间件顺序与鉴权门。
- 源（现状）：`admin.js` 中间件顺序、`registerAuthGate`(:179, allowlist `PUBLIC_API_PATHS`)、`admin-session-manager.js`、`admin-route-helpers.js`(`requireAdminRole`:112/`requireSuperAdminRole`:126)、`getAccountIdFromRequest`/`canAccessAccount`。
- 目标（Go）：`internal/httpapi/middleware/{cors,auth,session,account,timeout,role}.go`。
- 实现要点：
  - 顺序：CORS → 静态/资源 → 鉴权路由(login) → 健康 → **鉴权门**（`x-admin-token` 校验 + PUBLIC allowlist）→ 超时守卫 → 业务路由 → SPA fallback。
  - `auth.go`：校验 `x-admin-token`，注入 `currentUser`；封禁/过期卡用户强制失效。
  - `session.go`：**改进点**——会话可选持久化到 SQLite（修复现状重启即失效、不跨实例）；token 仍 `crypto/rand`。
  - `account.go`：解析 `x-account-id`（对齐 `resolveAccountId`），非特权角色做 per-account 访问控制。
  - `role.go`：admin / super-admin 门。
  - 5 分钟清扫过期会话（对齐现状 cleanup）。
  - **商用衔接**：PC-01（限流/CSRF/安全头）、PC-05（审计）会在此链插入中间件，本卡把链设计成可追加。
- 不要做：不改鉴权头名；不放宽 PUBLIC allowlist（逐条对照现状）。
- 验收：`go test`——PUBLIC 路径放行、非法 token 401、角色门 403、账号访问控制。
- 完成判据：☐ 中间件顺序一致 ☐ allowlist 对齐 ☐ 会话可持久化 ☐ 角色/账号门。
