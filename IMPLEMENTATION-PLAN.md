# Mihomo Health Controller Implementation Plan

> **For agentic workers:** Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** 实现已获用户批准的 Go 后端、Web 配置面板和 Docker Compose 部署。

**Architecture:** Go 标准库提供 HTTP API、嵌入式静态面板与站点池。后台串行调度，通过带 Bearer secret 的 Mihomo API 检查当前出口；达到连续失败阈值后整组测试，按主分组 all 原始顺序切换。配置原子持久化，变更配置时取消旧任务，避免旧请求继续切换。

**Tech Stack:** Go 1.26+、原生 HTML/CSS/JavaScript、Docker Compose。

**Spec:** README.md 与用户批准的六项基本配置、自动探查及历史持久化开关；默认间隔 5 秒、失败阈值 1 次、超时 4 秒，历史持久化默认关闭。

## Global Constraints

- 直接在当前 master 分支工作，不创建分支或 worktree；按用户已授予的仓库维护授权提交和推送。
- 保留国家 fallback 层级，不修改订阅示例，不将凭据打包进镜像。
- Secret 不回显；面板空 secret 表示保留，明确操作才清除。
- 原始顺序优先，全部失败保留当前选择，API 认证错误不当作出口故障。
- 主循环不重入，配置变更取消旧请求，跳过忙碌期间的时钟周期。
- Compose 只部署健康检查服务，使用 Docker Hub 已发布镜像，通过 API 连接已有 Mihomo；面板仅回环地址发布，配置使用持久化卷。

## Review Focus

- 配置变更/停用时，旧任务不能切换分组或覆盖新状态。
- delay 结果缺项、零值、错误响应不得误判为可用。
- 凭据不得出现在 HTTP 响应、日志、重定向或容器构建上下文中。
- API 失联和认证失败不应触发无意义的整组探查。
- 保存失败、重启恢复以及刷新页面不能丢配置或意外启动未配置任务。

### Task 1: API client and ordered failover

Files: `go.mod`, `mihomo.go`, `controller.go`, `controller_test.go`.

- [x] 使用 httptest 编写失败测试：认证与编码、阈值累计/成功归零、原始顺序优先、全部失败保留、切换失败再尝试、取消与单任务运行。
- [x] 运行测试观察缺失功能失败，实现接口与串行探查逻辑，运行测试通过。

### Task 2: configuration, persistence and HTTP server

Files: `config.go`, `server.go`, `main.go`, `server_test.go`.

- [x] 先测试配置校验、原子保存/恢复、secret 保留/清除与脱敏、草稿连接测试、错误请求、配置取消旧任务。
- [x] 实现配置存储和后台调度，提供配置、状态、连接测试、分组列表与存活检查端点。
- [x] 运行全套测试与 go vet。

### Task 3: Web panel and deployment

Files: `web/index.html`, `web/app.js`, `web/style.css`, `Dockerfile`, `docker-compose.yml`, `.dockerignore`, `.gitignore`, `DEPLOY.md`, `README.md`.

- [x] 面板实现配置、连接测试、分组选择、启停、实时状态、原始顺序结果和事件记录；支持窄屏。
- [x] 添加健康检查服务 Compose 部署，提供持久化及回环网络说明。
- [x] 构建 Go 程序、跨编译 Linux；执行 HTTP 集成和浏览器操作检查；Compose 通过官方 Schema 校验，本机没有 Docker，未实跑镜像。
- [x] 最终代码审查，记录检查结果与实际环境限制，见 VERIFICATION.md。

## 完成记录

- Task 1: complete — API client、串行探查与原始顺序切换，回归测试通过。
- Task 2: complete — HTTP 配置、持久化与后台调度，回归测试通过。
- Task 3: complete — Web 面板、部署文件、浏览器与进程重启检查通过。
- Final: fixed — HTTP 503、超时参数上限、DNS 重绑定 Host 校验，均已先复现失败再修复通过；JSON null 拒绝一并回归。
- 决策：遵守用户规则保留当前分支，不创建 worktree；提交与推送按用户后续仓库维护授权执行。保留原订阅国家 fallback 层级，测试不读取真实凭据或连接真实节点。

## 后续：最近 1 小时记录

