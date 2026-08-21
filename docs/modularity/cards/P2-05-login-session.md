# 卡片 P2-05：登录会话 —— 🔴 go/no-go 真机闸门

| 调度头 | 值 |
| --- | --- |
| 波次 | **W5** |
| 前置依赖 | P2-02、P2-04 |
| 独占文件 | `internal/game/session/**` |
| 可与谁并发 | P1-05, P3-01 |
| 风险 | 🔴 高 |
| 闸门 | **本卡真机验收是整个迁移的关键闸门；通过才推进 P4+** |

---

- 目标：完成从登录 code 到游戏在线的握手会话。
- 源（现状）：`network.js` 登录流程 + `tsdk-ace-runtime.md` 初始化 6 步。
- 目标（Go）：`internal/game/session/login.go`。
- 实现要点：
  - 输入登录 code（来自 P3 yyb），执行 `AnoUserLogin(0, openId)` 绑定账号，登录请求用 TSDK 请求体加密；成功后启动 ACE 生命周期与数据轮询。
  - 暴露 `Login(ctx, code) (*Session, error)`，Session 持 openid/uin/在线态，交给 Runtime。
- 不要做：不在此处启动自动化循环（那是 P4）。
- 验收：**P2 阶段出口真机验收**——单账号用真实 code 登录成功，跑通首条 `AllLands` + 一次好友操作，ACE 心跳无告警 ≥30 分钟。
- 完成判据：☐ 登录握手成功 ☐ AllLands 正常 ☐ 好友操作成功 ☐ ACE 稳定。

> ⚠️ go/no-go：P2-02 + 本卡真机验证是整个迁移的关键闸门。通过后再推进 P3+。失败则按 04 §6 折中方案，把桥限制在 TSDK 单点。
