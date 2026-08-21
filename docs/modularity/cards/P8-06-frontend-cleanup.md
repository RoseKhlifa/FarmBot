# 卡片 P8-06：收窄类型 + 消除重复 + store 瘦身

| 调度头 | 值 |
| --- | --- |
| 波次 | **W14**（前端收口，依赖前五张） |
| 前置依赖 | P8-01~05 |
| 独占文件 | `stores/app.ts`、`stores/setting.ts`、`stores/user.ts`→拆、`composables/useStaleGuard.ts` |
| 可与谁并发 | 无（跨多 store 收口，单独做避免冲突） |
| 风险 | 🟡 低中 |

---

- 目标：清理点名的前端耦合坏味。
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
