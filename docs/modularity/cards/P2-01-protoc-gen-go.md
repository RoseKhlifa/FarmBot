# 卡片 P2-01：protoc-gen-go 生成 22 个协议

| 调度头 | 值 |
| --- | --- |
| 波次 | **W2** |
| 前置依赖 | P0-04 |
| 独占文件 | `proto/**` `internal/game/pb/**` |
| 可与谁并发 | P1-01 |
| 风险 | 🟢 低 |

---

- 目标：`.proto` 源改用 Go 官方编译，得到编译期类型安全的 `*.pb.go`。
- 源（现状）：`core/src/proto/*.proto`（22 个，1591 行）+ `utils/proto.js`（protobufjs 运行时加载 `types` map）。
- 目标（Go）：`proto/*.proto`（平移）→ `internal/game/pb/*.pb.go`（生成，提交入库）。
- 实现要点：
  - 平移 22 个 `.proto` 到 `proto/`；为每个补 `option go_package`（如 `.../internal/game/pb;pb`）。
  - `make gen-proto`：`protoc --go_out=... proto/*.proto`。**生成码提交仓库**，使用者无需装 protoc。
  - 注意现状可能存在的嵌套/opaque 字段（`activitypb` 返回 bytes，见 P5-06 activity）。
- 不要做：不手写 protobuf 结构；不改 `.proto` 的 message/field 编号（wire 兼容命脉）。
- 验收：`make gen-proto` 无错；`go build ./internal/game/pb/...` 通过。
- 完成判据：☐ 22 个 pb.go 生成 ☐ go_package 正确 ☐ 编译通过。
