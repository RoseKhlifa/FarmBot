# 卡片 PC-04：指标与监控

| 调度头 | 值 |
| --- | --- |
| 波次 | **W13** |
| 前置依赖 | P6-02 |
| 独占文件 | `internal/httpapi/metrics/**` + 路由 `/metrics` |
| 可与谁并发 | 全部 PC-* + P6-04（只新增文件） |
| 风险 | 🟡 低 |

---

- 目标：可观测——在线账号、操作计数、WS 连接、登录成败、TSDK 心跳、HTTP 时延。
- 源（现状缺口）：仅日志；无指标/健康细分。
- 目标（Go）：`internal/httpapi/metrics/` + `/metrics` 路由。
- 实现要点：
  - `/metrics`（Prometheus 文本格式，或轻量 JSON）：在线账号数、每账号操作计数、WS 连接数、登录成功/失败、TSDK 心跳失败数、HTTP 时延分位。
  - 指标端点走**独立鉴权或仅内网可达**（不暴露给普通面板用户）。
  - 指标采集点：AccountManager（在线数）、stats（操作计数）、realtime hub（WS 连接）、auth（登录成败）、ace/tsdk（心跳失败）。
- 不要做：不把指标暴露给普通用户 token；不在热路径做重计算。
- 验收：`/metrics` 返回可被 Prometheus 抓取的文本；关键指标随行为变化。
- 完成判据：☐ 核心指标齐全 ☐ 端点受保护 ☐ 采集点接入。
