# 00 · 并发调度总表（多 Codex 会话的大脑）

本文件是**给人看的调度依据**：你要多开 Codex 会话、每会话喂一张卡（`cards/Pn-xx-*.md`）并发推进时，
用这里的 ①依赖 DAG ②并发波次 ③**文件归属矩阵** 来决定「哪些卡能同时开、哪些必须排队」。

> 卡片总数：**52 张**（P0~P8 共 45 张迁移卡 + PC-01~07 共 7 张商用硬化卡）。
> 每张卡文件顶部都有一个「调度头」表格，字段与本文件一致，可交叉核对。

---

## 1. 怎么用（并发会话操作法）

1. **一会话一卡**：把某张 `cards/Pn-xx-*.md` 的全文投喂给一个 Codex 会话，让它只做这张卡。卡内「不要做」限定了边界，防止它越界改到别的卡的文件。
2. **开工前查两件事**：
   - **依赖**：该卡「前置依赖」列出的卡是否都已合并。未满足就别开（它会因缺接口而卡住或臆造）。
   - **文件冲突**：该卡「独占文件」与当前**在跑的其它会话**的独占文件是否有交集。有交集 = 不能同时跑（会 git 冲突 / 互相覆盖）。
3. **按波次批量开**：§3 的波次已经保证「同一波内的卡两两文件不相交且依赖已满足」，所以**同一波的卡可以全部并发**。做完一波、合并、跑一次 `make build && make test`，再开下一波。
4. **合并纪律**：每张卡完成即形成一个可编译提交（PR/分支）。**合并顺序 = 波次顺序**；同波内任意顺序合并。
5. **真机闸门**：遇到标 🔴 的卡（P2-02 / P2-05）必须先真机验证通过再推进后续波次（见 04 §6 go/no-go）。

### 冲突判定规则（黄金法则）
- 两张卡能并发 ⇔ **依赖都已满足** 且 **独占文件集合不相交**。
- 「独占文件」写的是该卡**会创建/修改**的文件或目录（`**` 表示整个子树）。
- **co-edit 标记**：少数卡会追加修改另一张卡创建的文件（如 P4-04 扩写 `runtime.go`、P6-05/P1-05 追加 `cmd/farmbot/`）。这类卡**不能与被它追加的那张卡同波**，必须排在其后单独进行；已在波次表中处理。
- PC-* 硬化卡一律**只新增文件**（如 `middleware/ratelimit.go`），刻意不改动 P6 已有文件，从而可与其它卡并发。

---

## 2. 里程碑视图（粗粒度）

| 里程碑 | 卡片 | 出口验收（真机/命令） |
| --- | --- | --- |
| **M1 地基** | P0-01..04 | `make build/lint/test` 成立；空服务 `/api/health` 200 |
| **M2 库 + 协议内核** | P1-*、P2-*、P3-01/02 | 🔴 TSDK 加解密对拍一致；WS 登录握手成功（真机） |
| **M3 运行时** | P3-03/04、P4-* | 单账号能登录+按调度挂机+重连（真机最小闭环） |
| **M4 领域** | P5-* | 各域与 Node 行为对拍；activity 七活动分文件可用 |
| **M5 HTTP/实时** | P6-*、PC-01/02/04/06 | 全 `/api/*` 契约快照与 Node 一致；实时三事件推送 |
| **M6 打包 + 前端 + 收尾** | P7-*、P8-*、PC-03/05/07 | `docker compose up` 单容器全功能；删除 Node |

关键路径（决定总工期）：**P0-01 → P2-02(WASM🔴) → P2-04 → P4-01 → P4-04 → P5-01 → P6-01 → P6-05 → P7-03(Docker)**。
其余卡都挂在这条链的旁支上，可大量并发吸收工期。**首选交付＝ Docker（P7-03）**；P7-02 跨平台二进制为可选旁支。

## 3. 并发波次表（同波可全并发）

> 每一波内的卡：依赖已被前面波次满足，且彼此独占文件不相交 → **可同时各开一个会话**。
> 「并发数」= 该波建议同时开的会话数。做完一波 → 合并 → `make build && make test` → 开下一波。

> 波次号为**权威**：每张卡「调度头」的波次必须与本表一致（已交叉核对 52 张卡）。
> P6-01→02→03 是线性依赖链（server→middleware→realtime），**不可同波**，故分列 W9/W10/W11。

