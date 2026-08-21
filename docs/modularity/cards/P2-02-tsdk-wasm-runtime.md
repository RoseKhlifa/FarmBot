# 卡片 P2-02：TSDK 安全运行时（wazero 加载 WASM）—— 🔴 最高风险

| 调度头 | 值 |
| --- | --- |
| 波次 | **W3**（尽早起，决定 go/no-go） |
| 前置依赖 | P0-03 |
| 独占文件 | `internal/game/tsdk/{runtime,imports,verify}.go` `assets/wasm/**` `assets/embed.go`(wasm 部分) |
| 可与谁并发 | P1-02 |
| 风险 | 🔴 极高（反作弊命脉，错误=全量封号/掉线） |
| 闸门 | **本卡 + P2-05 真机通过前，不推进 P3+**（04 §6 go/no-go） |

---

- 目标：用纯 Go 的 wazero 加载官方 `tsdk-v3.8.2.wasm`，实现 22 个宿主 import 与 12 个导出封装，**不重写加密算法**。
- 源（现状）：`core/src/utils/tsdk-runtime.js`(594)、`crypto-wasm.js`、`core/src/utils/*.wasm`；**规格权威**：`core/docs/tsdk-ace-runtime.md`。
- 目标（Go）：`internal/game/tsdk/{runtime.go,imports.go,verify.go}` + `assets/wasm/`（go:embed）。
- 实现要点：
  - `verify.go`：校验 SHA-256（`705e326c...4850a5f`）、导入数量、必要导出；失败**显式中止**（保持现状不回退伪 Token 姿态）。
  - `runtime.go`：wazero 实例化；按官方顺序——实例化 22 个 `a.a`–`a.v` 宿主 → mergewasm 解密 17 段 → `__wasm_call_ctors` → `SdkInitEx(3167,0)` 写 appKey → `_init_runtime(3167, appKeyPtr)`。
  - 12 个导出封装：create/destroy buffer(A/B)、getResult(C, 64B, WASM 所有)、init(G)、heartbeat(M)、getDataToServer(N, WASM 所有)、sendDataFromServer(O)、generateToken(aa)、encrypt/decrypt in-place(ba/ca)、encrypt/decrypt v2(da/ea)。**严格按内存所有权规则**：调用者分配的输入与 Token 返回指针复制后 `_destroy_buffer`；`C`/`N` 返回的 WASM 内存不由 Go 释放；复制前校验指针/长度/memory 边界。
  - `imports.go`：逐一实现 a.a–a.v（assertion/read-write file 到账号独立目录、JS stack、version `v3.8.2.1783066265`、clock、device info、app id `1112386029`、TQOS 异步 POST 等，逐行照规格表）。
  - **每账号一个 tsdk 实例**（字段挂在 P4 Runtime），随重连销毁重建。`FARM_TSDK_ACE_ENABLED=false` 关闭整链。
- 不要做：不试图逆向或重写 WASM 内部算法；不共享单例实例给多账号。
- 验收：**离线单测**——固定输入下 `encrypt/decrypt/generateToken` 与 Node `tsdk-runtime.js` 输出**逐字节一致**（先在 Node 侧 dump 若干 fixture）；SHA256/导出校验用例。
- 完成判据：☐ 三 wasm 校验通过 ☐ 12 导出封装正确 ☐ 22 import 实现 ☐ 与 Node 输出对拍一致。
