# 项目目标

- 主工作区：`D:\github\qq-farm-2.3.x-rebuild`。
- 旧版参考项目：`D:\github\qq-farm-2.3.1`。查旧功能、接口行为、文案和流程时先对照它，但不要机械复刻旧 UI。
- 目标是把 QQ 农场自动化工具整理成前后端更易维护、容易排查、方便扩展的结构。
- 重构方向：统一目录和模块边界，减少冗余和历史代码味道，降低阅读/修改成本，逐步解耦架构与依赖。
- 非目标：不做无关大改，不一次性全站格式化，不把“复刻 2.3.1 UI”当成目标。

# 当前状态

- TSDK/ACE 安全链路已按 `任务.md` 落地到 `codex/tsdk-ace-runtime`：保留旧 WASM 为 `tsdk-legacy.wasm`，新增官方 `v3.8.2.1783066265` 版本化 WASM、Node 宿主运行时、ACE Proto/上报服务、动态网关 Token、账号级启停与重连清理。运行时会校验 SHA-256 和导出表，处理官方 mergewasm 数据段解密；失败时显式中止，不再回退伪 Token。调用映射和内存所有权见 `core/docs/tsdk-ace-runtime.md`。离线测试 4/4、触碰文件 ESLint、Proto 加载及后端 require 均通过；受控在线 5/30 分钟好友操作仍需测试账号实测。
- 技术栈：后端 `core` 是 Node.js/CommonJS + Express + Socket.IO；前端 `web` 是 Vue 3 + Vite + TypeScript + Pinia + UnoCSS。
- 最新快速体检结果：`web/src` 全量 ESLint 通过，`web` 生产构建通过；`core/src/**/*.js` 全量 `node --check` 通过。源码扫描未发现真实替换字符类乱码、孤立 `undefined` 行或 `_v###` 反编译变量残留；`core` ESLint 因本地 `core/node_modules` 缺少 `@antfu/eslint-config` 未作为源码失败处理。
- UTF-8 源码扫描未发现 `core/src`、`web/src` 存在真实替换字符类乱码；PowerShell 仍可能把中文显示成乱码，不能据此改源码。
- 已完成第一批低风险前端规范清理：背包空态分支、主题读取空块、确认框无意义绑定、微信扫码调试输出、静态正则、`Friends.vue` 定义顺序、`Login.vue` 换行格式。
- `Renewal.vue` 已修复登录态入口与续费分支：`/renewal` 不再因有效 token 被路由守卫强制跳回 dashboard；已登录用户进入续费页会预填并锁定当前用户名，提交走 `/api/user/renew` 并同步本地用户信息；未登录用户仍走 `/api/public/renew`。旧版 `D:\github\qq-farm-2.3.1` 仅作为接口行为参考，未照搬产物。
- 登录页“账号续费”闭环已补齐：`Login.vue` 会把当前输入用户名带到 `/renewal`，`Renewal.vue` 公共续费成功后带用户名回 `/login`，登录页会在输入框为空时从 query 回填用户名；已登录用户续费仍走当前账号锁定流程。
- `Login.vue` 已开始结构瘦身：卡密领取结果弹窗、找回密码验证弹窗和设置新密码弹窗已抽到 `web/src/components/login/LoginModals.vue`，父页面保留登录/注册/找回密码请求与状态 wiring，行为不变。
- 登录密码强度逻辑已抽离：评分规则在 `web/src/composables/usePasswordStrength.ts`，展示条在 `web/src/components/login/PasswordStrengthMeter.vue`，登录注册表单和重置密码弹窗复用同一套规则与 UI。
- `Friends.vue` 的 QQ 好友自动同步设置块已抽到 `web/src/components/friends/FriendsSyncSettings.vue`；父页面保留设置保存/刷新/GID 弹窗状态，组件负责展示设置项、统计提示和入口按钮。
- `Friends.vue` 的好友列表卡片与分页已抽到 `web/src/components/friends/FriendsFriendList.vue`；父页面继续持有好友筛选、展开状态、操作确认、黑名单和头像错误状态。
- 后台连接堆积导致网站打不开的问题已做防护：`core/src/controllers/admin.js` 增加 HTTP request/header/keep-alive/idle socket 超时、连接 close 清理、JSON 请求体大小限制和 `/api/health`；`core/src/models/user-store.js` 将 IP 登录失败限制加硬为 1 分钟 6 次后锁 10 分钟，并在登录成功后清理当前 IP 失败计数。
- 页面切换短暂显示“未登录/账号未登录”的问题已修复：`status` store 新增当前账号状态就绪标记，`Dashboard.vue`、`Friends.vue`、`FarmPanel.vue`、`BagPanel.vue`、`TaskPanel.vue` 只在确认当前账号离线后显示离线空态，并避免组件首次挂载时清空已有账号状态。
- `Dashboard.vue` 已去掉顶部重复状态卡、日志说明条、动作说明条和中间重复摘要卡；保留账号资源卡、运行日志、筛选、倒计时和今日统计，仪表盘信息密度更低、干扰更少。
- `Shop.vue` 已把商城标题、账号资源、分区切换、排序和刷新收敛到一条稳定横栏；删除无引用的 `ShopOverviewPanel.vue`、`ShopInfoCard.vue`，去掉分区概览统计卡和各分区商品列表前的说明条，购买流程保持不变。
- `Illustrated.vue` 已把图鉴标题、解锁/未解锁/可购买/Lv 状态、图鉴类型切换、筛选、一键购买和刷新收敛到一条稳定横栏；去掉“当前查看建议、当前阻塞汇总、当前筛选下”等解释型摘要和重复统计卡，图鉴卡片和购买流程保持不变。
- 图鉴卡片已抽到 `web/src/components/illustrated/IllustratedItemCard.vue`，卡片密度、圆角、网格列数、图片区域、状态胶囊和底部按钮区已对齐商城商品卡；图鉴图片增加加载失败兜底。
- `Activity.vue` 已按商城/图鉴同款结构收紧：活动中心标题、荷露余额、今日剩余、奖池、兑换数量、当前账号、抽奖/兑换切换和刷新统一到顶部横栏；删除无引用的 `ActivityNavigation.vue`、`ActivityOverview.vue` 和旧活动卡类型，荷露抽奖/兑换流程保持不变；活动奖励图片增加加载失败兜底，避免显示破损图片。
- 活动中心已接入本期 4 个子活动入口：奇遇礼莲、荷露商店、荷风游记、节令小札；后端 `normalizeHeluGroup` 会从活动树中标准化子活动节点参数（id/title/status/time/payload），前端只展示这 4 个入口，不恢复已过期的粽香大比拼。奇遇礼莲/荷露商店继续绑定已验证抽奖/兑换操作，荷风游记/节令小札先展示活动节点参数，等确认协议操作字段后再补交互。
- `Friends.vue` 已去掉好友页顶部说明、好友总数/黑名单/最近访客/已知 GID 统计卡，以及“当前分区说明/当前局部结论”摘要组件；删除无引用的 `FriendsStatsGrid.vue`、`FriendsSummaryCards.vue`，保留搜索、tab、QQ 同步设置、好友列表、黑名单和访客功能。
- `Friends.vue` 已修复好友管理页后台刷新后自动回到顶部的问题：整页加载态只在首次无数据时显示，已有数据刷新不再卸载主体内容；账号 watcher 也收窄到账号 id/运行状态变化，避免普通账号对象更新触发列表重载。
- 布局账号入口已调整：新增 `TopAccountMenu.vue`，将账号切换、添加账号、管理账号和备注编辑入口从左侧栏搬到顶部栏右侧；`Sidebar.vue` 删除原账号选择块，左侧更专注于用户信息、导航和底部状态。
- 左侧导航菜单已简化为短标签：概览、个人、好友、活动、商城、图鉴、分析、设置、后台；后台仍保留管理员可见限制。
- 活动页荷露抽奖点完后疑似掉线的问题已做保守防护：`core/src/services/activity.js` 对活动 Operate 增加连接状态检查，免费多抽改为串行节流请求并延迟刷新活动状态；`web/src/views/Activity.vue` 防止抽奖请求重复提交。服务已重启，`/api/health` 返回 200。
- 蹲守与飞升/秒偷功能已完全去除：前端设置 tab、独立蹲守页、相关组件、setting store 字段、后端 `/api/instant-steal`/`/api/stakeout` 路由、worker RPC/自动启动、runtime provider/config snapshot、store 配置模型和对应 service 文件均已删除；源码残留扫描无匹配。
- `core/src/controllers/admin-bag-routes.js` 已从 `_v###`/逗号表达式风格清理为命名 helper + 清晰路由处理；接口路径、主要返回结构和旧版缺账号行为保持对齐。
- `core/src/controllers/admin-farm-resource-routes.js` 已清理为命名 helper + 清晰路由处理；`/api/status` 缺账号 200 返回、其它资源接口缺账号 400 的旧行为保持对齐。
- `core/src/controllers/admin-public-info-routes.js` 已清理为命名 helper + 清晰路由处理；公共信息、更新日志、鉴权校验和 scheduler fallback 的返回结构保持对齐。
- `core/src/controllers/admin.js` 已完成入口可读性清理：CORS、静态资源、鉴权门、SPA fallback、Socket.IO 订阅逻辑均已命名化；`core/src/controllers/admin*.js` 当前无 `_v###` 残留。
- 已完成的关键结构成果：`Activity.vue`、`Settings.vue`、`Shop.vue` 已拆成页面协调器 + 组件/composables；`Friends.vue` 已开始拆页面壳，标题/搜索、统计卡、摘要提示和 tab 条已抽到 `web/src/components/friends/`；后端 `admin.js` 已拆出大量领域路由，入口主要负责启动、注册、静态资源、鉴权和 Socket.IO。
- `AdminPanel.vue` 已继续结构拆分：登录日志面板已抽到 `web/src/components/admin/AdminLoginLogPanel.vue`，登录日志状态/格式化/清空逻辑已抽到 `web/src/composables/useAdminLoginLogs.ts`，清空确认弹窗已抽到 `web/src/components/admin/AdminLoginLogConfirmModal.vue`；卡密面板已抽到 `web/src/components/admin/AdminCardPanel.vue`，卡密状态、筛选、复制、创建、删除和领取开关逻辑已抽到 `web/src/composables/useAdminCards.ts`，卡密确认弹窗已抽到 `web/src/components/admin/AdminCardConfirmModals.vue`；用户面板已抽到 `web/src/components/admin/AdminUserPanel.vue`，用户状态、统计、续费、封禁、删除、清理到期和编辑逻辑已抽到 `web/src/composables/useAdminUsers.ts`，用户确认弹窗已抽到 `web/src/components/admin/AdminUserConfirmModals.vue`；系统配置面板已抽到 `web/src/components/admin/AdminSystemPanel.vue`，系统/微信配置状态、加载、保存、重置和确认框开关已抽到 `web/src/composables/useAdminSystemConfig.ts`，对应确认弹窗已抽到 `web/src/components/admin/AdminSystemConfigConfirmModals.vue`；后台通用提示弹窗已抽到 `web/src/components/admin/AdminAlertModal.vue`；顶部汇总和 tab 外壳已抽到 `web/src/components/admin/AdminPanelHeader.vue`、`web/src/components/admin/AdminPanelTabs.vue`，页面目前主要负责模块 wiring。
- 活动能力状态：Helu 活动可见；南瓜活动后端保留但前端隐藏，属于 dormant/hidden 能力。
- 当前只保留 `progress.md` 作为接力摘要；`maintenance.md` 已删除，后续维护信息写回本文件但保持短而有用。