| 波次 | 可并发卡片 | 并发数 | 前置(须已合并) | 备注 |
| --- | --- | --- | --- | --- |
| **W0** | P0-01 | 1 | — | 骨架必须最先，单独一波 |
| **W1** | P0-02, P0-03, P0-04 | 3 | P0-01 | 配置/日志/构建互不相干 |
| **W2** | P1-01, P2-01 | 2 | P0-* | 建库框架 + proto 生成并行 |
| **W3** | P1-02, P2-02🔴 | 2 | P1-01/P2-01 | schema + **WASM 移植(最高风险,尽早起)** |
| **W4** | P1-03, P1-04, P2-03, P2-04 | 4 | P1-02, P2-02 | 四个 repo/协议件并行；P2-04 依赖 P2-02 |
| **W5** | P1-05, P2-05🔴, P3-01 | 3 | P1-03/04, P2-04 | 导入器 + **登录会话真机闸门** + yyb 平移 |
| **W6** | P3-02, P3-03, P5-08 | 3 | P3-01, P1-02 | 微信账号入库 + yyb 门面 + 平台件(mailer 先于 P5-07) |
| **W7** | P4-01, P4-02 | 2 | P2-05, P3-03, P1-03 | Runtime 骨架 + 调度器 |
| **W8** | P4-03(含P3-04), P4-04 | 2 | P4-01/02 | Manager(并入 yyb 接线) + 循环编排(co-edit runtime.go,排 P4-01 后) |
| **W9** | P5-01, **P6-01** | 2 | P4-04 / P4-03 | warehouse 先行(被广泛复用) + 服务器骨架起步 |
| **W10** | P5-02, P5-04, P5-05, P5-07, **P6-02** | 5 | P5-01 / P6-01,P1-04 | farm/mall/task/轻量域 大并发 + 中间件 |
| **W11** | P5-03, P5-06, **P6-03** | 3 | P5-01/02 / P6-02,P4-04 | friend(依赖 farm) + activity 拆分 + 实时 Hub |
| **W12** | P6-04, P8-01, P8-02 | 3 | P6-02/03, 对应 P5-* | 领域 handler(每组依赖对应域) + 前端 realtime/api 起步 |
| **W13** | PC-01, PC-02, PC-04, PC-06, P7-02*, P8-03, P8-04, P8-05 | 8 | P6-04 契约冻结 | 硬化卡(只增文件) + 可选二进制 + 前端三大件拆分(依赖 P8-01/02) |
| **W14** | P6-05, P8-06 | 2 | P6-01~04,P3-03,P5-08 / P8-01~05 | 组合根(co-edit main.go,收口) + 前端收口 |
| **W15** | P7-01, P7-03, PC-03, PC-05, PC-07 | 5 | P6-05, P2-02 | 内嵌 + **Docker 镜像** + 备份/审计/健康排空(硬化) |
| **W16** | P7-04 | 1 | P6-04 全切+稳定运行一周期, P7-03 | **删除 Node(不可逆,全流程最末)** |

> **P7-03(Docker)在 W15**：它只需 P7-01(内嵌资产)+P6-05(组合根)就绪即可构建镜像，是商用首选交付物，尽早可跑。
> PC-07 的 compose 探针 co-edit 排在 P7-03 之后（同波内先合 P7-03 再合 PC-07，或 PC-07 顺延半波）。

前端 W12→W14 分三批（各卡独占不同组件/composable，可并发）：
- **批A（W12）**：P8-01(realtime/) + P8-02(api/) — 依赖 P6-03/P6-04 契约
- **批B（W13）**：P8-03(AccountModal) + P8-04(Login) + P8-05(Friends+Dashboard) ← 依赖批A
- **批C（W14）**：P8-06(类型/去重/store 瘦身) ← 依赖批B

> P7-02（跨平台二进制）为**可选**卡（W13 旁支），商用 Docker 交付不需要；不做也不影响主链。

---

## 4. 文件归属矩阵（防并发写冲突的权威表）

每张卡「独占」的文件/目录。**审核两张卡能否同波：看这里两行有无交集。**
（`**` = 整个子树；标 *co-edit* 的是「追加修改他人文件」，已在波次表隔离到被追加卡之后。）

