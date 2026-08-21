# FarmBot 模块化与语言统一改造方案

本目录是把 FarmBot 从当前的 **Node(core) + Vue(web) + Go(yyb-go) 三体单体仓**，
整合为 **Go 后端 + TypeScript 前端**、`yyb-go` 作为后端内部功能模块、单一构建与部署产物的
完整改造方案。文档面向 **Codex/AI 执行**：每个模块给出可直接投喂的任务卡（动哪些文件、
拆成什么、目标 Go 包、接口契约、验收命令、依赖顺序、完成判据）。

## 目标（由维护者确定，2026-08-19）

1. **语言收敛为两种**：Go（后端）+ TypeScript（前端）。当前 Node.js/CommonJS 的 `core` 后端整体迁移到 Go。
2. **`yyb-go` 不再是独立项目**：其扫码 / `wxapp/getCode` 登录能力作为 Go 后端的**内部包**融入，
   消除「跨进程 HTTP 跳转 + 双份 Token 布线 + 单独部署」的历史痛点。
3. **前端保留并继续模块化**：Vue3 + Vite + TS 不重写，沿用「页面协调器 + components/composables」既有模式，
   只把与后端的 API / 实时通道对接到新的 Go 服务。
4. **单一部署产物**：一个 Go 二进制内嵌前端 `web/dist` + WASM + 静态游戏数据；源码 / 二进制 / Docker 三种方式都能一键起。

## 阅读顺序

| 文档 | 内容 | 主要读者 |
| --- | --- | --- |
| [01-current-architecture.md](01-current-architecture.md) | 现状：三体结构、进程模型、耦合痛点、「两个项目」问题、拓扑图 | 人 + AI（迁移前必读，源清单） |
| [02-target-architecture.md](02-target-architecture.md) | 目标：Go+TS 架构、六个关键决策（并发/持久化/协议/WASM/实时/yyb 收编）、目标拓扑 | 人 + AI |
| [03-directory-structure.md](03-directory-structure.md) | 目标仓库目录树（Go 惯例 cmd/ internal/ + web/），逐目录标注 | 人 + AI |
| [04-migration-strategy.md](04-migration-strategy.md) | 分阶段绞杀式（strangler）迁移、依赖 DAG、如何全程保持可运行、风险登记 | 人 + AI |
| [00-orchestration.md](00-orchestration.md) | **并发调度总表**（依赖 DAG + 波次 + 文件归属矩阵）——多会话并发的大脑 | 人 |
| [05-commercial-hardening.md](05-commercial-hardening.md) | 商用硬化设计（PC-* 卡总纲：限流/多租户/加密/备份/审计/指标/健康） | 人 + AI |
| [cards/](cards/) | **52 张任务卡**（一卡一文件，Codex 投喂单元） | AI |

## 任务卡索引

拆分后每张卡是 `cards/` 下一个独立文件（文件名即卡号），**一个会话喂一个文件**。
调度、依赖、能否并发一律以 [00-orchestration.md](00-orchestration.md) 为**权威**（每张卡顶部「调度头」与其交叉核对一致）。

| 阶段 | 卡片（`cards/`） | 覆盖模块 |
| --- | --- | --- |
| P0 地基 | `P0-01`~`P0-04` | Go 骨架、配置、日志、CI/lint |
| P1 持久化 | `P1-01`~`P1-05` | SQLite schema + repositories + JSON 导入器（替代 JSON 文件） |
| P2 协议内核 | `P2-01`~`P2-05` | protoc-gen-go、WS 传输、TSDK/ACE(wazero)、登录会话🔴 |
| P3 yyb 收编 | `P3-01`~`P3-04` | yyb-go → `internal/yyb` 内部包（消灭"两项目分开部署"） |
| P4 账号运行时 | `P4-01`~`P4-04` | goroutine-per-account、AccountManager、调度器、自动化循环 |
| P5 领域服务 | `P5-01`~`P5-08` | 仓库/农场/好友/商城/任务/活动(拆分)/轻量域/平台+许可 |
| P6 HTTP/实时 | `P6-01`~`P6-05` | Gin 服务器、中间件、实时 Hub、领域 handler、组合根 |
| P7 构建部署 | `P7-01`~`P7-04` | 内嵌打包、**单容器 Docker(首选)**、可选二进制、删除 Node |
| P8 前端对接 | `P8-01`~`P8-06` | API/实时适配 + 继续拆巨石页面 + 收窄类型 |
| PC 商用硬化 | `PC-01`~`PC-07` | 限流/多租户配额+凭据加密/备份导出/指标/审计/安全头/健康排空 |

## 任务卡统一模板

每张卡遵循以下结构，便于 Codex 逐卡执行：

```
### 卡片 Pn-xx：<标题>
- 目标：一句话
- 前置依赖：<卡片号>
- 源（Node/现状）：file:line 清单
- 目标（Go/TS）：目标包 / 文件路径
- 实现要点：要建的类型、接口、边界
- 不要做：明确排除项（防止 scope 蔓延）
- 验收：可执行的命令（go build / go test / eslint / vite build）
- 完成判据：可勾选的结果
```

## 全局约束（沿用 progress.md 既有铁律）

- **增量推进**：不做一次性大规模文件搬迁造成引用噪音；每卡自成可验证单元。
- **入口只做 wiring**：`cmd/farmbot/main.go` 只组装，逻辑下沉到 `internal/*` 领域包。
- **迁移期双跑可回退**：Go 新服务与 Node 旧服务在绞杀期通过反向代理并存，逐路由切换（见 04）。
- **中文按 UTF-8 真实内容为准**，终端乱码不作为改码依据。
