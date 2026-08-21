# 卡片 P1-01：SQLite 打开 + 迁移框架

| 调度头 | 值 |
| --- | --- |
| 波次 | **W2** |
| 前置依赖 | P0-02 |
| 独占文件 | `internal/store/db.go` `internal/store/migrations/`(框架，不含 0001) |
| 可与谁并发 | P2-01 |
| 风险 | 🟢 低 |

---

- 目标：建立 DB 连接、WAL、版本化迁移执行器。
- 目标（Go）：`internal/store/db.go` + `internal/store/migrations/`。
- 实现要点：
  - `modernc.org/sqlite`（纯 Go）；打开 `<DataDir>/farmbot.db`，`PRAGMA journal_mode=WAL; busy_timeout=5000`。
  - 简单迁移器：`embed` 读 `migrations/*.sql` 按序号执行，记录 `schema_migrations` 表；幂等（已执行的跳过）。
  - 提供 `Open(cfg) (*DB, error)` 与 `Close()`；连接池上限设小（SQLite 写单线程友好）。
- 不要做：不引入重型 ORM 迁移工具；不做多数据库抽象；不在本卡写具体表（那是 P1-02）。
- 验收：`go test ./internal/store/...`（建库→执行迁移→断言 `schema_migrations` 存在）。
- 完成判据：☐ DB 可打开 ☐ 迁移可重复执行幂等 ☐ WAL 生效。
