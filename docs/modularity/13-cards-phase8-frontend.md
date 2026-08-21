# P8 · 前端对接与继续模块化任务卡

目标：前端保留 Vue3+Vite+TS，不重写。做三件事：① 对接 Go 后端（API 按域拆分 + 实时通道换传输）；
② 继续拆 `progress.md` 点名的巨石页面（协调器模式）；③ 修复实时生命周期缺陷、收窄类型、消除重复。可在 P6 契约冻结后并行。

> 现状（来自 01 §6）：`api/index.ts` 单文件 axios；Socket.IO 仅 `stores/status.ts` 且仅 Dashboard 建连、从不 disconnect；巨石 `Login`(1489)/`AccountModal`(1433)/`Friends`(1413)/`Dashboard`(1399)；多 store 重复 `isCurrentAccount` 守卫、三处重复 theme 应用、`AccountModal` 15 处直连 api 无 store。

---

### 卡片 P8-01：实时通道客户端（替代 socket.io-client）
- 目标：把前端从 Socket.IO 切到 Go 的原生 WS/SSE（P6-03 定的传输），并修复生命周期缺陷。
- 前置依赖：P6-03。
- 源（现状）：`stores/status.ts` 的 `io(...)`、事件 `status:update`/`log:new`/`account-log:new`/`*:snapshot`、`connectRealtime`/`disconnectRealtime`。
- 目标（前端）：新增 `web/src/realtime/client.ts`（轻量 WS/SSE 封装）+ `web/src/composables/useRealtime.ts`。
- 实现要点：
  - 封装 connect/subscribe(accountId)/disconnect/自动重连；保留事件名语义，帧解析 `{event,data}`。
  - **修复**：把 socket 生命周期从 Dashboard 提到 **app 级 composable**（在 `DefaultLayout` 或 app 初始化建连，登出/卸载 disconnect），根治「仅 Dashboard 建连、从不 disconnect、其他页静默退化轮询」。
  - `stores/status.ts` 改为消费该 composable，不再自持 socket。
- 不要做：不保留 socket.io 依赖；不在多个页面各自建连。
- 验收：`pnpm -C web build` + `lint` 通过；实时事件在任意页面在线时到达；登出正确断开。
- 完成判据：☐ WS/SSE 封装 ☐ app 级生命周期 ☐ 事件语义保留 ☐ socket.io 移除。

---

### 卡片 P8-02：API 层按域拆分
- 目标：把单文件 `api/index.ts` 演进为按域模块，收敛重复的账号头注入。
- 前置依赖：P6-04（契约稳定）。
- 源（现状）：`api/index.ts`(77) + 各 store/组件直连 `api`；每调用重复传 `x-account-id`（60+ 处，与拦截器重复）。
- 目标（前端）：`web/src/api/`——`client.ts`(axios 实例+拦截器)、`account.ts`/`friend.ts`/`farm.ts`/`mall.ts`/`task.ts`/`activity.ts`/`user.ts`/`card.ts`/`yyb.ts`/`system.ts` 等按域文件。
- 实现要点：
  - 拦截器统一注入 `x-admin-token` + `x-account-id`（单一真相源），**各 store/组件不再手传账号头**。
  - 契约对齐 P6 handler；错误处理（401 跳登录、`账号未运行`/超时不弹 toast）保留。
  - **消除组件直连 api**：`AccountModal` 15 处、`Login` 4 处、`Dashboard` 2 处等收敛到对应 api 域模块 + store。
- 不要做：不改后端契约来迁就前端；不保留双份账号头来源。
- 验收：`pnpm build`+`lint`；抓包确认账号头仅注入一次；无组件绕过 api 域模块。
- 完成判据：☐ 按域 API 模块 ☐ 账号头单源 ☐ 组件不再直连 ☐ 契约对齐。

---

### 卡片 P8-03：拆分 AccountModal.vue（1433）
- 目标：五种登录流从一个巨石模态拆为协调器 + 子流程组件 + store。
- 前置依赖：P8-02。
- 源（现状）：`AccountModal.vue`——`activeTab` `capture|manual|yyb|yybqr|yyb3rd` 五流 + 嵌套帮助向导 + 15 处直连 api + 两个 poller + 内嵌 admin wx-config 编辑器。
- 目标（前端）：`components/account/AccountModal.vue`(协调器) + `LoginManualTab.vue`/`LoginCaptureTab.vue`/`LoginYybTab.vue`/`LoginYybQrTab.vue`/`LoginThirdPartyTab.vue`/`WxConfigEditor.vue` + `composables/useAccountLogin.ts`（封装 poller、api 调用、状态机）。
- 实现要点：每个 tab 组件只负责其 UI + 事件；轮询/接口/状态机进 composable 或 store；capture 1500ms 与 QR 递归 poller 用 `useIntervalFn`/可取消封装并在卸载清理。
- 不要做：不把五流合回；不在组件内直连 api（走 P8-02 域模块）。
- 验收：`pnpm build`+`lint`；五种登录流手测可用；卸载无残留定时器。
- 完成判据：☐ 五 tab 拆分 ☐ 逻辑进 composable ☐ 无直连 api ☐ poller 清理。

