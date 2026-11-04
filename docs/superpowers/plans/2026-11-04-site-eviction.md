# 全局站点临时隔离 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [x]`) syntax for tracking.

**Goal:** 连续失败 20 次后按需复核、临时隔离并自动恢复，所有实例共享并持久化。
**Architecture:** SitePool 管理线程安全的站点状态与原子存储，Controller 负责复核/复测的 HTTP 调用，InstanceManager 共享池，Status/Web 展示动态数量。
**Tech Stack:** Go 标准库、原生 JavaScript、现有测试工具。
**Spec:** ../specs/2026-11-04-site-eviction-design.md

## Global Constraints

- 当前分支直接修改，不建分支/worktree，不自动提交或推送。
- 阈值 20，复核冷却 30 分钟，隔离重试 1 小时，全局复测节流 1 分钟。
- 只统计常规探查，控制错误/取消不计数，复核不阻塞原故障切换。
- 全局状态 DATA_DIR/site-pool.json，写失败不发布，空池仍复测。

## Review Focus

- 跨实例成功不得被旧复核覆盖；租约与版本令牌都需校验。
- 旧配置/取消不能提交结果，也不能泄漏复测租约。
- 池为空不死锁、不随机下标越界，可在出口恢复后复活。
- 损坏持久化文件与保存失败不能悄悄复原隔离项。
- 所有抽样（含恢复检查）必须排除隔离项。

## Task 1: 共享池和持久化

Files: site_pool.go, site_pool_test.go。
Interfaces: OpenSitePool(sites, filename), Pick(exclude, limit), Record(site, success, now), FinishReview(site, token, quarantine, now), ClaimRetry(now), FinishRetry(site, token, success, nodeFailure, now), Counts()。
- [x] 写失败测试验证 19/20、成功清零、冷却、复核令牌失效、隔离与恢复节流、持久化和写失败、并发。
- [x] 运行失败测试，再实现共享池；测试通过。

## Task 2: 控制器和实例集成

Files: controller.go, site_checks.go, recovery.go, instances.go, site_checks_test.go, instances_test.go。
- [x] 写控制器集成失败测试：正常失败不加测、20 次才加测、坏出口不隔离、坏站点隔离、全局跨实例和重启、旧请求与空池恢复。
- [x] 实现正常结果计数、故障转移后复核、隔离复测、共享池注入及恢复抽样；通过相关回归。

## Task 3: 展示、文档和完整验证

Files: web/app.js, web/index.html, web_test.cjs, README.md, DEPLOY.md, SITES.md。
- [x] 写 Web 测试验证 0 仍显示数字及全局隔离数，更新展示与文档。
- [x] 运行 go test ./...、go test -race ./...、go vet ./...、node --test web_test.cjs、node --check web/app.js、git diff --check。
- [x] 独立审查完整改动，修复后重跑必要测试；不自动提交。

## Execution ledger

User confirmed revised design after rejecting per-failure checks. Default timing constants chosen as conservative implementation details. Existing static 470-site pool stays unchanged. Work on current master as explicitly required by user AGENTS instructions; no worktree. No commit until requested.

Task 1: complete — 19/20 boundary, reset/cooldown, shared concurrency, pending counts and isolation reload, write failure and retry pacing covered.
Task 2: complete — immediate normal-result recording; only review network deferred after failover; shared manager injection; recovery sampling and empty-pool revival covered.
Task 3: complete — dynamic active/total/quarantined metrics, zero display, README/DEPLOY/SITES updated; Go suite, vet, build, 22 Web checks, JS syntax and diff check pass.
Review: independent reviewer identified delayed success, same-direction failure invalidation, cooldown lost on success, and cancelled retry lease (including cancellation at claim return). Each regression failed before its fix and now passes.
Ruling: preserve cooldown across successful counter resets; only success/new review/state transitions invalidate review epochs. Failed observations cannot contradict a pending review.
Ruling: cancelled retries release transient leases without persisting old results; global one-minute pacing remains in force.
Validation limitation: go test -race could not run because CGO_ENABLED=0 and no gcc/clang compiler is installed. This is not recorded as a passing race check.
No real Mihomo services or production data modified during implementation/tests. Static sites.txt/sites.json/probe-results.json unchanged. No commit or push performed.