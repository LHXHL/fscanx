# BL-MSC-002 -msc Ctrl+C 退出可靠性

日期：2026-11-13
状态：已完成（用户真机测试通过）

## 目标

让 `-msc` 的 Ctrl+C 行为可预期：第一次信号停止 masscan 并有界等待在飞扫描收尾；超时或第二次信号立即清理临时文件并以 130 退出。正常完成路径不变。

## 根因

1. `Plugins/masscan.go` 的信号处理协程在第一次 Ctrl+C 后调用 `cancel()`；`ctx.Done()` 随即关闭，协程在第二个 `select` 中命中 `<-ctx.Done()` 退出。`signal.Notify` 仍生效，但无人读取 `sigCh`，后续 Ctrl+C 被吞掉。
2. masscan 停止后，`ScanFromReader` 依次执行 `LogWG.Wait()`、`close(targetInputCh)`、`PoolScan.StopAndWait()`，没有等待上限；慢扫描会长时间阻塞退出。

## 执行计划

1. 新增 `Plugins/masscan_interrupt.go`：`waitMasscanInterrupt` / `startMasscanInterrupt` 两段式 helper，支持第一次信号、第二次信号、计时到期、正常完成四个分支。
2. 修改 `Plugins/masscan.go`：signal 缓冲区改为 2；第一次信号 cancel masscan 并启动 3 秒计时；第二次或超时执行统一清理并 `os.Exit(130)`；正常完成通过 done 通道停止 helper；返回中断哨兵。
3. 修改 `main.go`：识别中断哨兵，以 130 退出且不打印 “masscan mode failed”。
4. 新增 `Plugins/masscan_interrupt_test.go`，覆盖四个分支。
5. 验证：`go test ./Plugins ./common`、`go build .`；reviewer 独立审查；回归 `-msc` 原有单测。
6. 同步 backlog、控制台、模块文档、README、CHANGELOG；跑 `check-sync.py`。

## 风险

- 信号测试不能真实调用 `os.Exit`，helper 用回调注入；真机 Ctrl+C 仍需用户验收。
- 3 秒为固定 grace；若后续需要按场景调整，再评估新增参数。
- 强制退出可能丢失在飞日志，属预期；正常收尾路径仍会等待日志。

## 验证方式

- 单测：helper 四分支（first/second/timeout/done）。
- 构建与回归：`go build .`、`go test ./Plugins ./common`。
- 文档：`python .claude/skills/project-control-plane/scripts/check-sync.py`。
- 真机：用户在实际终端按一次/两次 Ctrl+C，确认退出时间与临时文件清理。

## 决策记录

- 用户 2026-11-13 选择“有界收尾 + 二次强退”，不采用第一次立即强退或仅修复第二次信号的方案。
- 不新增命令行参数，3 秒 grace 固定。
- 不改 `--std` 的中断行为。

## 完成记录

- 2026-11-13：实现完成。新增 `Plugins/masscan_interrupt.go` 与单测；`Plugins/masscan.go` 接入两段式中断并返回中断哨兵；`main.go` 对中断退出 130。
- 已验证：`go build .`、`go test ./Plugins ./common`、`go test -race -run TestWaitMasscanInterrupt ./Plugins`、`go vet ./Plugins ./common` 全部通过。
- 2026-11-13：用户真机测试确认 Ctrl+C 行为（第一次有界收尾、第二次强退），BL-MSC-002 验收通过。
