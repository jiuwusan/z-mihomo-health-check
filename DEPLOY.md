# z-mihomo-health-check 部署

面板与探查任务均由 Go 后端提供，不依赖 Node.js 或前端构建工具。面板关闭后继续探查；配置保存在数据卷中，容器重建后保留，保存为启用状态的配置会在重启后自动恢复探查。

## 1. 默认部署：连接已有 Mihomo

需要 Docker Engine / Docker Desktop 与支持 `docker compose` 的 Compose v2。独立项目位于 `E:\personal\code\z-mihomo-health-check`。默认 `docker-compose.yml` 只启动健康检查服务，通过外部 `wk` 网络访问已有 Mihomo。在项目目录执行：

```sh
# 网络已存在时跳过创建；z-media-workflow 等项目可共用 wk
docker network ls --filter name=wk
docker network create wk
docker compose config --quiet
docker compose pull
docker compose up -d
docker compose ps
docker compose logs -f --tail=100 z-mihomo-health-check
```

默认配置沿用 `z-media-workflow` 的部署约定：

- 服务和容器名：`z-mihomo-health-check`。
- 镜像名：`jiuwusan/z-mihomo-health-check:latest`，由 GitHub Actions 构建并推送 Docker Hub。
- 网络：已存在的外部 `wk` 网络。
- 重启策略：`always`，停止宽限：45 秒，包含进程存活检查。
- 日志：`json-file`，单文件最多 10MB，保留 3 个。
- 数据：`z-mihomo-health-check-data` 命名卷挂载到 `/data`。
- 面板：宿主机 `127.0.0.1:8080`，前端与 Go API 同源提供。

