# BL-DIR-002：修复 Seal 并发测试的调度竞速断言

## 目标

`TestSealDuringInflightKeepsAllTasks` 在本机（go1.20.14/windows）稳定失败（常规 9/10、GOMAXPROCS=1 5/5），消除其误报根源，恢复测试套件全绿。

## 当前状态

- 2026-11-14 排查闭合：孤立探针（seal 协程 50ms 先机 → 10/10 通过；原测试同批 5/5 失败）证明失败纯属协程起跑竞速，非生产缺陷。瞬时断言改为有界等待（waitSealedFor，2s），验证待跑完收尾。

## 问题定位（证据链）

1. 失败点：dirscan_test.go 消费者协程 `t.Error("Seal 应已置位")`——消费到第 2 个任务时 `dirSched.sealed` 仍为 false，0.00s 即失败。
2. 代码事实：`SealAndDrain`（dirscan.go:411-412）置位 `sealed` 与消费者链路（complete→wakeup→调度器再发射 task2→消费）之间**无任何 happens-before 边**，纯调度竞速；单站点在途≤1 的节奏使"消费到 task2"必然经过两次协程切换，但 seal 协程（新起 goroutine + Once 检查 + 锁）不保证更快。
3. git 事实：dirscan_test.go 自 49dfbfa（Mode F 验收前最后一笔）后未改——验收时同代码通过、今天稳定失败，属环境敏感的潜在测试缺陷爆发。
4. 探针实验（唯一变量 = seal 协程 50ms 起跑先机）：探针 10/10 通过 vs 原测试 5/5 失败 → 根因确证为起跑竞速。
5. 生产无损：`sealed` 只约束"拒绝新注册"与"排空后 close(taskCh)"，置位早晚与在途任务消费无关；所有失败运行中 `processed==4` 核心不变量均通过。

## 方案

瞬时断言改为有界等待：消费者在 processed==2（task2 在途、未 complete）时轮询等待 `sealed` 置位（≤2s，`waitSealedFor` 辅助函数，持锁读）。无死锁：SealAndDrain 置位 sealed 不依赖消费者推进。测试意图不变——"Seal 与在途任务并发生效且不丢任务"。

## 风险

- 无生产代码改动；仅测试断言语义从"瞬时"放宽为"有界等待内"，不丢任务断言保持原样。

## 验证方式

- 修复后 `-count=20`、`-race -count=5`、`GOMAXPROCS=1 -count=10` 全过；全量 `go test ./Plugins ./common` 全绿（该测试为唯一失败项）。

## 决策记录

- 保留并发行意图而非删除断言：直接删会丢失"Seal 在有在途任务时置位"这一行为覆盖；有界等待是无同步假设下能测到的最强形式。
