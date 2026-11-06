# 基准出口复验实施计划

> **For agentic workers:** Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** 随机探测连续失败达到实例阈值后，先对当前国家分组执行最多三次基准出口复验。

**Architecture:** 在 Controller.Check 达到阈值后、现有组内自恢复前增加串行复验。使用当前成员的 testUrl / expectedStatus；未配置 URL 时使用 http://www.gstatic.com/generate_204，预期 204。配置 URL 的 Selector 缺少 expectedStatus 时沿用项目默认 200-299，自动组空 expected 原样传递。单次超时沿用实例探查超时，不额外等待。

**Tech Stack:** Go，现有 Mihomo HTTP 客户端与 httptest。

**Spec:** 本计划中的行为约定对应用户已选方案一及最多三次连续复验要求。

## Global Constraints

- 任一次成功即保留分组、清零出口失败计数；三次全部出口失败后才继续原恢复流程。
- 基准复验不增加或清零站点池独立计数；原始失败站点仍供后续主分组测试使用。
- 控制接口异常、取消、主选择/顺序和组内配置/固定状态变化停止本轮；不解除固定，不主动切换。
- 在当前分支实施；提交与推送按用户后续明确指令执行，不修改线上配置。

## Review Focus

- 第一、第二、第三次成功均应立即结束，不额外测试或切换。
- 三次失败后顺序进入组内自恢复、随机复验及必要的主组测试。
- 缺少 URL 的默认地址、预期状态与实例超时须准确传递。
- 认证错误与无效响应不能当作出口故障累计重试。
- 手动换组、元数据变更和配置取消不得继续旧任务。

## Task 1: 基准复验与回归验证

- [x] 添加 baseline_recheck_test.go，覆盖阈值、三次内成功、全部失败、默认地址、错误、变化与取消；运行并确认新测试失败。
- [x] 新增 baseline_recheck.go 并在 controller.go 达到阈值处调用；沿用 checkedInnerGroup 保护选择和元数据。
- [x] 调整既有 HTTP 测试服务，明确提供基准失败响应，使旧测试继续验证原恢复流程。
- [x] 更新 README.md / DEPLOY.md；运行 go test ./...、go vet ./...、node --test web_test.cjs 和 git diff --check，审阅变更。

## 验证结果

- 新增核心回归测试先失败，接入实现后通过；完整 `go test ./... -count=1` 与 `go vet ./...` 通过。
- `node --test web_test.cjs`：30 项通过；已跟踪与新增 Go 文件的差异空白检查通过。
- 独立代码审查未发现需要修正的问题。
- `go test -race ./... -count=1` 无法执行：本机 CGO_ENABLED=0，且未发现 gcc/clang；未修改全局环境或安装依赖。
- 在当前分支完成实现；提交与推送按用户后续明确指令执行，未直接部署服务。