默认部署只需 `docker-compose.yml`，连接参数通过面板保存，无需订阅文件、源码、Dockerfile 或 `.env`。项目通过 [GitHub Actions](./.github/workflows/docker-publish.yml) 构建镜像，设置 Docker Hub Secrets 后推送 `master` 即可自动发布；部署机器执行 `docker compose pull`、`docker compose up -d` 拉取并更新。调整宿主机端口时直接编辑 YAML 中的 `ports`。工作流触发方式、Secret 命名和标签规则见 [README.md](./README.md#7-github-actions-发布镜像)。

现有 Mihomo 必须接入 `wk`，且控制接口监听容器网络地址，例如 `0.0.0.0:9090`，并配置 secret。面板 API 地址填写例如 `http://mihomo:9090`；`mihomo` 应是该网络内可解析的容器服务名或网络别名。无需将控制接口映射到公网。

打开 [http://127.0.0.1:8080](http://127.0.0.1:8080)，填写：

| 面板配置 | 值 |
| --- | --- |
| Clash API 地址 | 默认独立部署示例：`http://mihomo:9090` |
| API Secret | Mihomo 配置中的 `secret`，不是节点密码 |
| 目标分组 | 测试连接后选择 `🚀 节点选择` |
| 探查间隔 | 默认 5 秒 |
| 连续失败次数 | 默认 1 次 |
| 探查超时 | 默认 4 秒，范围 1–32 秒 |
| 持久化历史记录 | 默认关闭；按需开启 |

先点击“测试连接并加载分组”，确认可连接，再勾选“启用自动探查”并保存。新安装默认不启动探查。

面板没有内置登录，默认仅在宿主机回环地址发布，并只接受 `localhost`、`127.0.0.1`、`::1` Host。远程服务器可通过 SSH 转发访问：

```sh
ssh -L 8080:127.0.0.1:8080 user@server
```

多人远程访问时，在前方配置带认证的 HTTPS 反向代理；在 `.env` 添加 `PANEL_ALLOWED_HOSTS=health.example.com`（多个主机名用逗号分隔，不带协议或端口），并让代理保留请求 Host。

## 2. 探查和切换语义

- 通过 Clash API 的 `Authorization: Bearer <secret>` 认证，由 Mihomo 发起真实代理探查；Go 控制器不会绕过代理直接访问站点。
- 每个调度周期只从嵌入的 200 个站点中随机抽取一个，对目标 select 分组当前出口执行 `/proxies/{group}/delay`，预期状态为 `200-299`。
- 成功清零失败计数；出口测试返回失败/超时并达到配置阈值后，立即调用 `/group/{group}/delay`，本轮使用相同的随机 URL。
- 整组测试结束后重新读取主分组 `all`，按该数组顺序查找本轮成功的首个成员，绝不按 delay 数值或 JSON map 顺序选择。缺项、0、负数、超出 uint16 范围均不视为可用。
- 首个可用分组已经在使用时不重复 PUT；切换请求失败后重新确认当前选择，再按原始顺序尝试后续可用项。全部失败则保留当前选择。
- API 认证错误、API 连接失败、无效配置/响应不会增加出口失败计数，也不会触发整组探查；面板展示错误。单出口 delay API 的 HTTP 503（测试失败）和 504（超时）视为出口测试失败；其他接口上的同类状态不会增加出口失败次数。
- 任务串行执行。慢请求或整组切换跨过时钟周期时跳过已错过的周期，不补发积压任务。修改配置/停用时取消旧任务并重置探查计数、延迟等当前运行状态，保留最近 1 小时的历史记录；已被 Mihomo 接收并执行的切换请求无法撤销。
- 正常探查仅验证当前国家分组出口，不递归控制其内部节点；国家分组的原有 fallback 自动探查继续工作，实际请求量可能高于常规探查频率。
- HTTP API 请求超时为配置的测试超时加 2 秒，留给 API 处理和传输；间隔和超时可以独立配置，过长探查可能导致跳过部分周期。

运行历史保存最近 **1 小时**的事件，与当前探查配置独立。面板每页显示最多 100 条，可点击“查看更早”翻页或“返回最新”恢复实时记录；后端接口 `GET /api/events?before={记录ID}&limit=100` 使用游标分页，`limit` 支持 1–100，不返回超过 1 小时的记录。修改配置、停用探查均保留当前内存历史。

面板提供“持久化历史记录”开关（API 字段 `persist_history`），**默认关闭**。关闭时只使用内存，启动不加载历史文件、运行及正常停止不写历史文件；重启后没有上一进程的内存记录。旧配置不包含该字段时同样按关闭处理。配置参数本身仍立即持久化，和历史记录开关独立。

开启后，后台每 **5 分钟**将当前内存历史批量保存到 `/data/history.json`（本地默认 `./data/history.json`），与配置共用已有数据卷，无需增加数据库。快照没有变化则跳过写入；写入失败会输出错误并在下一批次重试。查询只读取内存，不触发磁盘读写。配置保存的开启状态会保留，下次启动时恢复快照内仍有效的记录。

运行中开启不会立即读写历史文件，也不会合并之前忽略的磁盘快照，而是在下一批次保存当前内存；即使内存为空也会首次写入有效快照。关闭时保留当前内存和已有磁盘文件，后续批次及停止补写均跳过；已开始的磁盘批次可能完成。重新开启后按当前内存继续保存；只有启动时已开启才恢复旧快照。关闭开关不会主动删除已有文件。

过期数据在内存写入及查询时清理；开启持久化时，启动恢复和批量保存也清理过期项，磁盘快照在下一批次同步更新。即使停用探查，只要持久化开关开启，历史批量保存仍继续运行。使用同目录临时文件、同步写入后替换历史快照，保留记录序号和分页标识。

持久化开启时，正常停止（例如 `docker compose stop`、`restart` 或 `down`）会先停止探查，再额外保存一次未落盘历史；Compose 给控制器 45 秒停止宽限。强制杀进程、断电或写入故障可能丢失上次成功落盘后的记录，正常批次间隔下最多约 5 分钟。开启状态下启动遇到历史文件损坏会提示错误并停止，避免静默覆盖；关闭状态下不读取该文件。

Secret 不返回到浏览器；输入为空保留旧值，勾选“清除已保存的 Secret”后保存才清除。Secret 以访问权限受限的配置文件保存在数据卷，未做静态加密。

## 3. 本地开发与验证

安装 Go 1.26 或更新版本：

```sh
go test ./...
go vet ./...
go run .
```

本地默认面板 `http://127.0.0.1:8080`，数据目录 `./data`。环境变量 `LISTEN_ADDR` 修改监听地址，`DATA_DIR` 修改持久化目录。

```sh
go build -o mihomo-health .
./mihomo-health healthcheck
```

`healthcheck` 仅判断面板进程是否存活，不表示代理出口健康；出口状态见面板。使用模拟 Clash API 的测试无需真实订阅或外网；真实机场节点的联通性仍需部署后验证。

## 4. 维护

```sh
docker compose logs --tail=100 z-mihomo-health-check
docker compose pull
docker compose up -d
docker compose down
```

`down` 保留数据卷；不要在希望保留配置和历史时添加 `-v`。备份时保管 `z-mihomo-health-check-data` 卷中的 `config.json` 和 `history.json`；前者包含 API Secret。

遇到 8080 端口占用时，修改 `docker-compose.yml` 中宿主机端口。请在已有 Mihomo 服务正常运行后，通过面板测试 API 连接。

## 5. 从原笔记目录迁移

代码和订阅本地示例已移至 `E:\personal\code\z-mihomo-health-check`，原 `z-notes/mihomo-health-check` 只保留迁移说明。Go 模块已独立命名为 `z-mihomo-health-check`，运行和构建不依赖原笔记仓库。

若之前已部署过旧版容器，先正常停止旧服务并保留原数据卷，再用 `docker volume ls` 确认实际卷名。旧版可能使用 `mihomo-health_controller-data` 或 `mihomo-health-external_controller-data`。不要直接新建空卷后误以为旧配置或历史丢失。

选择复用旧卷时，将所选 Compose 文件底部的 `health-data` 定义改为实际旧卷名并声明外部卷，例如：

```yaml
volumes:
  health-data:
    external: true
    name: mihomo-health_controller-data
```

默认新卷名可通过 `.env` 的 `CONTROLLER_DATA_VOLUME` 调整。复用旧配置后检查面板中的 API 地址，改为健康检查容器通过 `wk` 网络能够访问的已有 Mihomo 服务地址，例如 `http://mihomo:9090`。

官方参考：[Mihomo API](https://wiki.metacubex.one/api/)、[Compose 网络模式](https://docs.docker.com/reference/compose-file/services/#network_mode)。
