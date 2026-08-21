# 卡片 P1-05：JSON → SQLite 一次性导入器

| 调度头 | 值 |
| --- | --- |
| 波次 | **W5** |
| 前置依赖 | P1-03、P1-04 |
| 独占文件 | `internal/store/migrate_json.go` + `cmd/farmbot/import.go`(新文件) |
| 可与谁并发 | P2-05, P3-01（不与 P0-01 同波，避免争 cmd/farmbot/） |
| 风险 | 🟡 中 |
| co-edit 提示 | 新增 `cmd/farmbot/import.go`；不改 P0-01 的 `main.go`（用同包新文件挂子命令） |

---

- 目标：老用户 `core/data/*.json` 平滑升级到 SQLite。
- 目标（Go）：`internal/store/migrate_json.go` + `cmd/farmbot/import.go` 的 `--import-json <dir>` 子命令（或首启自动探测）。
- 实现要点：
  - 读旧目录全部 JSON（accounts/store/users/cards/login-*/stats/三套缓存），映射写入对应表。
  - **只读不删**旧文件；导入后打印各表计数供人工核对；同名冲突可配置跳过/覆盖。
  - 首次启动若检测到旧 JSON 且新库为空，提示（或自动）执行一次导入。
- 不要做：不删除旧数据；不在导入中夹带 schema 变更；不改 `main.go` 已有内容（新增文件挂子命令）。
- 验收：用一份样例 `data/` 运行导入，`go test` 断言迁入计数与源文件条目一致。
- 完成判据：☐ 全类型数据可迁入 ☐ 计数校验 ☐ 旧文件保留。
