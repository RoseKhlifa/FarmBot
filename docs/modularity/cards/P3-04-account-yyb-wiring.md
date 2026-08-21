# 卡片 P3-04：账号启动接入 yyb（替代 refreshYybCodeIfNeeded）

| 调度头 | 值 |
| --- | --- |
| 波次 | **W8**（与 P4-03 同卡实现，见备注） |
| 前置依赖 | P3-03、P2-05 |
| 独占文件 | 无独占；逻辑并入 P4-03 的 `internal/account/manager.go` |
| 可与谁并发 | 作为 P4-03 的一部分实现，不单独开会话 |
| 风险 | 🟡 中 |

---

- 目标：账号启动/重连前刷新登录 code 改为直调 yyb Service。
- 源（现状）：`worker-manager.js:162 refreshYybCodeIfNeeded`（内置账号走 globalWxConfig，thirdparty 走第三方 API）。
- 目标（Go）：在 P4 `account/manager.go` 启动流程调用 `yyb.Service.GetCode(...)` 得到 code 再交 `session.Login`。
- 实现要点：
  - 内置账号：直调 `yyb.Service.GetCode(openid, appID)`。
  - 第三方 provider：保留一个 `ThirdPartyProvider` 接口（对齐 `utils/thirdpartyYyb.js`），可插拔；默认走内置 yyb。
- 不要做：不保留 `YYB_API_URL/KEY` 环境布线（内部直调）。
- 验收：账号启动流程单测用 mock yyb Service 返回 code；真机在 P4 出口联调。
- 完成判据：☐ 启动/重连直调 yyb ☐ 第三方 provider 可插拔 ☐ env 布线移除。

> 调度说明：本卡不新建独占文件，实现落在 P4-03 的 `manager.go` 里。投喂时可与 P4-03 合并为同一会话，或在 P4-03 完成后作为紧接的小补丁。
