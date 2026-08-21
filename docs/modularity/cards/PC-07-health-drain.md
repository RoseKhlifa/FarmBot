# 卡片 PC-07：健康分级与灰度

| 调度头 | 值 |
| --- | --- |
| 波次 | **W15**（最终硬化，依赖优雅退出 + Docker 就绪） |
| 前置依赖 | P6-01（优雅退出）、P6-05、P7-03 |
| 独占文件 | `internal/httpapi/handlers/health.go`（+ compose 探针追加） |
| 可与谁并发 | 其它 W16（本卡为收尾，通常单独做） |
| 风险 | 🟡 低中 |

---

- 目标：存活/就绪分离 + 优雅排空。
- 源（现状）：单 `/api/health`；无就绪/存活分离、无优雅排空。
- 目标（Go）：`internal/httpapi/handlers/health.go` + compose 探针分别指向。
- 实现要点：
  - `/api/health`（存活，快，仅表明进程活着）与 `/api/ready`（就绪：DB 可连、迁移已到位、必要资源已加载）分离；compose/K8s 探针分别指向。
  - **优雅排空**：收到 SIGTERM 先停止接新请求与新账号启动，等在途完成再退出（配合 P6-01 优雅退出）。
  - 排空期间 `/api/ready` 返回不就绪，`/api/health` 仍存活，让 LB 摘流但不杀进程。
- 不要做：不让就绪检查做重量级探测（快而准）；不在排空期立即硬杀在途任务。
- 验收：两端点语义正确；SIGTERM 触发排空、在途完成后退出；compose 探针指向 `/api/health`。
- 完成判据：☐ health/ready 分离 ☐ 优雅排空 ☐ 排空期就绪翻转 ☐ compose 探针更新。
