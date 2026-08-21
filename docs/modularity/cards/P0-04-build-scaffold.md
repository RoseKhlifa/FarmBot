# 卡片 P0-04：构建脚手架（Makefile + lint + CI）

| 调度头 | 值 |
| --- | --- |
| 波次 | **W1** |
| 前置依赖 | P0-01 |
| 独占文件 | `Makefile` `.golangci.yml` |
| 可与谁并发 | P0-02, P0-03 |
| 风险 | 🟢 低 |
| co-edit 提示 | P7-02(可选) 之后会向 `Makefile` 追加 release 目标 |

---

- 目标：统一构建/生成/测试/lint 入口，锁定验收命令。
- 源（现状）：根 `package.json` scripts、`web/package.json`。
- 目标（Go）：根 `Makefile`（或 `Taskfile`）+ `.golangci.yml`。
- 实现要点：
  - Make 目标：`build`(go build 出二进制)、`build-web`(pnpm -C web build)、`gen-proto`(protoc 生成，见 P2-01)、`test`(go test ./...)、`lint`(golangci-lint run + eslint web)、`docker`(见 P7-03)。
  - `.golangci.yml`：启用 govet/staticcheck/gofmt/errcheck 等基础 linter。
  - 保留 `web` 的 `pnpm lint`/`vite build` 作为前端验收。
  - **Docker 优先**：`docker` 目标是主交付路径；`release`(跨平台二进制) 为可选，默认不做（见 02/P7）。
- 不要做：不引入复杂 CI 平台绑定；先本地可跑。
- 验收：`make build`、`make lint`、`make test` 均可执行（此阶段 test 可为空但命令成立）。
- 完成判据：☐ Makefile 就位 ☐ golangci 配置就位 ☐ 三条命令可跑。
