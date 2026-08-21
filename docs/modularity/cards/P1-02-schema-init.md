# 卡片 P1-02：Schema 设计（0001_init.sql）

| 调度头 | 值 |
| --- | --- |
| 波次 | **W3** |
| 前置依赖 | P1-01 |
| 独占文件 | `internal/store/migrations/0001_init.sql` |
| 可与谁并发 | P2-02 |
| 风险 | 🟠 中（表结构定错会连累所有 repo） |

---

- 目标：定义全部表结构，一次性覆盖现状 JSON 承载的数据。
- 目标（Go）：`internal/store/migrations/0001_init.sql`。
- 实现要点（表清单，字段以现状 JSON 结构为准逐一核对）：
  - `accounts`：id, name/remark, owner_user, yyb_openid, provider, created_at...（源 `store.js` addOrUpdateAccount）。
  - `account_config`：account_id FK, JSON 列或展开列承载 automation/planting_strategy/intervals/fertilizer/bag 等（源 `DEFAULT_ACCOUNT_CONFIG`, store.js:251）。建议**热字段展开 + 其余 JSON 列**折中。
  - `users`：username, pwd_hash(PBKDF2), salt, role, status, expire_at...（源 user-store.js）。
  - `cards`：code, type, duration, status, bound_user, created/claimed_at（卡密系统）。
  - `login_attempts`：ip, count, locked_until（限流）。
  - `login_logs`：user, ip, ua, result, ts。
  - `friend_gid_cache` / `friend_dog_info` / `friend_list_cache`：account_id, payload JSON, updated_at（三套磁盘缓存）。
  - `blacklist`：account_id, gid, reason, added_at。
  - `stats`：account_id, metric, value（或按天）。
  - `global_config`：单行或 key-value（UI 主题、离线提醒 SMTP/push、公告、systemConfig、loginLinks、globalWxConfig、captureConfig、deviceProtocol、antiResaleConfig）。
  - 注：`wechat_accounts`（收编 yyb）放到 **P3-02 的 0002_wechat.sql**，不在本卡，避免与 yyb 平移卡争抢同一文件。
- 不要做：不过度规范化到难以对齐现状；JSON 列是可接受的过渡手段；不在本卡建 wechat/tenant/audit 表（分属 P3-02/PC-02/PC-05）。
- 验收：迁移执行成功；`go test` 断言各表与关键列存在。
- 完成判据：☐ 全部现状数据有落点 ☐ 外键/唯一约束合理 ☐ 迁移通过。
