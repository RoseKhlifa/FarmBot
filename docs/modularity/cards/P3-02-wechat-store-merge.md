# 卡片 P3-02：微信账号存储并入主库

| 调度头 | 值 |
| --- | --- |
| 波次 | **W6** |
| 前置依赖 | P3-01、P1-02 |
| 独占文件 | `internal/store/migrations/0002_wechat.sql` `internal/yyb/store.go`(co-edit) |
| 可与谁并发 | P3-03 |
| 风险 | 🟡 中 |
| co-edit 提示 | 改写 P3-01 建的 `internal/yyb/store.go`，故排在 P3-01 之后、不与其同波 |

---

- 目标：yyb 的 `wechat_accounts`/`sessions`/`features` 并入主 SQLite（或同库独立表），消除双份身份。
- 源（现状）：`yyb-go/internal/store/store.go`（自带 SQLite，表 `wechat_accounts`(openid UNIQUE)/`sessions`/`features`）。
- 目标（Go）：`internal/yyb/store.go` 改为走主库 `internal/store` 的连接；表定义放 `0002_wechat.sql`。
- 实现要点：
  - 把 yyb 的建表/查询迁到主库连接；`sessions`（per account+tcp_proxy 的 MMTLS blob）、`features`（getCode/getPhoneNumber/operateWxData 开关）一并纳入。
  - farmbot 账号通过 `yyb_openid` 关联微信账号（P1-02 `accounts.yyb_openid`），实现单一身份。
  - 头像/QR 图片目录仍可用磁盘（`<DataDir>/yyb/{avatars,qr}`），无需入库。
  - **商用衔接**：`login_buffer`/`credentials`/`sessions` blob 的静态加密在 PC-02 落地；本卡把列留好，不在此实现加密。
- 不要做：不保留 yyb 的独立 db 文件；不做双写。
- 验收：`go test`——微信账号 upsert/list/delete 往返；与主库同事务无冲突。
- 完成判据：☐ 表并入主库 ☐ openid 关联就位 ☐ 独立 db 移除。