| 卡 | 独占文件 / 目录 |
| --- | --- |
| P0-01 | `go.mod` `go.sum` `cmd/farmbot/main.go`(占位) `internal/**/doc.go` 目录骨架 |
| P0-02 | `internal/config/**` |
| P0-03 | `internal/platform/logger/**` |
| P0-04 | `Makefile` `.golangci.yml` |
| P1-01 | `internal/store/db.go` `internal/store/migrations/`(框架) |
| P1-02 | `internal/store/migrations/0001_init.sql` |
| P1-03 | `internal/store/{account_repo,cache_repo,config_repo,stats_repo}.go` |
| P1-04 | `internal/store/{user_repo,card_repo}.go` |
| P1-05 | `internal/store/migrate_json.go` + `cmd/farmbot/import.go`(*co-edit,新文件*) |
| P2-01 | `proto/**` `internal/game/pb/**` |
| P2-02🔴 | `internal/game/tsdk/**` `assets/wasm/**` `assets/embed.go`(wasm 部分) |
| P2-03 | `internal/game/ace/**` |
| P2-04 | `internal/game/transport/**` |
| P2-05🔴 | `internal/game/session/**` |
| P3-01 | `internal/yyb/protocol/**` `internal/yyb/qr/**` `internal/yyb/store.go`(初版) |
| P3-02 | `internal/store/migrations/0002_wechat.sql` `internal/yyb/store.go`(*co-edit,排 P3-01 后*) |
| P3-03 | `internal/yyb/service.go` |
| P3-04 | 逻辑并入 P4-03 的 `manager.go`（*本卡不独占文件，与 P4-03 同卡实现*） |
| P4-01 | `internal/account/runtime.go` `internal/account/status.go` |
| P4-02 | `internal/account/scheduler.go` |
| P4-03 | `internal/account/manager.go` |
| P4-04 | `runtime.go`/`status.go`(*co-edit,排 P4-01 后*) |
| P5-01 | `internal/domain/warehouse/**` |
| P5-02 | `internal/domain/farm/**` |
| P5-03 | `internal/domain/friend/**` |
| P5-04 | `internal/domain/mall/**` |
| P5-05 | `internal/domain/task/**` |
| P5-06 | `internal/domain/activity/**` |
| P5-07 | `internal/domain/{career,illustrated,social}/**` |
| P5-08 | `internal/license/**` `internal/platform/{mailer,pusher,machineid}/**` |
| P6-01 | `internal/httpapi/server.go` |
| P6-02 | `internal/httpapi/middleware/{cors,auth,session,account,timeout,role}.go` |
| P6-03 | `internal/httpapi/realtime/**` |
| P6-04 | `internal/httpapi/handlers/**` |
| P6-05 | `internal/app/**` `cmd/farmbot/main.go`(*co-edit,收口*) |
| P7-01 | `assets/embed.go`(web/gameConfig 部分) `internal/httpapi/webdist/`(若用) |
| P7-02(可选) | `Makefile`(release 目标,*co-edit P0-04 后*) |
| P7-03 | `Dockerfile` `docker-compose.yml` `.dockerignore` |
| P7-04 | 删除 `core/**` `yyb-go/**`；改 `README.md` 根 `package.json` |
| P8-01 | `web/src/realtime/**` `web/src/composables/useRealtime.ts`；改 `stores/status.ts` |
| P8-02 | `web/src/api/**` |
| P8-03 | `web/src/components/account/**` `composables/useAccountLogin.ts` |
| P8-04 | `web/src/views/Login.vue` `components/login/**` `composables/useAuthFlow.ts` |
| P8-05 | `web/src/views/{Friends,Dashboard}.vue` `components/{friends,dashboard}/**` |
| P8-06 | `web/src/{main.ts,App.vue}` `stores/{app,setting,user}.ts` `composables/useStaleGuard.ts` |
| PC-01 | `internal/httpapi/middleware/ratelimit.go`(新增) |
| PC-02 | `internal/httpapi/middleware/tenant.go`(新增) `internal/store/migrations/0003_tenant.sql` |
| PC-03 | `internal/platform/backup/**`(新增) `cmd/farmbot/backup.go` |
| PC-04 | `internal/platform/metrics/**`(新增) `internal/httpapi/handlers/metrics.go` |
| PC-05 | `internal/store/migrations/0004_audit.sql` `internal/httpapi/middleware/audit.go`(新增) |
| PC-06 | `internal/httpapi/middleware/secure_headers.go`(新增) |
| PC-07 | `internal/httpapi/handlers/health.go`(新增) `docker-compose.yml`(*co-edit P7-03 后*) |

> 冲突提示：P1-05/P6-05/P7-02/PC-07 都涉及别人的文件（`cmd/farmbot/`、`Makefile`、`docker-compose.yml`），
> 已在波次表把它们排到被依赖卡之后、且不与竞争者同波。其余卡都只独占自己的子树，放心并发。

---

## 5. 卡片文件命名与索引

拆分后每张卡是 `cards/` 下一个独立 md（文件名即卡号），一个会话喂一个文件：

```
docs/modularity/
├── 00-orchestration.md          ← 本文件(调度大脑)
├── 01-current-architecture.md   ← 现状(源清单)
├── 02-target-architecture.md    ← 目标架构
├── 03-directory-structure.md    ← 目录树
├── 04-migration-strategy.md     ← 绞杀策略/风险
├── 05-commercial-hardening.md   ← 商用硬化设计(PC-* 卡的总纲)
├── README.md                    ← 入口/读法
└── cards/                       ← 一卡一文件(投喂单元)
    ├── P0-01-go-module-scaffold.md ... P0-04-*.md
    ├── P1-01..05-*.md
    ├── P2-01..05-*.md
    ├── P3-01..04-*.md
    ├── P4-01..04-*.md
    ├── P5-01..08-*.md
    ├── P6-01..05-*.md
    ├── P7-01..04-*.md
    ├── P8-01..06-*.md
    └── PC-01..07-*.md
```

> 现存的 `05-cards-phase*.md`（分阶段合订本）在拆分后由 `cards/` 取代；README 会更新索引指向 `cards/`。

