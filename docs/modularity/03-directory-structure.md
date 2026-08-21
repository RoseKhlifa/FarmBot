# 03 · 目标目录结构

遵循 Go 项目惯例（`cmd/` 入口、`internal/` 私有包、生成码提交、资源内嵌）。前端 `web/` 基本不动。
**迁移期**新旧并存：新增 `server/`（Go）而暂不删 `core/`（Node），绞杀完成后再移除 `core/`（见 04）。

## 1. 迁移完成后的最终树

```
FarmBot/
├── go.mod                          # module github.com/<owner>/farmbot  (合并原 yyb-go 的依赖)
├── go.sum
├── Makefile                        # build / build-web / gen-proto / test / lint / release
├── README.md
├── docker-compose.yml              # 单服务, 单端口, 无双进程编排
├── Dockerfile                      # 2 阶段: web(node build) + go build; 取代原 3 阶段
│
├── cmd/
│   └── farmbot/
│       └── main.go                 # 仅 wiring: 读配置 → 建 Application → 起 HTTP → 优雅退出
│
├── internal/                       # 全部后端私有包 (external 不可 import)
│   ├── app/
│   │   ├── application.go          # 组合根: Application{Config,Stores,AccountMgr,Realtime,Yyb,Auth}
│   │   └── wire.go                 # 依赖装配 (手写或 google/wire)
│   │
│   ├── config/
│   │   ├── config.go               # env + 文件 → Config 结构体 (沿用 ADMIN_PORT/FARM_DATA_DIR/YYB_*...)
│   │   └── paths.go                # 数据目录/资源目录解析 (等价 runtime-paths.js)
│   │
│   ├── store/                      # 持久化 (替代 models/store.js + user-store.js + stats.js + 磁盘缓存)
│   │   ├── db.go                   # SQLite 打开/WAL/迁移执行
│   │   ├── migrations/             # 版本化 SQL: 0001_init.sql ...
│   │   ├── queries/                # sqlc 的 .sql (若用 sqlc)
│   │   ├── gen/                    # sqlc 生成码
│   │   ├── account_repo.go         # 账号 + per-account 配置/策略/间隔
│   │   ├── user_repo.go            # 面板用户 + PBKDF2 + 登录限流
│   │   ├── card_repo.go            # 卡密授权系统
│   │   ├── cache_repo.go           # 好友 GID/狗信息/列表 + 黑名单
│   │   ├── stats_repo.go           # per-account 统计
│   │   ├── config_repo.go          # 全局/系统/wx/主题/防倒卖配置
│   │   └── migrate_json.go         # 一次性 core/data/*.json → SQLite 导入器
│   │
│   ├── game/                       # 游戏协议内核 (替代 utils/network,proto,tsdk-runtime,crypto-wasm)
│   │   ├── pb/                     # protoc-gen-go 生成: plantpb.pb.go, friendpb.pb.go ... (22 个)
│   │   ├── transport/
│   │   │   ├── ws.go               # 单连接 WS 客户端 (等价 network.js 的 sendMsgAsync/心跳/重连)
│   │   │   └── dispatch.go         # 入站 notify 分发 (等价 handleNotify)
│   │   ├── tsdk/
│   │   │   ├── runtime.go          # wazero 加载 tsdk-v3.8.2.wasm + 12 导出封装
│   │   │   ├── imports.go          # 22 个宿主 import a.a–a.v 的 Go 实现
│   │   │   └── verify.go           # SHA-256 + 导出表校验
│   │   ├── ace/
│   │   │   └── service.go          # ACE 生命周期节拍 5/25/30/150/180s + AntiData 上报
│   │   └── session/
│   │       └── login.go            # 登录会话 (SdkInitEx/AnoUserLogin/网关 Token)
│   │
│   ├── account/                    # 账号运行时 (替代 core/worker.js + runtime/*)
│   │   ├── manager.go              # AccountManager: map[id]*Runtime + start/stop/restart/重连退避
│   │   ├── runtime.go              # Runtime: 持 WS+UserState+TSDK+调度器, 跑自动化循环
│   │   ├── scheduler.go            # 命名调度器 (替代 services/scheduler.js, 降为实例字段)
│   │   └── status.go               # per-account 运行态快照 (替代 status.js 单例)
│   │
│   ├── domain/                     # 领域服务 (替代 services/*, 全部接口化, 依赖 game + store 接口)
│   │   ├── farm/                   # farm-api + land-analyzer + fertilizer + planting + orchestrator
│   │   ├── friend/                 # friend-api + operation-limits + land-analyzer + visit + orchestrator + golden-bug
│   │   ├── warehouse/              # 背包/出售/使用
│   │   ├── mall/                   # 商城 + 神秘商店 + 月卡 + qqvip
│   │   ├── task/                   # 任务/每日
│   │   ├── activity/               # 拆分后: nangua.go helu.go qingmei.go guanxing.go season.go solarterms.go
│   │   │   ├── decode.go           # 共享 protobuf 线格式扫描器 (原 activity.js 一半)
│   │   │   └── core.go             # getActivityGroup / operateActivity 共享 RPC
│   │   ├── career/                 # 个人生涯
│   │   ├── illustrated/            # 图鉴
│   │   └── social/                 # share + invite + interact + dog-gifts
│   │
│   ├── yyb/                        # ← 原 yyb-go 收编 (内部包, 同进程直调)
│   │   ├── service.go              # Service.GetCode/QRCreate/QRPoll/QRConfirm/ListAccounts (Go 接口, 无 HTTP)
│   │   ├── protocol/               # 平移 yyb-go/internal/protocol (MMTLS/ilink/shortlink...)
│   │   ├── qr/                     # 平移 yyb-go/internal/qr
│   │   └── store.go                # 微信账号表 (并入主 SQLite 或独立表)
│   │
│   ├── httpapi/                    # HTTP 层 (替代 controllers/admin*.js 全部)
│   │   ├── server.go               # Gin 引擎组装 + 静态托管 web/dist + SPA fallback + 优雅退出
│   │   ├── middleware/
│   │   │   ├── auth.go             # x-admin-token 校验 + PUBLIC 路径 allowlist
│   │   │   ├── session.go          # 会话管理 (可选持久化到 SQLite, 修复重启失效)
│   │   │   ├── account.go          # x-account-id 解析 + 访问控制
│   │   │   ├── cors.go  timeout.go # CORS / 请求超时守卫
│   │   │   └── role.go             # admin / super-admin 角色门
│   │   ├── realtime/
│   │   │   └── hub.go              # WS/SSE Hub: 按账号订阅, 广播 status/log/account-log
│   │   └── handlers/               # 领域 handler (一域一文件, 对应原 40+ admin-*-routes.js)
│   │       ├── auth.go account.go friend.go farm.go bag.go mall.go
│   │       ├── task.go activity.go illustrated.go career.go analytics.go
│   │       ├── settings.go system.go user.go card.go login_log.go
│   │       ├── yyb.go              # 直调 internal/yyb.Service, 无代理
│   │       ├── capture.go qr_login.go proxy.go public_info.go
│   │       └── shop_seed.go shop_pet.go shop_decoration.go shop_mystery.go
│   │
│   ├── license/
│   │   └── license.go              # 机器码绑定授权 (等价 services/license.js, FARM_LICENSE_ENABLED)
│   │
│   └── platform/                   # 跨领域基础设施
│       ├── logger/                 # slog + secret 脱敏 (等价 logger.js)
│       ├── mailer/                 # SMTP (等价 email.js)
│       ├── pusher/                 # 推送 (等价 push.js)
│       └── machineid/              # 机器指纹 (等价 machine-id.js)
│
├── proto/                          # 22 个 .proto 源 (从 core/src/proto 平移)
│   └── *.proto
│
├── assets/                         # 内嵌资源 (go:embed)
│   ├── wasm/                       # tsdk-v3.8.2.wasm, tsdk-legacy.wasm, tsdk.wasm
│   ├── gameConfig/                 # Plant.json/RoleLevel.json/种子图 (从 core/src/gameConfig 平移)
│   └── embed.go                    # //go:embed 声明 + 访问函数
│
├── web/                            # 前端 (保留, 只做 API/实时适配 + 继续拆分, 见 P8)
│   ├── src/
│   │   ├── api/                    # 由单文件 index.ts 演进为按域模块 (P8)
│   │   ├── realtime/               # 新增: WS/SSE 客户端封装 (替代 socket.io-client)
│   │   ├── stores/ composables/ components/ views/ layouts/ router/
│   │   └── ...
│   └── dist/                       # 构建产物 → 被 Go embed
│
├── scripts/                        # extract_seed_icons.py 等
├── docs/
│   └── modularity/                 # 本方案
│       ├── README.md 01~04 *.md
│       └── cards/  (若拆分至独立目录)
│
└── (迁移完成后删除) core/           # 旧 Node 后端, 绞杀期保留供对照/回退
```

