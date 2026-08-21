# 卡片 P6-05：Application 组合根

| 调度头 | 值 |
| --- | --- |
| 波次 | **W14**（P6 收口，依赖前四张 + 关键领域就位） |
| 前置依赖 | P6-01~04、P3-03、P5-08 |
| 独占文件 | `internal/app/application.go` `internal/app/wire.go` `cmd/farmbot/main.go` |
| 可与谁并发 | 无（收口卡，需最新各模块接口） |
| 风险 | 🟠 中 |

---

- 目标：把所有依赖装配集中到一个组合根，`main.go` 只调用它。
- 源（现状）：`runtime-engine.js`(组装) + `client.js`(顶层 wiring)。
- 目标（Go）：`internal/app/application.go` + `internal/app/wire.go`。
- 实现要点：
  - `Application` 持有并按依赖顺序构造：Config → Stores(repos) → game 工厂 → yyb.Service → AccountManager → 领域服务工厂 → Realtime Hub → Auth/Session → HTTP Server。
  - 领域服务与 Runtime 的注入在此定义（谁依赖谁一目了然，替代现状散落的回调注入）。
  - `cmd/farmbot/main.go` 仅：读 config → `app.New(cfg)` → `app.Run(ctx)` → 等待信号 → `app.Shutdown()`。
- 不要做：不在 main.go 放逻辑；不用包级 init 做隐式装配。
- 验收：`go build`；`go run ./cmd/farmbot` 全链路起服务并可优雅退出。
- 完成判据：☐ 组合根集中装配 ☐ main 仅 wiring ☐ 全链路可起可停。

> **P6 出口里程碑**：Go 单进程可独立起服务、托管前端、跑通鉴权与实时推送、至少一组领域路由端到端可用。绞杀代理可开始把流量按域切向 Go。
