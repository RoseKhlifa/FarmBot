# 卡片 P8-05：拆分 Friends.vue（1413）与 Dashboard.vue（1399）

| 调度头 | 值 |
| --- | --- |
| 波次 | **W13** |
| 前置依赖 | P8-01、P8-02 |
| 独占文件 | `components/friends/**`、`components/dashboard/**`、相关 composables |
| 可与谁并发 | P8-03, P8-04 |
| 风险 | 🟠 中 |

---

- 目标：两大数据页按协调器模式继续拆分。
- 源（现状）：`Friends.vue`（黑名单/访客/三 Teleport 弹窗内联、`FriendsFriendList` 16 props 含 8 函数 props）；`Dashboard.vue`（引 6 store、内联日志控制台 + 今日统计引擎 + RAF 动画）。
- 目标（前端）：
  - Friends：抽 `FriendsBlacklistTab.vue`/`FriendsVisitorsTab.vue`/`GidManagerModal.vue`/`BatchAddGidModal.vue`/`DeleteConfirmModal.vue`；格式化 helper 进 `composables/useFriendFormatters.ts`，消除向子组件下钻 8 个函数 props。
  - Dashboard：抽 `components/dashboard/LogConsole.vue`（日志解析/合并/过滤/自动滚）+ `TodayStatsPanel.vue`（`OP_META`/统计引擎进 `composables/useTodayStats.ts`）+ `AccountHeader.vue`；倒计时/环形动画进 composable。
- 实现要点：逻辑进 composable，组件只渲染；账号切换的「清理 + 刷新」统一到一个 `useAccountScope` composable，消除现状 BagPanel/FarmPanel/TaskPanel/Dashboard/Friends 各自重复的 watch(currentAccountId)。
- 不要做：不一次性重写；按 tab/区块逐块抽，每块抽完 build 通过再下一块。
- 验收：`pnpm build`+`lint`；两页功能等价手测；无重复账号切换守卫。
- 完成判据：☐ Friends 子组件+formatters ☐ Dashboard 日志/统计/头部抽离 ☐ 账号切换统一 ☐ 逐块可验证。
