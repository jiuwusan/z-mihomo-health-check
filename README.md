# z-mihomo-health-check

Go 后端与 Web 面板，通过 Clash API 探查当前出口，连续失败达到阈值后，按主分组原始顺序选择可用国家分组。

独立 Go 项目，包含配置面板、200 个随机测试站点、后台自动探查、原始顺序切换、配置持久化及 Docker Compose 部署。部署步骤见 [DEPLOY.md](./DEPLOY.md)。

```sh
# wk 已存在时跳过创建
docker network create wk
docker compose config --quiet
docker compose pull
docker compose up -d
```

默认 [docker-compose.yml](./docker-compose.yml) 只启动健康检查服务，加入已有 `wk` 网络，连接现有 Mihomo。打开 [http://127.0.0.1:8080](http://127.0.0.1:8080)，填写后端可访问的 API 地址（例如 `http://mihomo:9090`）、Secret、目标分组，测试连接后启用并保存。首次安装默认停用。

Compose 参考 `z-media-workflow`：镜像 `jiuwusan/z-mihomo-health-check:latest`、同名容器、`restart: always`、45 秒停止宽限、持久化卷及 10MB × 3 日志轮转。镜像由 [GitHub Actions](./.github/workflows/docker-publish.yml) 自动构建并推送 Docker Hub，Compose 仅使用已发布镜像，部署机器只需部署文件，无需源码或 Dockerfile。

站点池见 [SITES.md](./SITES.md)，脚本可读取 [sites.txt](./sites.txt) 或 [sites.json](./sites.json)。本机探测记录见 [probe-results.json](./probe-results.json)。

## 1. 目标与使用场景

健康检查服务由 Go 后端和嵌入式 Web 面板组成，通过 API 连接已有 Mihomo。随本地迁移保留的订阅示例 `MY-1793463295556.yaml` 含真实凭据，已被 Git 和 Docker 构建上下文忽略；该示例包含 55 个 SS 节点、8 个国家/地区 fallback 分组，以及 `🚀 节点选择`、`🎯 全球直连` 两个 select 分组，仅用作策略组结构参考。

控制器管理 `🚀 节点选择`，保留国家分组及其内部的节点选择逻辑。国内直连规则维持订阅配置。后续运行配置不将主分组展开为 55 个实际节点。

## 2. 健康状态探查

已准备 [200 个海外测试站点](./SITES.md)，覆盖云服务、开发平台、教育科研、新闻文化等类别。站点池附有本机 HTTPS HEAD 探测快照，接入前需通过实际候选节点复测。

常规探查规则：

- 探查间隔默认 **5 秒**，可在面板配置为 1–86400 秒。
- 每轮从站点池中**均匀随机抽取 1 个站点**，测试 `🚀 节点选择` 当前选中的出口，不遍历整个池。
- 每轮重新随机选择，允许不同轮次抽中同一站点。
- 连续失败达到面板配置的阈值后触发主分组整组 delay 测试，随后按第 3 节规则选择可用国家分组；默认阈值为 **1 次**，成功一次即清零。

默认 5 秒间隔下，单个常规探查循环每分钟计划发起 12 次测试；200 个站点等概率抽取时，每个站点平均约 1,000 秒（16 分 40 秒）被常规探查选中一次。修改间隔后，单站长期平均选中间隔为“站点数量 × 配置间隔”，不保证两次访问之间的最短间隔。失败触发的整组 delay 测试会增加实际出站请求；订阅原有的国家 fallback 分组也有独立探查，实际总量不受每分钟 12 次限制。随机分散请求用于降低单站访问频率，不能保证不触发风控。

常规探查调用 Mihomo 单代理 delay API，测试超时默认 4 秒，面板支持 1–32 秒。超时或测试失败累计出口失败次数；控制接口失联、认证或响应格式错误单独展示，不增加出口失败次数。后台串行调度，跨过的时钟周期跳过，不补发积压任务。站点池初筛快照的 12 秒超时不作为运行时配置。

## 3. 节点自动切换策略

### 3.1 失败处理流程

1. 常规探查成功：保留当前选择，等待下一个配置周期（默认 5 秒）。
2. 常规探查失败：累计失败次数，达到配置阈值时触发 `🚀 节点选择` 的整组 delay 测试。整组测试使用同一个随机测试 URL，以便在相同目标下判定各国家分组的可用性。
3. 整组测试完成后，从有序列表的首项开始依次检查，选择本轮测试成功的第一个国家分组。**不按延迟数值排序，不使用 delay 响应 JSON 对象的遍历顺序。**
4. 若该分组就是当前选择，则保持当前选择；若不同，则通过 API 切换主分组。切换请求失败时，重新确认当前选择，尚未切换成功则按原顺序尝试下一个本轮通过测试的分组。
5. 全部分组不可用，或整组测试请求失败：保留当前选择，记录故障，下一周期重新执行常规探查，不进入无间隔重试循环。

每轮故障处理完成后恢复常规探查。控制器避免多轮故障处理同时运行；修改配置或停用会取消旧配置的任务、重置探查计数等当前状态，保留最近 1 小时的历史记录。

### 3.2 国家分组优先级

当前示例中，`🚀 节点选择.proxies` 的原始顺序如下：

1. `🇺🇸 SS 美国`
2. `🇯🇵 SS 日本`
3. `🇸🇬 SS 新加坡`
4. `🇷🇺 SS 俄罗斯`
5. `🇬🇧 SS 英国`
6. `🇭🇰 SS 香港`
7. `🇹🇼 SS 台湾`
8. `🏳️ SS 其他`

例如，美国测试成功且延迟为 900 ms，日本测试成功且延迟为 100 ms，仍选择美国；只有美国本轮不可用，才检查日本，依此类推。每次故障恢复选择都从列表首项开始，不从当前分组的下一项开始。常规探查成功时不会为了回到高优先级国家而主动触发整组测试。

订阅更新后，以新配置的主分组原始顺序为准，不硬编码上述列表。运行时读取主分组的 `all` 数组，并在接入验证时核对它与生效配置的成员顺序一致。

### 3.3 Mihomo API

- `GET /proxies/{主分组名}`：读取当前选择 `now` 和有序成员列表 `all`。
- `GET /proxies/{主分组名}/delay?url={随机URL}&timeout={毫秒}&expected=200-299`：常规探查当前选择的出口。
- `GET /group/{主分组名}/delay?url={随机URL}&timeout={毫秒}&expected=200-299`：失败后测试主分组内的国家分组。
- `PUT /proxies/{主分组名}`，请求体 `{"name":"国家分组名"}`：选择国家分组。

默认主分组名为 `🚀 节点选择`，面板可加载并选择其他 select 分组；实际请求对路径分组名和查询 URL 分别编码。整组 delay 结果仅用于判定本轮可用性，不作为排序来源；不得用之前的历史成功记录代替本轮结果。结果缺项、0、负数或超出 uint16 范围视为本轮不可用。

接口依据：[Mihomo API 文档](https://wiki.metacubex.one/api/)。本方案只选择主分组的直接成员，不手动切换国家分组内部的实际节点，也不将国家 fallback 组改为 select。

## 4. 配置与脚本

| 配置项 | 默认值 |
| --- | --- |
| Clash API 地址 | `http://mihomo:9090`，按已有服务实际地址修改 |
| Secret | 面板输入，不回显；留空保留已保存值 |
| 目标分组 | `🚀 节点选择` |
| 探查间隔 | 5 秒 |
| 连续失败次数 | 1 次 |
| 探查超时 | 4 秒 |
| 启用自动探查 | 关闭 |
| 持久化历史记录 | 关闭，按需开启 |

Go 程序嵌入站点池和前端资源，无第三方 Go 依赖。配置默认存入 `./data/config.json`，容器中存入 `/data/config.json`。配置保存后立即生效；已启用的配置在进程重启后恢复运行。历史默认只存内存；面板开启“持久化历史记录”（`persist_history`）后，每 5 分钟批量保存到同目录 `history.json`，未变化时跳过写入，正常停止时额外保存一次。Secret 文件未静态加密，请按凭据文件保管。

首次安装的 API 地址使用 `wk` 网络中的服务名示例，已有配置文件中的地址原样保留；本机开发连接本机 Mihomo 时可填写 `http://127.0.0.1:9090`。Compose 在宿主机所有网卡发布面板端口，可通过服务器 IP 或域名访问，无需配置 Host 白名单；远程访问与数据卷说明见 [部署文档](./DEPLOY.md)。

## 5. 验证与故障排查

```sh
go test ./...
go vet ./...
go run .
```

安装 Node.js 的开发环境还可执行 `node --test web_test.cjs` 验证面板异步响应顺序；运行应用和构建容器无需 Node.js。

测试使用模拟 Clash API，验证 Bearer 认证、阈值累计/重置、原始顺序优先、不可用结果、切换失败重试、配置取消、后台调度、Secret 保留/清除、访问来源校验与配置持久化。实际节点连通性需要在目标机器上验证。

面板展示当前国家分组、最近测试延迟、失败计数、随机站点、本轮国家分组测试结果及最近 **1 小时**的事件。每页最多 100 条，支持“查看更早”与“返回最新”，修改配置不清空记录。历史持久化默认关闭，重启后内存历史不保留；开启时才加载/保存磁盘快照并支持跨重启恢复。关闭不会删除已有快照。内存及时清理过期项，查询不读写磁盘；开启后的批量保存及异常退出窗口见 [DEPLOY.md](./DEPLOY.md)。HTTP 503/504 delay 失败会累计出口失败；401/403 提示检查 Secret。容器健康检查仅表示 Web 进程可用，不代表代理出口正常。

## 6. 实践记录

2026-11-01：完成 Go 后端、Web 面板和独立服务 Compose 部署，并抽离到 `E:\personal\code\z-mihomo-health-check`。测试与环境限制见 [VERIFICATION.md](./VERIFICATION.md)。

## 7. GitHub Actions 发布镜像

工作流位于 [.github/workflows/docker-publish.yml](./.github/workflows/docker-publish.yml)，参考 `z-media-workflow`，沿用 Docker Hub Secret 命名和镜像标签约定。

1. 将本项目作为独立 GitHub 仓库的根目录上传，主分支使用 `master`。
2. 在 Docker Hub 准备 `jiuwusan/z-mihomo-health-check` 仓库，并创建具有该仓库推送权限的访问令牌。
3. 在 GitHub 仓库 **Settings → Secrets and variables → Actions** 配置 `OCKERHUB_USERNAME` 和 `DOCKERHUB_TOKEN`；用户名也兼容 `DOCKERHUB_USERNAME`，两者同时存在时优先使用 `OCKERHUB_USERNAME`。
4. 推送 `master`，或在 **Actions → Build and Push Docker Image → Run workflow** 手动触发。

流程先在 Ubuntu 上执行 Go 1.26 的 `go test -race`、`go vet` 和 Node.js 22 的 Web 检查，再通过 Buildx 构建并推送 `linux/amd64`、`linux/arm64` 镜像。`master` 发布 `latest`、`master`、`sha-<提交短哈希>`；手动构建其他分支发布分支和提交标签，不覆盖 `latest`。面向 `master` 的 PR 执行相同测试及双架构构建，但不登录 Docker Hub、不推送镜像。

运行应用不需要 Node.js，Node.js 仅供 CI 的 Web 回归测试。配置、历史文件、订阅和 API Secret 不进入镜像构建上下文，Actions 也不需要这些运行时凭据。官方参考：[Docker 多架构 GitHub Actions](https://docs.docker.com/build/ci/github-actions/multi-platform/)、[Go 环境设置](https://github.com/actions/setup-go)。

工作流成功发布镜像后更新服务：

```sh
docker compose pull
docker compose up -d
```

`latest` 镜像已通过 GitHub Actions 成功发布，支持 `linux/amd64`、`linux/arm64`。后续版本的构建与推送结果以 GitHub Actions 运行记录为准。Dockerfile 用于 CI 构建，部署 Compose 不包含 `build` 配置。

许可证：[Apache-2.0](./LICENSE)，沿用原笔记仓库的许可证。