---

### 卡片 P8-04：拆分 Login.vue（1489）
- 目标：登录/注册/找回/续费/领卡/更新日志六合一巨石拆分。
- 前置依赖：P8-02。
- 源（现状）：`Login.vue`——52% 装饰动画 CSS + 六个功能 + 4 处直连 api（renew/card-claim/game-version）。
- 目标（前端）：`views/Login.vue`(协调器) + 复用已有 `LoginModals`/`PasswordStrengthMeter`/`UpdateLogModal` + 各功能逻辑进 `composables/useAuthFlow.ts`；装饰背景抽 `components/login/LoginBackground.vue`（含其 CSS）。
- 实现要点：把各弹窗的 state/handler/校验从父组件下沉到各自组件或 composable（现状是全塞父组件再下钻 ~30 绑定）；直连 api 收敛到 `api/user.ts`/`api/system.ts`；登录后用 `router.push` 而非 `window.location.href`。
- 不要做：不动视觉效果；不保留巨绑定下钻。
- 验收：`pnpm build`+`lint`；六功能手测；路由跳转正常。
- 完成判据：☐ 背景/弹窗抽离 ☐ 逻辑进 composable ☐ 直连收敛 ☐ 绑定面收窄。

---

### 卡片 P8-05：拆分 Friends.vue（1413）与 Dashboard.vue（1399）
- 目标：两大数据页按协调器模式继续拆分。
- 前置依赖：P8-01、P8-02。
- 源（现状）：`Friends.vue`（黑名单/访客/三 Teleport 弹窗内联、`FriendsFriendList` 16 props 含 8 函数 props）；`Dashboard.vue`（引 6 store、内联日志控制台 + 今日统计引擎 + RAF 动画）。
- 目标（前端）：
  - Friends：抽 `FriendsBlacklistTab.vue`/`FriendsVisitorsTab.vue`/`GidManagerModal.vue`/`BatchAddGidModal.vue`/`DeleteConfirmModal.vue`；格式化 helper 进 `composables/useFriendFormatters.ts`，消除向子组件下钻 8 个函数 props。
  - Dashboard：抽 `components/dashboard/LogConsole.vue`（日志解析/合并/过滤/自动滚）+ `TodayStatsPanel.vue`（`OP_META`/统计引擎进 `composables/useTodayStats.ts`）+ `AccountHeader.vue`；倒计时/环形动画进 composable。
- 实现要点：逻辑进 composable，组件只渲染；账号切换的「清理 + 刷新」统一到一个 `useAccountScope` composable，消除现状 BagPanel/FarmPanel/TaskPanel/Dashboard/Friends 各自重复的 watch(currentAccountId)。
- 不要做：不一次性重写；按 tab/区块逐块抽，每块抽完 build 通过再下一块。
- 验收：`pnpm build`+`lint`；两页功能等价手测；无重复账号切换守卫。
- 完成判据：☐ Friends 子组件+formatters ☐ Dashboard 日志/统计/头部抽离 ☐ 账号切换统一 ☐ 逐块可验证。

---

### 卡片 P8-06：收窄类型 + 消除重复 + store 瘦身
- 目标：清理 01 §6 点名的前端耦合坏味。
- 前置依赖：P8-01~05。
- 源（现状）：三处重复 theme 应用（main.ts/App.vue/app.ts）；`setting.ts` 默认值写两遍；`user.ts` 认证+后台 CRUD 双职责；多 store 重复 `isCurrentAccount`；`AccountModal` props `editData?:any` 等 any。
- 目标（前端）：
  - theme 应用统一到 `app.ts`（main/App 只调用它）。
  - `setting.ts` 默认值抽单一常量，init 与 clear 复用。
  - `user.ts` 拆为 `useUserStore`(认证) + `useAdminStore`(后台 CRUD)。
  - `isCurrentAccount` 陈旧响应守卫抽 `composables/useStaleGuard.ts` 复用。
  - 关键 props/接口补类型，消除 `any`（AccountModal/Friends 等）。
- 不要做：不追求一次性全量强类型（先覆盖巨石与公共封装）。
- 验收：`vue-tsc` 无新增错误；`pnpm build`+`lint` 通过；theme/默认值/守卫单点。
- 完成判据：☐ theme 单点 ☐ 默认值单源 ☐ user store 拆分 ☐ 守卫复用 ☐ 关键 any 收窄。
