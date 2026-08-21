# 卡片 P1-04：Repository 接口 + 实现（用户/卡密/审计）

| 调度头 | 值 |
| --- | --- |
| 波次 | **W4** |
| 前置依赖 | P1-02 |
| 独占文件 | `internal/store/{user_repo,card_repo}.go` |
| 可与谁并发 | P1-03, P2-03, P2-04 |
| 风险 | 🟠 中（计费闭环，错误影响可用性） |

---

- 目标：迁移 SaaS 租户层的持久化。
- 源（现状）：`user-store.js`（1201 行）。
- 目标（Go）：`internal/store/{user_repo,card_repo}.go`。
- 实现要点：
  - `UserRepo`：CRUD、PBKDF2 校验（对齐 user-store.js:231 的哈希参数，保证旧密码可验证）、登录限流/锁定（IP 1 分钟 6 次锁 10 分钟，对齐现状加固值）、默认超管初始化（对齐 `initDefaultAdmin`）、登录审计写入/查询。
  - `CardRepo`：卡密 create/renew/claim（对齐 user-store.js:489-628,948）。
  - **改用原子写/事务**（修复现状 user-store.js 用非原子 `fs.writeFileSync` 的隐患）。
- 不要做：不改卡密业务规则；不改默认管理员账号/密码（admin/admin，部署后由用户改）；操作审计表(PC-05)不在此卡。
- 验收：`go test ./internal/store/...`（PBKDF2 与旧哈希兼容用例；卡密状态机用例；限流锁定用例）。
- 完成判据：☐ 旧密码哈希可校验 ☐ 卡密闭环 ☐ 限流/审计就位 ☐ 测试通过。