# 当前优先级

1. 继续拆高风险大页面，优先 `Friends.vue` 或 `AdminPanel.vue`，其次 `Login.vue`、`Sidebar.vue`、`Dashboard.vue`、`Analytics.vue`。
2. 继续拆 `AdminPanel.vue`，优先按卡密、用户、系统配置拆成 focused components/composables。
3. 对照旧版 `D:\github\qq-farm-2.3.1` 补齐/核对功能时，只迁移必要行为，按当前项目结构重落地。
4. 目录结构统一要随着模块拆分逐步做，不要先做大规模移动造成引用噪音。

# 关键决策

- 大页面只做协调器；业务规则、状态组合、重复 UI 拆到 composables、stores 或 focused components。
- 后端入口只做 wiring；具体接口按领域放到 `admin-*-routes.js` 和领域 helper/service 中。
- 后端 admin 控制器目录的 `_v###` 债务已清空；后续不要重新引入逗号表达式/反编译式临时变量风格。
- 隐藏或内部能力必须在接力记录中说明；尤其南瓜活动不要重新暴露到导航或活动页，除非用户明确要求。
- 中文乱码判断以 UTF-8 真实内容为准；终端乱码先用 Node 读取文件确认。
- 前端全量 ESLint 当前可通过；继续优先对触碰文件跑 targeted lint，阶段性再跑全量 lint 和构建。
- `git status`/`git diff` 在当前 worktree 曾不可用或不可靠；继续工作以实际文件内容和命令检查为准，不依赖 git 状态判断。

