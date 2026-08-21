# 卡片 P2-03：ACE 生命周期服务

| 调度头 | 值 |
| --- | --- |
| 波次 | **W4** |
| 前置依赖 | P2-02 |
| 独占文件 | `internal/game/ace/**` |
| 可与谁并发 | P1-03, P1-04, P2-04 |
| 风险 | 🟠 中高 |

---

- 目标：移植反作弊心跳/上报节拍。
- 源（现状）：`core/src/services/ace-service.js` + `tsdk-ace-runtime.md` §网关与 ACE。
- 目标（Go）：`internal/game/ace/service.go`。
- 实现要点：
  - 节拍常量：每 5s 检查 `_get_data_to_server` 非空则调 `gamepb.acepb.AceService.AntiData`；`_process_received_data` 5s；`_send_heartbeat_tick` 25s；速度检测 30s；状态上报 150s；函数检查 180s。普通用户心跳独立 25s。
  - 同账号最多一个在途 AntiData，失败 2–30s 有限退避。服务器 `reply.data` 原样回灌 `_send_data_from_server`。
  - 用 Go timer/ticker + context 取消；挂在 Runtime 上，随账号停止/重连销毁。
- 不要做：不改节拍时长；不并发多条 AntiData。
- 验收：单测断言各 ticker 周期与在途去重逻辑；与 Node 行为对照。
- 完成判据：☐ 六类节拍就位 ☐ 在途去重+退避 ☐ 随账号生命周期销毁。
