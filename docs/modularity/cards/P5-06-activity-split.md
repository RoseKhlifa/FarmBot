# 卡片 P5-06：activity 活动域拆分（2300 行巨石 → 一活动一模块）

| 调度头 | 值 |
| --- | --- |
| 波次 | **W10** |
| 前置依赖 | P5-01、P2-01 |
| 独占文件 | `internal/domain/activity/**` |
| 可与谁并发 | P5-02, P5-04, P5-05, P5-07 |
| 风险 | 🟠 中（手写解码器易错，需 fixture 对拍） |

---

- 目标：把七个无关限时活动 + 手写 protobuf 解码器拆成内聚小模块。这是最大的单文件模块化。
- 源（现状）：`core/src/services/activity.js`(2300)。七活动 + 约一半篇幅的线格式扫描器：
  - 南瓜商店：`getNanguaShop`(:1968)/`buyNanguaShopItem`(:358)/`refreshNanguaShop`(:394)
  - 荷露：`getHeluActivity`(:1632)/`drawHeluGiftLotus`(:1710)/`exchangeHeluShopItem`(:1817)/`computeHeluDrawActions`(:1338)
  - 青梅：`getQingmeiActivity`(:1137)/`claimQingmeiSeeds`(:1175)/`brewAndSellQingmeiWine`(:1225) + 4 normalizer
  - 季节通行证：`getSeasonPassport`(:1511)/`claimSeasonPassportRewards`(:1515)
  - 节气：`getSolarTermsInfo`(:1537)/`claimSolarTermsReward`(:1541)
  - 观星礼录：`getGuanxingActivity`(:2204)/`claimGuanxingRewards`(:2215)
  - 星砂（stub :2255）
  - 共享：`getActivityGroup`(:131)/`operateActivity`(:154)；解码器 `readProtoFields`(:462)/`scanLengthDelimitedFields`(:884)/`getProtoNumber/Bytes/String`/`parseActivityItemMessage`/`scanRandomShopInfoFromRawBody`/`scanExchangeShopInfoFromRawBody`/`scanDrawInfoFromRawBody`。
- 目标（Go）：`internal/domain/activity/`——
  - `core.go`（`getActivityGroup`/`operateActivity` 共享 RPC）
  - `decode.go`（共享线格式扫描器；**优先尝试用 P2-01 生成的 activitypb 正式解码**，仅对无 schema 的 opaque bytes 保留手写扫描器）
  - `nangua.go` `helu.go` `qingmei.go` `season.go` `solarterms.go` `guanxing.go` `starsand.go`（一活动一文件）
  - `service.go`（聚合对外接口，供 P6 handler 与 Runtime 调度调用）
- 实现要点：
  - 每活动内部只依赖 `core.go` + `decode.go` + warehouse 注入接口 + gameConfig。
  - `isQingmeiClaimedToday` 这类进程内「今日已领」标志改为 Runtime 实例字段（不再包级状态）。
  - 消费者当前在 `core/worker.js:946-1043` lazy-require；Go 侧由 Runtime 持 `activity.Service` 直接调。
- 不要做：不把七活动合回一个大文件；不在无必要时保留手写解码（能用 proto 就用 proto）。
- 验收：`go test ./internal/domain/activity/...`——每活动 get/operate 各一用例；解码器对已知 fixture 输出与 Node 一致。
- 完成判据：☐ 七活动分文件 ☐ 解码器独立且优先用 proto ☐ 今日标志实例化 ☐ 逐活动对拍。