# 未完成待办

- 前端结构治理：
  - `Friends.vue`：页面仍大；标题/搜索、统计卡、摘要提示和 tab 条已抽到 `web/src/components/friends/`。下一步适合继续拆 QQ 好友自动同步设置块、好友列表项/分页、访客列表或 GID 弹窗。
  - `AdminPanel.vue`：主要面板 UI、卡密/用户/日志/系统配置逻辑、全部后台弹窗模板、顶部汇总和 tab 外壳均已抽离；页面目前主要保留各模块 wiring，后续优先转向 `Friends.vue`、`Login.vue`、`Sidebar.vue` 等仍偏大的页面。
  - 快速排查显示剩余主要债务不是语法错误，而是大文件和类型边界：`Friends.vue`、`Login.vue`、`Sidebar.vue`、`Dashboard.vue`、`Analytics.vue`、`models/store.js`、`core/worker.js`、`models/user-store.js`、`services/activity.js` 等仍适合分批拆分；前端大量 `any` 应优先从 store/API 边界和高频页面逐步收窄。
  - 逐步减少 `any`，优先在 store/API 边界和高频页面中补类型，不做一次性全项目类型重写。
- 后端结构治理：
  - 已清理 `admin.js`、`admin-bag-routes.js`、`admin-farm-resource-routes.js`、`admin-public-info-routes.js`，这些文件应保持 0 个 `_v###`。
  - 大型服务/模型仍需逐步拆分：`models/store.js`、`core/worker.js`、`models/user-store.js`、`services/activity.js`、好友/农场/仓库相关服务。
