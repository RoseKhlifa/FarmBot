# 卡片 P5-07：其余轻量领域（career/illustrated/social）

| 调度头 | 值 |
| --- | --- |
| 波次 | **W10** |
| 前置依赖 | P5-01 |
| 独占文件 | `internal/domain/career/**` `internal/domain/illustrated/**` `internal/domain/social/**` |
| 可与谁并发 | P5-02, P5-04, P5-05, P5-06 |
| 风险 | 🟢 低 |

---

- 目标：搬迁剩余自成一体的 RPC 特性服务。
- 源（现状）：`career-api.js`(297)、`illustrated.js`(258)、`share.js`(174)、`invite.js`(161)、`interact.js`(223)、`dog-gifts.js`、`email.js`(→platform)。
- 目标（Go）：`internal/domain/career/`、`internal/domain/illustrated/`、`internal/domain/social/`（share+invite+interact+dog-gifts 聚合）；`email` 归 `internal/platform/mailer`（与 P5-08 协调，mailer 若已由 P5-08 建则本卡只引用）。
- 实现要点：各为薄 RPC 封装，依赖 game + 必要 store；`interact` 供 friend 注入使用。
- 不要做：不为极小服务过度分包（social 聚合即可）。
- 验收：`go test` 各特性基本用例 + 抽样对拍。
- 完成判据：☐ 三域 + mailer 就位 ☐ 抽样对拍。