- [x] 按 1 小时时间窗口保存记录，内存写入和查询时清理过期数据；状态接口仅带最新 100 条。
- [x] 添加稳定 ID 游标分页接口及面板“查看更早 / 返回最新”，每页最多 100 条。
- [x] 添加 4 项 Go 历史行为测试及 1 项前端响应乱序回归测试，独立审查问题已修复。
- [x] 浏览器检查分页、新事件游标稳定性、返回实时记录、过期剔除与窄屏布局通过；全套检查及跨编译通过。

## 后续：历史持久化与批量落盘

- [x] 历史与探查配置独立，修改配置或停止时保留近 1 小时的事件及分页标识。
- [x] 启动读取 `DATA_DIR/history.json`，恢复序号并过滤过期记录。
- [x] 独立 5 分钟批次原子替换快照，内容无变化不写盘，查询不触发磁盘访问，失败后下批重试。
- [x] 正常停止先取消并等待探查，再补写历史；Compose 为控制器提供 45 秒停止宽限（独立项目部署时对齐 z-media-workflow）。
- [x] 7 项新增 Go 测试先失败再通过；程序进程恢复/配置修改/重建检查、独立源码审查和 Linux 双架构构建通过。
- 决策：接受批量落盘的崩溃窗口，强杀/断电可能丢失上次成功保存后的记录；正常停止额外保存一次。磁盘过期项在下个批次删除，查询始终只返回近 1 小时。

## 后续：历史持久化可选，默认关闭

- [x] 增加 `persist_history` 配置与面板开关，默认关闭；缺失该字段的旧配置同样关闭。
- [x] 关闭时启动不读历史文件，批次及正常停止不写历史文件，保留当前内存记录和已有磁盘快照。
- [x] 开启后按 5 分钟批次保存当前内存；运行中开启不合并旧磁盘快照，启动时已开启才恢复快照。
- [x] 补充默认关闭、运行时切换、API 配置保存、空内存开启回归测试；浏览器保存/刷新与窄屏检查通过，独立审查无重要问题。

## 后续：抽离 z-mihomo-health-check 独立项目

- [x] 将完整目录迁移至 `E:\personal\code\z-mihomo-health-check`，保留订阅本地文件与忽略规则，复制原仓库许可证；原笔记目录仅保留迁移说明。
- [x] 独立 Go 模块改名 `z-mihomo-health-check`；默认部署文件为 `docker-compose.yml`，参考 z-media-workflow 的镜像/容器命名、wk 网络、自动重启和日志轮转约定。
- [x] 只部署控制器，通过 API 连接已有 Mihomo；镜像 `jiuwusan/z-mihomo-health-check:latest` 由 GitHub Actions 构建推送，Compose 仅拉取并运行已发布镜像。
- [x] 修正文档、命令和本地部署路径，说明旧卷复用；运行全套测试、Linux 构建、Compose 官方 Schema 校验和独立源码审查。
- 迁移检查：新目录独立构建，空工作目录运行验证通过；原笔记目录只有迁移入口。迁移阶段没有自动创建分支、提交或发布镜像。

## 后续：GitHub Actions 自动发布

- [x] 参考 z-media-workflow 创建 `.github/workflows/docker-publish.yml`，主分支使用 master，支持 PR 和手动触发。
- [x] 先运行 Go race/vet 与 Web 检查，再构建 linux/amd64、linux/arm64，推送 `jiuwusan/z-mihomo-health-check`；PR 不登录、不推送。
- [x] 沿用 `OCKERHUB_USERNAME` / `DOCKERHUB_USERNAME` 回退与 `DOCKERHUB_TOKEN`，主分支生成 latest、master、sha 标签，其他分支不覆盖 latest。
- [x] 更新发布与拉取更新说明，actionlint、本地 Go/Web 测试和独立静态审查通过。
- 发布记录：仓库、Docker Hub Secrets 与自动发布已验证，Ubuntu 测试及双架构镜像推送成功，运行记录见 VERIFICATION.md。

## 后续：镜像部署同步复验

- [x] Compose 只包含 image，部署文档统一使用 pull / up；源码构建仅由 GitHub Actions 中的 Dockerfile 执行。
- [x] 首次安装的 API 默认值、面板占位和说明统一到 wk 服务地址；已有保存地址原样保留，有新安装及旧配置恢复回归测试。
- [x] 更新计划和验证记录中的发布状态、单服务部署约束；自定义探查间隔说明明确默认 5 秒的频率基准。
- [x] 维护文档将更新、日志、停止命令分开展示，避免复制更新命令时顺带停止服务。
