# 04 · 迁移策略（分阶段绞杀 · 全程可运行）

## 1. 总原则

- **绞杀式（Strangler Fig）**：不停机、不大爆炸。Go 新服务逐能力接管，Node 旧服务同时在跑，
  用一个反向代理按路由把流量从 Node 切到 Go，直到 Node 无流量后整体移除。
- **每卡自成可验证单元**：每张任务卡结束时仓库都能编译 / 构建 / 通过既有验收，绝不留半成品主干。
- **契约优先**：先固定 REST/实时/DB 契约（02 §5），再各自实现，前后端与新旧后端解耦推进。
- **协议内核先行**：`internal/game`（WS+protobuf+TSDK/ACE）是所有领域的地基，必须最先打通并用真实账号验证，
  否则上层全是空中楼阁。

## 2. 阶段依赖 DAG

```
P0 地基(scaffold/config/logger/CI)
      │
      ├────────────► P1 持久化(SQLite + repos + JSON导入器)
      │                     │
      ▼                     │
P2 协议内核                  │
  ├ P2-1 proto 生成 ◄────────┤ (领域/账号都要用)
  ├ P2-2 WS 传输             │
  ├ P2-3 TSDK/ACE (wazero)   │   ← 最高风险, 尽早真机验证
  └ P2-4 登录会话            │
      │                     │
      ▼                     ▼
P3 yyb 收编 ◄──────── (登录 code 供账号启动用)
      │
      ▼
P4 账号运行时(Manager + Runtime + 调度器)  ◄── 依赖 P1,P2,P3
      │
      ▼
P5 领域服务(farm/friend/warehouse/mall/task/activity...)  ◄── 依赖 P2,P4
      │
      ▼
P6 HTTP/实时(Gin 路由 + 中间件 + handler + Hub)  ◄── 依赖 P4,P5
      │
      ├────────────► P7 构建部署(embed/Docker/release)
      │
      ▼
P8 前端对接(API/实时适配 + 继续拆巨石 + 收窄类型)  ◄── 依赖 P6 契约
```

关键路径：**P0 → P2-3(WASM) → P4 → P5 → P6**。P1 可与 P2 并行；P8 可在 P6 契约冻结后并行。

## 3. 绞杀期如何并存（务必先搭好再迁）

迁移中间态用一层路由分流，让「已迁到 Go 的路由走 Go，其余仍走 Node」：

```
浏览器 → 反向代理(Caddy/Nginx 或 Go 自身前置)
           ├─ 已迁移路由 (如 /api/status, /api/yyb/*)  → Go :3007
           └─ 未迁移路由                                → Node :3008
```

- 起步：Node 仍监听原端口对外；Go 起在旁路端口，代理按 allowlist 把已完成路由指向 Go。
- 每完成一组 P5/P6 handler，就把对应路由前缀加入 Go 分流表。
- **数据一致性**：绞杀期两端都读同一份数据源。推荐 P1 先行——Node 与 Go **同时读写同一 SQLite**
  （Node 侧加一个薄 SQLite 适配层临时替换 JSON 读写），避免「JSON 与 SQLite 双写不一致」。
  若不想改 Node，则退化方案：Go 只接管**只读 + 幂等**路由，写路由最后一批切。
- 收尾：当分流表覆盖全部路由且真机稳定运行一个周期后，删除 Node `core/` 与代理层，Go 直接对外。

## 4. 风险登记（按严重度）

| 风险 | 级别 | 说明 | 缓解 |
| --- | --- | --- | --- |
| WASM 安全运行时移植 | 🔴 极高 | TSDK/ACE 是反作弊命脉，行为错误 = 全量封号/掉线 | 用 wazero 加载**同一 wasm**不重写算法；严格照 `tsdk-ace-runtime.md` 的 import/export 表；SHA256+导出校验；**先离线单测再单账号真机验证 ≥5/30min 好友操作**，通过才推广 |
| WS 帧/心跳时序 | 🔴 高 | 心跳/退避/kickout 时序错会掉线或被踢 | 逐字段对照 `network.js`；保留 5s/25s/30s/150s/180s 节拍常量；用抓包回放测试 |
| 好友操作限额/风控 | 🟠 中高 | 每日 steal/help/bad 限额与节流逻辑复杂 | `friend-operation-limits.js` 逐规则搬运 + 单测覆盖边界 |
| SQLite 并发写 | 🟠 中 | 现状无锁, Go 高并发写需正确事务 | WAL + 每 repo 方法用短事务；账号级串行化写 |
| activity.js 手写解码器 | 🟠 中 | 一半是脆弱的 protobuf 线格式扫描 | 优先尝试用 proto 定义正式解码；无 schema 的字段再移植扫描器 + 单测 |
| API/实时契约漂移 | 🟡 中 | 前端依赖既有响应形状 | 02 §5 冻结契约；P8 前建契约快照测试逐路由比对 Node vs Go |
| 卡密/授权行为 | 🟡 中 | 涉及计费闭环, 错误影响用户可用性 | `user-store.js` 卡密逻辑 + login 审计逐条搬 + 单测 |
| 数据迁移丢失 | 🟡 中 | 老用户 JSON → SQLite | 导入器只读旧文件不删；导入后校验计数；保留 JSON 备份 |
| 前端实时退化 | 🟢 低 | 现 socket 仅 Dashboard 建连 | P8 顺手修：提到 app 级 composable |

## 5. 每阶段的「完成即验收」基线

- **P0**：`go build ./...` 通过；`main.go` 能起空 HTTP 返回 `/api/health` 200。
- **P1**：`go test ./internal/store/...` 通过；导入器能把样例 JSON 灌入 SQLite 并校验计数。
- **P2**：离线单测 TSDK 加解密/Token 与 Node 输出逐字节一致；WS 能连上网关并完成登录握手；**单账号真机跑通 AllLands + 一次好友操作**。
- **P3**：Go 内 `yyb.Service.GetCode` 返回可用 code；扫码登录闭环在 Go 内完成，无外部 yyb 进程。
- **P4**：能 start/stop 单账号 Runtime，自动化循环运行，重连退避生效。
- **P5**：各领域方法与 Node 行为对拍（同输入同结果）；activity 六活动分别可读可操作。
- **P6**：全部 `/api/*` 路由在 Go 可用，契约快照与 Node 一致；实时三事件正常推送。
- **P7**：单 `go build` 出跨平台二进制（内嵌前端+wasm）；`docker build` 单阶段镜像可跑；三种部署都自带 yyb。
- **P8**：`vite build` 通过；前端指向 Go 后端全功能可用；巨石页面按批拆分；socket 生命周期修复。

## 6. 建议节奏

1. **先做 P0 + P1 + P2-1/2-3**（地基 + 库 + 协议内核最高风险项）并真机验证 WASM——这是 go/no-go 关口。
2. WASM 验证通过后，P2-2/2-4 → P3 → P4 打通「能登录、能挂机」的最小闭环（哪怕只有 farm 一个领域）。
3. 用绞杀代理把 `/api/status` 等只读路由先切 Go，验证前端联调链路。
4. P5 领域逐个搬（farm→friend→warehouse→mall→task→activity→其余），每域搬完切对应路由。
5. P6 补齐 admin/settings/user/card 等管理路由 → P7 打包 → 删除 Node → P8 收尾前端。

> go/no-go：若 P2-3 真机验证失败且无法在合理时间内解决，则回退到「Go 主体 + 通过 CGO/子进程调用一段保留的 Node/WASM 桥」的折中，但**不**放弃语言统一目标——把桥限制在 TSDK 单点。
