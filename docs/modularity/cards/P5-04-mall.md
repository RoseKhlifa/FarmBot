# 卡片 P5-04：mall 商城/货币域

| 调度头 | 值 |
| --- | --- |
| 波次 | **W10** |
| 前置依赖 | P5-01 |
| 独占文件 | `internal/domain/mall/**` |
| 可与谁并发 | P5-02, P5-05, P5-06, P5-07 |
| 风险 | 🟡 中 |

---

- 目标：合并商城、神秘商店、月卡、qqvip。
- 源（现状）：`mall.js`(611)、`mystery-shop.js`(86)+`mystery-scheduler.js`(110)、`monthcard.js`(177)、`qqvip.js`(168)。
- 目标（Go）：`internal/domain/mall/`——`mall.go`、`mystery.go`(+调度注册到 P4-02)、`monthcard.go`、`qqvip.go`、`service.go`。
- 实现要点：化肥购买供 farm 调用（farm 注入 `mall.Service`）；mystery 自动买调度并入 Runtime 调度器。
- 不要做：不 lazy-require warehouse（直接注入接口）。
- 验收：`go test` + 对拍神秘商店购买/月卡领取。
- 完成判据：☐ 4 服务归一域 ☐ 调度并入 ☐ 对拍一致。