- 功能面核对：
  - 新增或恢复功能时，确认它是可见、隐藏、内部还是休眠。
  - 检查是否还有后端存在但前端无入口、且未记录状态的能力。

# 接力约定

- 新对话先读本文件，再读相关源码；不要凭记忆改。
- 每次只选一个明确模块推进：先读当前实现，必要时对照 `D:\github\qq-farm-2.3.1`，再小步修改和验证。
- 手工编辑使用 `apply_patch`；不要回滚用户或其他过程留下的无关改动。
- 后端改动后至少跑 `node --check` 对触碰文件；拆/改路由时再跑 `node -e "require('./core/src/controllers/admin'); console.log('backend require ok')"`。
- 前端改动后至少跑 targeted `npx eslint ...`；影响页面/组件结构时跑 `cd web && npm run build`。
- `@vueuse/core` Rollup pure-comment warning 是已知非阻塞警告；UnoCSS 拉取 Google web fonts 超时也可能在网络不稳时出现。构建退出码为 0 即可记录为通过。
- 遇到中文显示异常，先用 `node -e "const fs=require('fs'); console.log(fs.readFileSync('path','utf8'))"` 确认真实内容。
- 不把全站 lint、全站格式化、全站类型改造作为默认目标；只有用户明确要求或分阶段收敛到位后再做。
- 要记得及时更新本文件

## 2026-07-06 TSDK / ACE 修复

- 已对官方 `game.js` 完成关键静态去混淆：官方调用为
  `SdkInitEx(3167, 0)`；`AnoUserLogin(0, openId)` 仅维护账号身份。
- 官方网关 Token 来自 `AceManager.randomStr()`，格式为 64～127 个字母数字字符
  加 `=`，不是 `_generate_token` 返回的 24 字符 hash；登录后还会优先消费一次
  `_get_encrypted_init_info`，对应抓包里首条 `AllLands` 的特殊长 Token。
- ACE 生命周期已按官方封装拆分：5 秒处理接收队列和轮询上报、25 秒 TSDK
  heartbeat、30 秒速度检测、150 秒状态上报、180 秒函数检查。
- 官方 20260706 抓包确认：登录和好友操作 Token 均符合随机格式；4 分钟后仍能
  Enter、PutWeeds、PutInsects、Farming，说明持续有效依赖 ACE 状态而非固定 Token。