## 2. 与现状的目录映射速查

| 现状 (Node) | 目标 (Go) |
| --- | --- |
| `core/client.js` | `cmd/farmbot/main.go` |
| `core/src/controllers/admin.js` + `admin-*-routes.js` | `internal/httpapi/{server,middleware,handlers}` |
| `core/src/runtime/{engine,state,worker-manager,data-provider}.js` | `internal/account/{manager,runtime,scheduler}` + `internal/app` |
| `core/src/core/worker.js` | `internal/account/runtime.go` + 各 `internal/domain/*` |
| `core/src/models/store.js` | `internal/store/{account,cache,config,stats}_repo.go` |
| `core/src/models/user-store.js` | `internal/store/{user,card}_repo.go` |
| `core/src/services/*.js` | `internal/domain/*` |
| `core/src/services/activity.js`(2300) | `internal/domain/activity/{一活动一文件}` |
| `core/src/utils/{network,proto,tsdk-runtime,crypto-wasm}.js` | `internal/game/{transport,pb,tsdk,ace,session}` |
| `core/src/services/{logger,email,push}.js` `utils/machine-id.js` | `internal/platform/*` |
| `core/src/config/*.js` | `internal/config/*` |
| `core/src/proto/*.proto` | `proto/*.proto` → `internal/game/pb/*.pb.go` |
| `core/src/gameConfig/`, `utils/*.wasm` | `assets/{gameConfig,wasm}` (go:embed) |
| `yyb-go/internal/*` | `internal/yyb/*` |
| `core/src/controllers/admin-yyb-routes.js` | `internal/httpapi/handlers/yyb.go` (直调, 无代理) |
| `core/data/*.json` | `data/farmbot.db` (SQLite) + 一次性导入器 |
```
