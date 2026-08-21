# 卡片 PC-02：多租户、配额与凭据加密

| 调度头 | 值 |
| --- | --- |
| 波次 | **W13** |
| 前置依赖 | P3-02（stores/repo）、P5-08（yyb.Service 持久化） |
| 独占文件 | `internal/tenant/**`、`internal/crypto/secretbox.go`（+ repo 加解密挂钩点） |
| 可与谁并发 | 其它 PC-*；与 P3-02 错峰（本卡加解密需挂在 repo 写入处） |
| 风险 | 🟠 中（触及凭据存储，务必先加密再上线） |

---

- 目标：把工具变成可分套餐、可控配额、凭据静态加密的多租户服务。
- 源（现状缺口）：user→account 有归属，但无租户配额/套餐/隔离层；`login_buffer`/openid/credentials/sessions **明文入库**。
- 目标（Go）：`internal/tenant/`（plan/quota 校验）+ `internal/crypto/secretbox.go`（AES-256-GCM 封装）。
- 实现要点：
  - **租户模型**：在 `users`/`accounts` 之上增加 `plan`（账号数上限、并发挂机数、功能开关）与 `tenant_id`（经销商/团队，预留分片键）。中间件在账号启动/新增时校验配额，超限拒绝。
  - **凭据静态加密**：`wechat_accounts.login_buffer`、`credentials`、`sessions` blob 用主密钥（env `FARM_MASTER_KEY`，AES-256-GCM）加密后入库；密钥不入库、不入日志。DB 泄露 ≠ 凭据泄露。加解密挂在 repo 存取层（P3-02 / P5-08 的写入点）。
  - **数据最小化**：不需要的用户信息不存；头像等可再生数据可清理。
  - 兼容迁移：读到明文旧值时按需加密回写（一次性升级路径）。
- 不要做：不把主密钥写进 DB/镜像/git；不在日志打印明文凭据。
- 验收：`go test`——配额超限拒绝、加密往返一致、密钥缺失时安全失败（拒启动而非降级明文）。
- 完成判据：☐ plan/quota 校验 ☐ tenant_id 预留 ☐ 凭据 AES-GCM 加密 ☐ 明文迁移路径 ☐ 密钥仅经 env。
