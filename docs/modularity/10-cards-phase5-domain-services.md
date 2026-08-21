# P5 · 领域服务任务卡

目标：把 `core/src/services/*.js`（约 40 个）搬到 `internal/domain/*`，**全部接口化**，只依赖 `internal/game`（协议）与
`internal/store`（持久化）接口。拆开 `activity.js`(2300) 与好友密集子图。每域一张卡，可按 farm→friend→warehouse→mall→task→activity→其余 顺序推进，搬完一域即可在 P6 切对应路由。

> 通用约定：
> - 每个领域包暴露一个接口（如 `farm.Service`）+ 一个依赖 `game`/`store` 的实现；由 P4 Runtime 注入并持有实例。
> - 现状的**回调注入**（`status.setRecordGoldExpHook`、`friend-operation-limits.setOn*Callback`）改为**显式接口依赖或返回值**，让依赖图可见（消除 01 §6 的隐藏耦合）。
> - 现状 orchestrator 的 fan-out 保留为「组合根注入子服务」，而非包内 import 巨链。
> - 每域验收：`go test ./internal/domain/<域>/...` + 与 Node 行为对拍（同输入同结果）。

---

### 卡片 P5-01：warehouse（先行，被广泛复用）
- 目标：背包/出售/使用能力，被 friend/planting/task/mall/activity 复用，故先搬。
- 前置依赖：P4-04。
- 源（现状）：`core/src/services/warehouse.js`(724)——`sellAllFruits`、bag 读取/使用、依赖 `network`/`proto`/`store`(isAutomationOn)/`status`。
- 目标（Go）：`internal/domain/warehouse/`（`service.go` 接口 + 实现）。
- 实现要点：`Service{ ListBag, SellAll, SellItem, UseItem, ... }`；依赖 `game.Transport` + `store.ConfigRepo`；`status` 更新改为返回值/回调接口。
- 不要做：不内联 network 单例；不写死账号 env。
- 验收：`go test` + 对拍 `sellAllFruits` 结果。
- 完成判据：☐ 接口化 ☐ 依赖注入 ☐ 对拍一致。

---

### 卡片 P5-02：farm 农场域
- 目标：合并农场相关 7 个服务为一个内聚领域包。
- 前置依赖：P5-01。
- 源（现状）：`farm-api.js`(196)、`farm-land-analyzer.js`(493)、`farm-fertilizer.js`(398)、`planting-service.js`(1168)、`farming-orchestrator.js`(327)、`farm-scheduler.js`(79)、`analytics.js`(150)、`farm.js`(facade)。
- 目标（Go）：`internal/domain/farm/`——`api.go`(原始 RPC)、`land_analyzer.go`、`fertilizer.go`、`planting.go`(种子选择策略)、`analytics.go`(收益/经验排名)、`orchestrator.go`(主循环)、`service.go`(对外接口聚合)。
- 实现要点：
  - `orchestrator` 由组合根注入 api/analyzer/fertilizer/planting，而非包内硬 import 链。
  - `farm-scheduler`(化肥购买 timer) 并入 Runtime 调度器（P4-02）注册，逻辑放 `fertilizer.go`。
  - `analytics` 消费 gameConfig（`assets/gameConfig` via embed）。
- 不要做：不保留 facade 空壳；不跨域 import friend/mall（如需化肥购买调 mall，走注入的 `mall.Service` 接口）。
- 验收：`go test` + 对拍种子选择、地块相位分析、化肥策略。
- 完成判据：☐ 7 服务归一域 ☐ 策略对拍 ☐ 循环可跑。

---

### 卡片 P5-03：friend 好友域（最密子图）
- 目标：合并好友相关 6 个服务，显式化回调依赖。
- 前置依赖：P5-01、P5-02。
- 源（现状）：`friend-api.js`(867)、`friend-operation-limits.js`(616)、`friend-land-analyzer.js`(659)、`friend-visit.js`(991)、`friend-orchestrator.js`(1164, fan-out 10)、`golden-bug-service.js`(178)、`friend.js`(facade)。
- 目标（Go）：`internal/domain/friend/`——`api.go`、`limits.go`(每日 steal/help/bad 限额 + 操作)、`land_analyzer.go`、`visit.go`(进出好友农场+偷/帮序列)、`golden_bug.go`、`orchestrator.go`、`service.go`。
- 实现要点：
  - **消除回调注入**：`limits.go` 的 `setOnExpLimitReached/ResetCallback` 改为 orchestrator 通过接口方法查询/订阅（如 `limits.OnExpLimit() <-chan Event` 或返回结构体），依赖显式化。
  - 好友 GID/狗信息/列表缓存走 `store.CacheRepo`（P1-03）；黑名单增删走 CacheRepo。
  - quiet-hours、GID 归一化随 `api.go`。
  - orchestrator 依赖注入 farm/warehouse/limits/visit/analyzer + CacheRepo。
