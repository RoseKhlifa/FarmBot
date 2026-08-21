# 卡片 PC-03：备份与导出

| 调度头 | 值 |
| --- | --- |
| 波次 | **W15** |
| 前置依赖 | P3-02、P0-04（CLI 骨架） |
| 独占文件 | `internal/backup/**`、`cmd/farmbot`（追加 backup/export 子命令） |
| 可与谁并发 | 其它 W15 卡（PC-05、P7-04 除外——P7-04 单独收口） |
| 风险 | 🟡 低中 |

---

- 目标：一致性备份 + 单账号数据导出（迁移 / PIPL 可携权基线）。
- 源（现状缺口）：SQLite 单文件，无备份/导出机制。
- 目标（Go）：`internal/backup/` + `cmd/farmbot backup|export` 子命令。
- 实现要点：
  - `cmd/farmbot backup`：SQLite `VACUUM INTO` 生成一致性快照到 `<DataDir>/backups/farmbot-<ts>.db`（**时间戳由外部传入**，避免脚本内取时钟——对齐 Go 无 `time.Now()` 约束的调度习惯）。
  - 定时备份由**外部 cron / compose sidecar** 触发（不在应用内起后台时钟，保持进程单一职责）；保留 N 份滚动。
  - `export`：管理端可下载单账号数据（迁移 + 可携权）。
- 不要做：不在应用进程内起定时器做备份；不导出他人账号（走权限校验）。
- 验收：`backup` 产出可独立打开的快照；`export` 输出单账号 JSON；滚动保留生效。
- 完成判据：☐ VACUUM INTO 快照 ☐ 外部触发滚动保留 ☐ 单账号导出 ☐ 权限校验。