- `core` 定向 ESLint 通过，6 个离线测试通过；仍需使用测试账号完成至少 5 分钟
  在线好友操作验证后才能确认服务端链路完全恢复。

## 2026-08-19 Go 模块化迁移接力

- P3-02 已完成：yyb 的 `wechat_accounts`、`sessions`、`features` 通过 `0002_wechat.sql` 并入主 SQLite；`internal/yyb.DB` 现在只是共享主库门面，删除微信账号时同步清理会话和 `accounts.yyb_openid` 关联，不再创建独立 yyb 数据库。
- P1-05 已完成：新增 `--import-json <dir>`、`--conflict skip|overwrite`、`--data-dir` 导入命令，支持账号/配置/用户/卡密/登录记录/统计/好友缓存等旧 JSON 数据；导入只读源文件并输出计数。
- P5-08 已完成：新增单机 license 校验与机器码、SMTP mailer、HTTP pusher 基础包，保留旧 license 算法和默认关闭策略。
- P3-03 已完成：新增 `internal/yyb/service.go` 同进程 yyb 门面，覆盖 QR 创建/轮询/确认、账号管理、login_buffer 刷新和 wxapp 操作；协议池改为依赖独立 `internal/yyb/model` + 存储接口，避免包循环。
- 当前验证：`go test ./...`、`go vet ./...`、`go test -race ./internal/store ./internal/yyb` 通过。尚未做真实微信扫码和线上 wxapp 对拍，需在 P4 账号运行时联调时验证。

## 2026-08-20 W0-W12 收口

- W0-W12 的代码任务卡已全部完成：基础工程/配置/日志、SQLite 迁移与 repositories、JSON 导入、protobuf/TSDK/ACE/WS/登录会话、YYB 收编、账号 Runtime/调度/循环、warehouse/farm/friend/mall/task/activity/light domains、Gin/鉴权/账号访问/实时 Hub/领域 handlers，以及 W12 的前端 realtime/API 分域与账号头单源。
- 星砂不再是 stub，已接入 activity 服务与 HTTP 兑换路由；shop seed/pet/decoration/mystery、daily gifts、账号/密码/卡密/系统管理等原 501/假实现路径已改为真实 provider/domain 调用。
- 化肥 `/api/fertilizer/buy` 与 `/api/fertilizer/check-and-buy` 已改为调用 mall 域的真实购买/阈值检查接口，前端可获得实际购买数量。
- 前端已通过 `vue-tsc`、Vite build、全量 ESLint；Go 已通过 `go test ./...`、`go test ./... -race`、`go vet ./...`、`make lint`、`make build`、`make gen-proto`、`make test`。`make gen-proto` 在未安装 protoc 时校验 22 个已提交绑定。
- 真机登录、真实网关/微信扫码、长时间 ACE 稳定性尚未执行，按当前阶段要求留给后续联调；代码与离线测试闭环不依赖这些外部条件。

## 2026-08-20 P6-05 / P8-06 收口

- P6-05 已完成：`internal/app/application.go` 与 `internal/app/wire.go` 作为唯一 Go 组合根，集中持有数据库、repositories、YYB、sessions、runtime manager、realtime、metrics 与 HTTP server；`cmd/farmbot/main.go` 仅负责加载配置、创建 Application、运行和接收信号后关闭。
- P6-05 生命周期已补组合根测试：覆盖依赖装配、健康检查监听、上下文取消优雅退出、关闭幂等和关闭后拒绝重启；关闭顺序为 HTTP → runtime/realtime/session → SQLite。
- P8-06 已完成主题单点：`main.ts` 只初始化 Pinia/App store 并触发同步，`App.vue` 不再重复读取或应用主题，主题变量与 `.dark` 只由 `stores/app.ts` 维护。
- P8-06 已完成设置默认值单源：`stores/setting.ts` 使用 `createDefaultSettings()` 同时服务初始化和清理，嵌套配置每次生成独立对象。
- P8-06 已完成后台职责拆分：新增 `stores/admin.ts`，用户认证/续费保留在 `stores/user.ts`，后台用户、日志、卡密 CRUD 及领取开关统一由 admin store 提供，后台 composables 不再直接调用对应 API。
- P8-06 已完成陈旧账号守卫复用：新增 `composables/useStaleGuard.ts`，farm/activity/illustrated/friend/plant-blacklist/shop/status/setting 统一使用 `isCurrentAccount`。
- P8-06 已收窄关键边界：AccountModal 编辑数据、第三方/YYB 账号、好友/土地/访客接口、后台卡密组件改为明确类型；卡密格式化移至 `utils/card-format.ts`，避免认证 store 承担后台类型与格式化职责。
- 本轮验证通过：`go build ./...`、`go test ./...`、`go test ./... -race`、`go vet ./...`、`pnpm exec vue-tsc --noEmit`、`pnpm exec eslint "src/**/*.{ts,vue}"`、`pnpm build`。前端构建仍只有已知 UnoCSS 图标缺失与 Google Fonts 网络超时提示，退出码为 0；未执行真机测试。

