# P1 · 持久化任务卡

目标：用单一 SQLite（`data/farmbot.db`, WAL）+ repository 接口，替代现状散落的 JSON 文件与无锁并发写。
提供 JSON→SQLite 一次性导入器保证老用户平滑升级。可与 P2 并行。

> 现状数据文件与归属（来自 01 §4）：`store.json`/`accounts.json`/`known_friend_gids/`/`friend_dog_info/`/
> `friend_list_cache/`（store.js）；`users.json`/`cards.json`/`login-attempts.json`/`card-claim.json`/`login-logs.json`（user-store.js）；`stats/<id>.json`（stats.js）；`license.json`（license.js）。

---

### 卡片 P1-01：SQLite 打开 + 迁移框架
- 目标：建立 DB 连接、WAL、版本化迁移执行器。
- 前置依赖：P0-02。
- 目标（Go）：`internal/store/db.go` + `internal/store/migrations/`。
- 实现要点：
  - `modernc.org/sqlite`（纯 Go）；打开 `<DataDir>/farmbot.db`，`PRAGMA journal_mode=WAL; busy_timeout=5000`。
  - 简单迁移器：读 `migrations/*.sql` 按序号执行，记录 `schema_migrations` 表。
  - 提供 `Open(cfg) (*DB, error)` 与 `Close()`；连接池上限设小（SQLite 写单线程友好）。
- 不要做：不引入重型 ORM 迁移工具；不做多数据库抽象。
- 验收：`go test ./internal/store/...`（建库→执行迁移→断言表存在）。
- 完成判据：☐ DB 可打开 ☐ 迁移可重复执行幂等 ☐ WAL 生效。

---

### 卡片 P1-02：Schema 设计（0001_init.sql）
- 目标：定义全部表结构，一次性覆盖现状 JSON 承载的数据。
- 前置依赖：P1-01。
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
  - `wechat_accounts`：收编 yyb（openid UNIQUE, uin, login_buffer, credentials JSON, user_info JSON, avatar, status）——见 P3。
- 不要做：不过度规范化到难以对齐现状；JSON 列是可接受的过渡手段。
- 验收：迁移执行成功；`go test` 断言各表与关键列存在。
- 完成判据：☐ 全部现状数据有落点 ☐ 外键/唯一约束合理 ☐ 迁移通过。

---

### 卡片 P1-03：Repository 接口 + 实现（账号/配置/缓存/统计）
- 目标：领域层只依赖接口，隔离持久化细节。
- 前置依赖：P1-02。
- 源（现状）：`store.js` ~89 导出，按域分组（01 §4）。
- 目标（Go）：`internal/store/{account_repo,cache_repo,config_repo,stats_repo}.go`，各含接口 + SQLite 实现。
- 实现要点：
  - `AccountRepo`：List/Get/Upsert/Delete/GetByUser + 配置读写（GetConfig/ApplyConfigSnapshot，对齐 store.js 的 `getConfigSnapshot`/`applyConfigSnapshot`/`getAutomation`/`getPlantingStrategy`/`getIntervals` 等）。
  - `CacheRepo`：好友 GID/狗信息/列表的读写与失效（对齐 store.js 缓存 + `removeFriendFromCache`/`deleteAccountCaches`）+ 黑名单增删改。
  - `ConfigRepo`：全局/系统/wx/主题/防倒卖/离线提醒配置读写。
  - `StatsRepo`：per-account 计数读写（对齐 stats.js `operations`）。
  - **移除 `FARM_ACCOUNT_ID` 隐式全局**：所有方法显式接受 `accountID` 参数（根治 store.js:498 的隐式回退）。
  - 写操作用短事务；账号级写建议串行化（配合 P4 Runtime 的单 goroutine 写）。
- 不要做：不把业务规则塞进 repo（repo 只做存取）；不保留 env 隐式账号解析。
- 验收：`go test ./internal/store/...`（对每个 repo 做 CRUD 往返 + 并发写不损坏）。
- 完成判据：☐ 四类 repo 接口+实现就位 ☐ 显式 accountID ☐ 测试通过。

---

### 卡片 P1-04：Repository 接口 + 实现（用户/卡密/审计）
- 目标：迁移 SaaS 租户层的持久化。
- 前置依赖：P1-02。
- 源（现状）：`user-store.js`（1201 行）。
- 目标（Go）：`internal/store/{user_repo,card_repo}.go`。
- 实现要点：
  - `UserRepo`：CRUD、PBKDF2 校验（对齐 user-store.js:231 的哈希参数，保证旧密码可验证）、登录限流/锁定（IP 1 分钟 6 次锁 10 分钟，对齐现状加固值）、默认超管初始化（对齐 `initDefaultAdmin`）、登录审计写入/查询。
  - `CardRepo`：卡密 create/renew/claim（对齐 user-store.js:489-628,948）。
  - **改用原子写/事务**（修复现状 user-store.js 用非原子 `fs.writeFileSync` 的隐患）。
- 不要做：不改卡密业务规则；不改默认管理员账号/密码（admin/admin，部署后由用户改）。
- 验收：`go test ./internal/store/...`（PBKDF2 与旧哈希兼容用例；卡密状态机用例；限流锁定用例）。
- 完成判据：☐ 旧密码哈希可校验 ☐ 卡密闭环 ☐ 限流/审计就位 ☐ 测试通过。

---

### 卡片 P1-05：JSON → SQLite 一次性导入器
- 目标：老用户 `core/data/*.json` 平滑升级到 SQLite。
- 前置依赖：P1-03、P1-04。
- 目标（Go）：`internal/store/migrate_json.go` + `cmd/farmbot` 的 `--import-json <dir>` 子命令（或首启自动探测）。
- 实现要点：
  - 读旧目录全部 JSON（accounts/store/users/cards/login-*/stats/三套缓存），映射写入对应表。
  - **只读不删**旧文件；导入后打印各表计数供人工核对；同名冲突可配置跳过/覆盖。
  - 首次启动若检测到旧 JSON 且新库为空，提示（或自动）执行一次导入。
- 不要做：不删除旧数据；不在导入中夹带 schema 变更。
- 验收：用一份样例 `data/` 运行导入，`go test` 断言迁入计数与源文件条目一致。
- 完成判据：☐ 全类型数据可迁入 ☐ 计数校验 ☐ 旧文件保留。
