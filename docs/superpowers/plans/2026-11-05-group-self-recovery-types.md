# select 与 url-test 组内自恢复实施计划

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans; implement and test inline on the current branch.

**Goal:** 连续失败到阈值后优先尝试当前国家组自恢复，增加 Selector/URLTest；保留已有 Fallback 流程。
**Architecture:** 通用自恢复编排负责健康配置、主组/子组保护和随机出口复验。Selector/URLTest 的直接成员用受控并发的单节点测试更新健康记录，避免 URLTest 整组 API 自动解除固定；只依据该 URL 的健康记录选择候选。Selector 仅当前成员确认失败后按 All 原序 PUT；URLTest 不 PUT/DELETE，等待其内部约 10 秒选路缓存后再读结果。
**Tech Stack:** Go 标准库、现有 HTTP 假服务和 Go/Web 回归。

## 已确认范围与默认选择

- 主组始终是 Selector；新增支持的是主组当前选中的国家组。
- Fallback 保持既有配置 URL/expected 整组探查，已有手动 fixed 时不主动解除固定；组内固定或配置在期间变化则停止旧流程。
- Selector 有测试 URL 用配置，没有则随机选有效池中非原失败 URL，expected 默认200-299。URLTest 使用自己 URL/expected，缺失则跳过优化。
- Selector/URLTest 成员测试并发上限8。按用户后续要求，A 组自恢复的 Fallback 整组 delay 及 Selector/URLTest 单成员 delay 固定5秒、HTTP7秒，独立于实例及订阅超时；常规探查、出口复验、主组整组 delay 保持实例配置。批次总耗时可能更长。节点状态采用 extra[URL] 的实际健康结果，不能只信正延迟。
- URLTest 已 fixed 时不清除人工固定，跳过优化；未 fixed 时只更新健康，按其自动选择/容差；当前缓存仍指向坏节点且其他成员可用时，最多等待10秒再读、可被配置变化取消。
- 复验随机抽取其他有效URL，不复用原失败URL，排除隔离项。成功保留国家组，原失败URL独立计数不因另URL成功而清零。
- 检查主组选择/All及国家组Type/All/testUrl/expected/fixed；Selector还检查Now，URLTest/Fallback允许自动Now变化。手动变化、配置更新、控制错误都停止旧流程；API无条件写能力，GET/PUT间极短竞争不能原子排除。
- 不新分支/worktree，不自动提交推送，不调用真实Mihomo。

## Task 1: 成员探查与select

- [x] 先写失败测试：仅当前故障切换、按原序不按延迟、当前健康不切、无URL随机、API正延迟但expected不满足不选、主/子手动变化、取消。
- [x] 实现 ProbeGroupExpected、指定组SelectGroup、带URL健康校验的并发成员探查；实现 Selector 条件选择和确认。

## Task 2: URLTest及通用保护

- [x] 先写失败测试：自动Now变化允许、全程无PUT/DELETE/组整组强制解固定、已有/并发fixed保护、缓存等待取消、timeout参数继承。
- [x] 实现 URLTest自动选路及可取消缓存等待；复用随机复验，扩展Fallback组内保护。

## Task 3: 文档、验证和独立审查

- [x] 更新README/DEPLOY类型矩阵、超时和缓存说明。
- [x] 全量Go测试、vet、30项Web及diff检查，独立只读审查并修复实质问题。

## Execution ledger

开始于 a8473e9。用户明确授权按建议扩展，不额外要求设计确认。保持原有国家组切换优先级与隔离机制；前端无新增轮询。race环境此前缺少CGo编译器，若条件未变不声称通过race。

完成：新增类型测试先观测失败后实现通过；独立审查发现主分组测速阶段遗漏组内保护，新增回归先复现后修复并复审通过。最终 go test ./... -count=1、go vet ./...、Web 30 项及 git diff --check 通过。仅本地假服务验证，未访问真实 Mihomo；未提交或推送。
