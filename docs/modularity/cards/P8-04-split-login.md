# 卡片 P8-04：拆分 Login.vue（1489）

| 调度头 | 值 |
| --- | --- |
| 波次 | **W13** |
| 前置依赖 | P8-02 |
| 独占文件 | `views/Login.vue`、`components/login/**`、`composables/useAuthFlow.ts` |
| 可与谁并发 | P8-03, P8-05 |
| 风险 | 🟡 低中 |

---

- 目标：登录/注册/找回/续费/领卡/更新日志六合一巨石拆分。
- 源（现状）：`Login.vue`——52% 装饰动画 CSS + 六个功能 + 4 处直连 api（renew/card-claim/game-version）。
- 目标（前端）：`views/Login.vue`(协调器) + 复用已有 `LoginModals`/`PasswordStrengthMeter`/`UpdateLogModal` + 各功能逻辑进 `composables/useAuthFlow.ts`；装饰背景抽 `components/login/LoginBackground.vue`（含其 CSS）。
- 实现要点：把各弹窗的 state/handler/校验从父组件下沉到各自组件或 composable（现状是全塞父组件再下钻 ~30 绑定）；直连 api 收敛到 `api/user.ts`/`api/system.ts`；登录后用 `router.push` 而非 `window.location.href`。
- 不要做：不动视觉效果；不保留巨绑定下钻。
- 验收：`pnpm build`+`lint`；六功能手测；路由跳转正常。
- 完成判据：☐ 背景/弹窗抽离 ☐ 逻辑进 composable ☐ 直连收敛 ☐ 绑定面收窄。