## 2026-08-20 P7-04 收口

- 已删除旧 Node 后端目录 `core/` 和独立 `yyb-go/`；运行时用户数据目录不在删除范围内。
- 根 `package.json`、`pnpm-workspace.yaml`、锁文件、Vite 配置和 Makefile 已切换为 Go 单体 + `web` workspace；旧 YYB 外部进程环境变量和启动代理逻辑已移除，YYB 由 Go 进程内置提供。
- 已重写 `README.md`、`start.sh`、`start.bat`、`server-deploy.sh`，补齐源码运行、Docker 部署、备份/导入/导出、健康检查和数据目录说明；新增 `internal/tools/syncassets`，确保发布前端资源同步到嵌入目录。
- 已建立回退锚点 tag：`p7-04-pre-node-removal-20260820`。
- `make release` 已成功生成 Windows、Linux、darwin-amd64、darwin-arm64 四个平台产物；源码与发布二进制的 `/api/health`、`/api/ready` 冒烟检查通过。
- 验证通过：`go build ./...`、`go test ./...`、`go test ./... -race`、`go vet ./...`、`git diff --check`、前端 `vue-tsc`、ESLint、Vite build、`make build-web`、`make release`。Docker 因当前环境未安装未实测，真机测试按要求暂不执行。
- 发布前端构建期间恢复了工作区原有的 `web/src/layouts/DefaultLayout.vue`，当前内容已通过类型检查和构建；后续如继续调整布局，需以该文件为基准核对未提交差异。

## 2026-08-20 全面功能测试前置修复

- 初始管理员创建已接入 `ADMIN_PASSWORD`：仅首次创建时使用，已有管理员密码不会被每次启动覆盖；新增了 HTTP 登录闭环测试。
- 应用组合根现在强制要求有效 `FARM_MASTER_KEY`，缺失或非法时拒绝启动；YYB 安全测试不再继承宿主密钥，`go test` 在无密钥和有密钥两套环境均通过。

## 2026-08-20 功能逻辑与登录闭环复核

- 修复 Go 登录响应契约：只返回 `username/role/card/accountLimit/token`，不再泄露 `Password/PwdHash/Salt`；`/api/user/me`、注册、续费、后台用户列表和批量续费统一返回安全的前端字段；认证现在传递真实客户端 IP，恢复 IP 限流和 `429/423/403` 登录错误状态。
- 修复公开路由白名单漏项：`/api/register` 和 `/api/card/info/:code` 不再被 `AuthGate` 错误拦截；公共续费改为真正读取 `username/cardCode`，登录续费兼容前端 `cardCode` 字段。
- 修复后台用户编辑实际不落库的问题：新增事务性管理员更新，支持改用户名、密码、额度、有效期、永久卡、封禁/解封，并同步卡片 JSON、登录状态和会话失效；禁止删除当前登录管理员。
- 为账号和卡密 HTTP 模型补齐 lower-camel JSON 字段，避免接口 200 但前端读取不到 `id/running/code/enabled`。
- 新增登录响应、公开路由、管理员更新和封禁登录回归测试；本轮实际启动实例验证 `/api/health`、`/api/ready`、管理员登录、注册、封禁返回 403、账号/卡密列表及重命名改密登录。
- 已修复后端 `errcheck` 问题，更新 `.dockerignore`/`.gitignore` 的旧 `core` 路径，compose 和启动脚本补充密钥要求，新增 `.env.example`。
- 发布二进制实测：health/ready 正常，自定义管理员密码登录 200，默认 `admin` 密码在配置覆盖后返回 401；临时数据库和进程已清理。
