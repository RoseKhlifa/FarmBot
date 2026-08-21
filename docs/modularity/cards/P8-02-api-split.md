# 卡片 P8-02：API 层按域拆分

| 调度头 | 值 |
| --- | --- |
| 波次 | **W12** |
| 前置依赖 | P6-04（契约稳定） |
| 独占文件 | `web/src/api/**`（client.ts + 各域文件） |
| 可与谁并发 | P8-01（realtime 目录不同） |
| 风险 | 🟠 中（60+ 处账号头收敛，回归面大） |

---

- 目标：把单文件 `api/index.ts` 演进为按域模块，收敛重复的账号头注入。
- 源（现状）：`api/index.ts`(77) + 各 store/组件直连 `api`；每调用重复传 `x-account-id`（60+ 处，与拦截器重复）。
- 目标（前端）：`web/src/api/`——`client.ts`(axios 实例+拦截器)、`account.ts`/`friend.ts`/`farm.ts`/`mall.ts`/`task.ts`/`activity.ts`/`user.ts`/`card.ts`/`yyb.ts`/`system.ts` 等按域文件。
- 实现要点：
  - 拦截器统一注入 `x-admin-token` + `x-account-id`（单一真相源），**各 store/组件不再手传账号头**。
  - 契约对齐 P6 handler；错误处理（401 跳登录、`账号未运行`/超时不弹 toast）保留。
  - **消除组件直连 api**：`AccountModal` 15 处、`Login` 4 处、`Dashboard` 2 处等收敛到对应 api 域模块 + store（拆分本身在 P8-03/04/05 完成，本卡先备好域模块）。
- 不要做：不改后端契约来迁就前端；不保留双份账号头来源。
- 验收：`pnpm build`+`lint`；抓包确认账号头仅注入一次；无 store 绕过 api 域模块。
- 完成判据：☐ 按域 API 模块 ☐ 账号头单源 ☐ store 不再手传 ☐ 契约对齐。
