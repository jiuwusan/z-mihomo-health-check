# 项目架构

这是 Go 标准库实现的多实例 Mihomo 监控面板。主进程与各实例状态机、站点隔离池、文件持久化和出站协议已分离，继续保持原 HTTP 与磁盘格式。

## 目录与职责

- main.go：读取启动环境、嵌入 sites.txt、校验站点池、组装 manager/HTTP、信号关闭与 healthcheck 子命令。
- internal/app：多实例组装根；实例注册、配置/历史/站点池组装、各实例循环启动/停止、删除时取消请求与等待。只有这里组装 service 与 HTTP adapter。实例注册表变更仍在此协调，加载/编码/原子文件写入由 repository 执行。
- internal/httpapi：路由、请求 JSON/Origin/内容类型限制、安全响应头、秘密字段输入与脱敏展示、错误状态码与嵌入静态页面。通过 Monitoring/Instances 端口调用服务，不导入具体 service、gateway 或 repository。服务端控制器层与 net/http 路由同包，避免空转发包装。
- internal/service：Controller 是监控状态机（不是 HTTP controller）；负责选择/恢复/基线重检、rev 锁/状态版本、事件保留/分页、站点池转换和通知时机。设置用例先持久化再启用配置；连接测试通过 gateway；HistoryStore 负责快照/去重/定时保存策略，实际文件读取和原子写入在 repository。
- internal/gateway：Mihomo 协议、认证、超时、响应体限制、元数据解码、直接成员有限并发探查、Webhook 渠道发送；不决定恢复频率或事件持久化。
- internal/repository：配置、历史、站点池 JSON 加载/格式校验和权限/临时文件/fsync/原子替换。
- internal/config：纯配置 DTO、默认值及字段/URL 校验，不读取环境或磁盘。
- internal/domain：事件/状态/恢复/代理/站点状态与实例错误值对象，无磁盘和网络依赖。EventSequence 显式禁止 JSON 输出，保持 API 不暴露内部序号。
- internal/httpapi/web：原浏览器 app.js/HTML/CSS；embed 的相对 web/* 和公开资源 URL 均保持原状。

## 依赖方向

main/app → HTTP/service/config/repository；HTTP → config/domain 与服务端口；service → config/domain/repository/gateway；repository → config/domain；gateway → config/domain；config/domain 不依赖上层。实际 import 约束由 internal/architecture 测试执行，Go 编译器拒绝循环。测试允许跨层集成，但运行源码不允许借测试依赖绕过层次。

## 状态与兼容

实例具有独立配置/事件/恢复状态，共享一个带锁的站点池；通过 rev/currentVersion 防止旧请求覆盖新配置或人工选择。调用 Mihomo 与发送通知均在状态锁外，设置保存必须先完成文件写入，失败不能激活草稿。删除实例先取消生命周期，等待探查与历史 flush 后再移除；不能复活已经停止的实例。

保留 /healthz、/api/instances 及每实例操作的路由、方法、状态码、JSON、Secret/Webhook 保留/清除语义、历史筛选与分页；通知时机、失败隔离、站点连续失败阈值和租约不变。旧根目录 config/history 不自动合并进入新实例，数据格式保持原版本。
