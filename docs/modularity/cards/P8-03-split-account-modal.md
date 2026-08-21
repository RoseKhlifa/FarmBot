# 卡片 P8-03：拆分 AccountModal.vue（1433）

| 调度头 | 值 |
| --- | --- |
| 波次 | **W13** |
| 前置依赖 | P8-02 |
| 独占文件 | `components/account/**`、`composables/useAccountLogin.ts` |
| 可与谁并发 | P8-04, P8-05（各自独占不同页面/组件目录） |
| 风险 | 🟠 中 |

---

- 目标：五种登录流从一个巨石模态拆为协调器 + 子流程组件 + store。
- 源（现状）：`AccountModal.vue`——`activeTab` `capture|manual|yyb|yybqr|yyb3rd` 五流 + 嵌套帮助向导 + 15 处直连 api + 两个 poller + 内嵌 admin wx-config 编辑器。
- 目标（前端）：`components/account/AccountModal.vue`(协调器) + `LoginManualTab.vue`/`LoginCaptureTab.vue`/`LoginYybTab.vue`/`LoginYybQrTab.vue`/`LoginThirdPartyTab.vue`/`WxConfigEditor.vue` + `composables/useAccountLogin.ts`（封装 poller、api 调用、状态机）。
- 实现要点：每个 tab 组件只负责其 UI + 事件；轮询/接口/状态机进 composable 或 store；capture 1500ms 与 QR 递归 poller 用 `useIntervalFn`/可取消封装并在卸载清理。
- 不要做：不把五流合回；不在组件内直连 api（走 P8-02 域模块）。
- 验收：`pnpm build`+`lint`；五种登录流手测可用；卸载无残留定时器。
- 完成判据：☐ 五 tab 拆分 ☐ 逻辑进 composable ☐ 无直连 api ☐ poller 清理。
