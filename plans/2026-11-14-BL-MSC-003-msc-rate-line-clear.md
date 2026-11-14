# BL-MSC-003：-msc 结束后清除底部 rate 进度行

## 目标

masscan 进程退出后立即擦除底部 `[masscan] rate: ...` 进度行，不让它在下游 fscanx 扫描的整个过程中残留。

## 当前状态

- 2026-11-14 实现完成：stderr 解析协程读尽（masscan 退出）即调用 `common.StopProgress()` 擦除停用进度行。build/vet/单测通过，真机视觉待用户下次 `-msc` 实跑确认。

## 问题

`ScanWithMasscan` 原实现（Plugins/masscan.go）：

```
ScanFromReader(counted, "masscan")   // 阻塞到整轮下游扫描（端口探测+插件+web）结束
stderrWG.Wait()
waitErr := cmd.Wait()
common.StopProgress()                // ← 才擦进度行
```

`ScanFromReader` 会等 `LogWG.Wait()` + 工作池排空才返回，期间输出协调层（common/progress.go `withProgress`）每打一条日志都会重绘 `progressText`——即 masscan 最后一帧 `rate: x-kpps, 100% done`。结果：rate 行在整个下游扫描期间"钉"在终端底部，与模块文档「启动流程 step 5：masscan 结束后……清掉进度行」不符。

## 方案

stderr 管道读尽（EOF）＝ masscan 进程退出（子进程退出才关闭管道写端）。在 stderr 解析协程末尾直接 `common.StopProgress()`：擦除 + `progressEnabled=false`，此后 `withProgress` 不再重绘，下游日志正常输出、无残行。

原 196 行的 `common.StopProgress()` 保留为幂等兜底；中断路径（第一次 Ctrl+C 的 `TryStopProgress`）不受影响。

## 风险

- stderr EOF 早于进程真正退出？仅当 masscan 主动关 stderr（不发生）；管道 EOF 即写端全关。
- 停用后下游无任何进度行：符合预期（fscanx 本就不做自身进度条，模块文档「不做」已声明）。

## 验证方式

- `go build .`、`go vet`、`go test ./Plugins ./common` 回归（进度层单测已在 BL-MSC-001 覆盖擦除/重绘字节序列）。
- 真机：`-msc` 实跑，确认 masscan 到 100% 退出后 rate 行消失、下游扫描日志底部干净。

## 决策记录

- 擦除时机取 stderr EOF（进程退出）而非"100% done"状态行：masscan 退出前还有 wait 重传阶段，保留该行有信息量；EOF 时机可靠且实现最小。
