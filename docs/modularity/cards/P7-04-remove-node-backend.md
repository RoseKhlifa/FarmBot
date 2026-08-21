# 卡片 P7-04：删除旧 Node 后端 + 文档更新

| 调度头 | 值 |
| --- | --- |
| 波次 | **W16**（绞杀收口，全流程最末一波，不可逆） |
| 前置依赖 | P6-04（全部路由切 Go 并真机稳定运行一个周期）、P7-03 |
| 独占文件 | 删除 `core/`、`yyb-go/`；重写 `README.md`、根 `package.json` |
| 可与谁并发 | 无（清场卡，需全绿后单独做） |
| 风险 | 🔴 高（删代码不可逆，务必留 tag + 全量验证在前） |

---

- 目标：绞杀完成后移除 `core/` 与代理层，更新 README/部署文档。
- 源（现状）：`core/`、`yyb-go/`（已收编）、绞杀代理、`README.md` 部署章节、根 `package.json`(workspace)。
- 目标：删除 `core/`、`yyb-go/`（其代码已在 `internal/`）；更新根 `package.json` 或改为纯 `web/` 前端 + Go 后端结构；README 重写部署三方式。
- 实现要点：
  - 确认无路由仍指向 Node、无脚本引用 core 后再删。**保留一个 git tag 作回退锚点。**
  - README：源码（`go run` + `pnpm -C web dev`）、二进制（下载单文件运行）、Docker（compose up，**首选**）三方式，均说明 yyb 已内置、数据在 `data/farmbot.db`、首启可 `--import-json` 迁移旧数据。
- 不要做：不在未全量验证前删 core；不删用户 `data/`。
- 验收：仓库 `go build ./...` + `vite build` 通过；三种部署文档实测可跑。
- 完成判据：☐ core/yyb-go 移除 ☐ 无残留引用 ☐ README 三方式更新 ☐ 回退 tag 就位。

> **迁移终点**：至此纯 Go 单体 + 内嵌前端 + 内置 yyb，Docker 一条命令部署。商用硬化（PC-*）可在此基座上独立推进。
