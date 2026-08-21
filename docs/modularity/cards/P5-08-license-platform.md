# 卡片 P5-08：license 与 platform 基础设施

| 调度头 | 值 |
| --- | --- |
| 波次 | **W6**（早于其余 P5，mailer 先就位供 P5-07 引用） |
| 前置依赖 | P0-03、P1-01 |
| 独占文件 | `internal/license/**` `internal/platform/{mailer,pusher,machineid}/**` |
| 可与谁并发 | P3-02, P3-03（不同目录） |
| 风险 | 🟡 中 |

---

- 目标：搬迁授权与跨领域基础设施。
- 源（现状）：`services/license.js`(213, 机器码绑定 AES-256-CBC+MD5, `FARM_LICENSE_ENABLED`)、`push.js`、`email.js`、`utils/machine-id.js`、`account-resolver.js`。
- 目标（Go）：`internal/license/license.go`、`internal/platform/{mailer,pusher,machineid}/`。
- 实现要点：
  - license 校验保持算法一致（机器码派生 + 加密校验）；`FARM_LICENSE_ENABLED=false` 默认关闭（对齐现状 client.js:22 gate）。
  - `account-resolver` 的引用解析（id/name/ref → accountID）改为 store 层或 handler 层的小工具函数。
  - **商用衔接**：本卡的 license 是「单机机器码授权」；商用的多租户订阅/计费在 PC-03 另做，二者可共存（license 管部署实例，PC-03 管终端用户订阅）。
- 不要做：不改授权加密方案；不弱化默认关闭行为。
- 验收：`go test`——license 启用/关闭分支；机器码稳定性。
- 完成判据：☐ license 对齐 ☐ mailer/pusher/machineid 就位。
