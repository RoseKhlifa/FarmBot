# 卡片 P1-03：Repository 接口 + 实现（账号/配置/缓存/统计）

| 调度头 | 值 |
| --- | --- |
| 波次 | **W4** |
| 前置依赖 | P1-02 |
| 独占文件 | `internal/store/{account_repo,cache_repo,config_repo,stats_repo}.go` |
| 可与谁并发 | P1-04, P2-03, P2-04 |
| 风险 | 🟠 中 |

---

- 目标：领域层只依赖接口，隔离持久化细节。
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
