# 卡片 P7-03：Docker 单/双阶段镜像（首选部署方式）

| 调度头 | 值 |
| --- | --- |
| 波次 | **W15** |
| 前置依赖 | P7-01、P6-05 |
| 独占文件 | 根 `Dockerfile`、`docker-compose.yml`（重写） |
| 可与谁并发 | P7-02（Makefile 不同文件） |
| 风险 | 🟠 中（部署首选路径，需最先打通并长期维护） |

---

- 目标：用 2 阶段（web 构建 + go 构建）取代现状 3 阶段 + 双进程 shell 编排。**这是商用首选部署方式，优先级最高。**
- 源（现状）：`core/Dockerfile`(3 阶段)、`core/docker/start.sh`(tini + yyb `&` + `exec node`)、`docker-compose.yml`、`server-deploy.sh`。
- 目标（Go）：根 `Dockerfile`（2 阶段）+ 精简 `docker-compose.yml`。
- 实现要点：
  - 阶段 1 `node:20` 跑 `pnpm -C web build`；阶段 2 `golang:1.23` 拷入 `web/dist` 与源码 `go build`；最终 `FROM gcr.io/distroless/static` 或 `alpine` 拷单二进制。
  - **无 start.sh / 无 tini / 无 `&`**：单进程即入口（yyb 已在进程内），信号直达 Go 的优雅退出。
  - compose 单服务、单端口 3007、单数据卷（`data/farmbot.db` + 运行时目录）；健康检查探 `/api/health`。
  - 移除 `YYB_API_URL/KEY/TOKEN`/`YYB_PORT` 及其 auto-gen 逻辑（进程内直调不需要）。
  - **商用衔接**：镜像标签/版本策略、`docker compose` 的 `.env` 样例（管理员初始密码、license 开关、PC-03 计费开关）在 PC-06 补齐；本卡先把单进程镜像打通。
- 不要做：不保留双进程编排；不保留跨进程 Token 布线。
- 验收：`docker build` 成功；`docker compose up` 起单容器，前端可用、yyb 扫码可用、无外部 yyb 进程。
- 完成判据：☐ 2 阶段镜像 ☐ 单进程单端口 ☐ 单数据卷 ☐ yyb 内置。