- 不要做：不保留隐藏回调；不在 analyzer 里 lazy-require api（Go 无循环 require，直接依赖）。
- 验收：`go test`——限额边界用例（每日上限/重置）、偷帮序列、黑名单；对拍 Node。
- 完成判据：☐ 6 服务归一域 ☐ 回调显式化 ☐ 限额对拍 ☐ 缓存走 repo。

---

### 卡片 P5-04：mall 商城/货币域
- 目标：合并商城、神秘商店、月卡、qqvip。
- 前置依赖：P5-01。
- 源（现状）：`mall.js`(611)、`mystery-shop.js`(86)+`mystery-scheduler.js`(110)、`monthcard.js`(177)、`qqvip.js`(168)。
- 目标（Go）：`internal/domain/mall/`——`mall.go`、`mystery.go`(+调度注册到 P4-02)、`monthcard.go`、`qqvip.go`、`service.go`。
- 实现要点：化肥购买供 farm 调用（farm 注入 `mall.Service`）；mystery 自动买调度并入 Runtime 调度器。
- 不要做：不 lazy-require warehouse（直接注入接口）。
- 验收：`go test` + 对拍神秘商店购买/月卡领取。
- 完成判据：☐ 4 服务归一域 ☐ 调度并入 ☐ 对拍一致。

---

### 卡片 P5-05：task 任务域
- 目标：任务/每日奖励领取，监听 taskInfoNotify。
- 前置依赖：P5-01。
- 源（现状）：`task.js`(552)——监听 `taskInfoNotify`，依赖 network/proto/store/scheduler/stats + lazy warehouse。
- 目标（Go）：`internal/domain/task/`（`service.go`）。
- 实现要点：订阅 P2-04 分发的 `taskInfoNotify` 事件；领取调 warehouse 注入接口；调度并入 P4-02。
- 验收：`go test` + 对拍任务领取。
- 完成判据：☐ 事件订阅 ☐ 领取对拍 ☐ 调度并入。

---
### 卡片 P5-06：activity 活动域拆分（2300 行巨石 → 一活动一模块）
- 目标：把七个无关限时活动 + 手写 protobuf 解码器拆成内聚小模块。这是最大的单文件模块化。
- 前置依赖：P5-01、P2-01。
- 源（现状）：`core/src/services/activity.js`(2300)。七活动 + 约一半篇幅的线格式扫描器（01 §4、领域分析 §5）：
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

---

### 卡片 P5-07：其余轻量领域（career/illustrated/social）
- 目标：搬迁剩余自成一体的 RPC 特性服务。
- 前置依赖：P5-01。
- 源（现状）：`career-api.js`(297)、`illustrated.js`(258)、`share.js`(174)、`invite.js`(161)、`interact.js`(223)、`dog-gifts.js`、`email.js`(→platform)。
- 目标（Go）：`internal/domain/career/`、`internal/domain/illustrated/`、`internal/domain/social/`（share+invite+interact+dog-gifts 聚合）；`email` 归 `internal/platform/mailer`。
- 实现要点：各为薄 RPC 封装，依赖 game + 必要 store；`interact` 供 friend 注入使用。
- 不要做：不为极小服务过度分包（social 聚合即可）。
- 验收：`go test` 各特性基本用例 + 抽样对拍。
- 完成判据：☐ 三域 + mailer 就位 ☐ 抽样对拍。

---

### 卡片 P5-08：license 与 platform 基础设施
- 目标：搬迁授权与跨领域基础设施。
- 前置依赖：P0-03、P1-01。
- 源（现状）：`services/license.js`(213, 机器码绑定 AES-256-CBC+MD5, `FARM_LICENSE_ENABLED`)、`push.js`、`email.js`、`utils/machine-id.js`、`account-resolver.js`。
- 目标（Go）：`internal/license/license.go`、`internal/platform/{mailer,pusher,machineid}/`。
- 实现要点：
  - license 校验保持算法一致（机器码派生 + 加密校验）；`FARM_LICENSE_ENABLED=false` 默认关闭（对齐现状 client.js:22 gate）。
  - `account-resolver` 的引用解析（id/name/ref → accountID）改为 store 层或 handler 层的小工具函数。
- 不要做：不改授权加密方案；不弱化默认关闭行为。
- 验收：`go test`——license 启用/关闭分支；机器码稳定性。
- 完成判据：☐ license 对齐 ☐ mailer/pusher/machineid 就位。

