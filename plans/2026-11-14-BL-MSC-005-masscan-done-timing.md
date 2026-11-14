# BL-MSC-005：masscan done 统计行提前到 masscan 退出时打印

## 目标

`[*] masscan done: found=N, cost=X` 在 masscan 进程退出时即打印（下游 fscanx 扫描继续跑），cost 改为 masscan 自身耗时；不再等整轮任务结束才出现。

## 当前状态

- 2026-11-14 实现中。

## 问题

`runMasscanTargets` 尾部：`ScanFromReader`（阻塞至整轮下游扫描收尾）→ `stderrWG.Wait` → `cmd.Wait` → 才打印 done 行。与 BL-MSC-003 的 rate 行残留同类生命周期缺陷；模块文档「启动流程 step 5」写的是"masscan 结束后……打印统计"，实现违背。且 `cost=time.Since(start)` 把下游扫描时间也算进 masscan 耗时，数值误导。

## 方案

- `ScanFromReader` 移入 goroutine（`scanDone` 通道收口）；主路径 `stderrWG.Wait()` + `cmd.Wait()`（= masscan 真正退出）后即取 found、打印 done 行。
- `masscanCountingReader.count` 改 `atomic.Int64`：masscan 退出时主协程读的是快照（读侧在扫描协程，无锁）；`status.Found` 在 stderr EOF 时已终值、权威。快照可能漏末尾几行（仅无 stderr 状态行的修改版 masscan 的兜底路径），统计行非结果数据，可接受。
- 三条返回路径（interrupted / waitErr / 正常）都先 `<-scanDone` 并入扫描协程再返回——否则 main 的 `LogWG.Wait()`/`close(Results)` 与扫描协程的 `LogWG.Add`/`Results<-` 竞争，可能 close 后 send panic。
- done 行中途打印的日志通道安全性：`LogSuccess` 调用即 `LogWG.Add`（log.go:41）+ 同步 send，main 后续 `LogWG.Wait()` 兜底，无风险。

## 风险

- found 快照偏低（仅兜底路径、末尾几行）：注释载明。
- 输出顺序变化：done 行混入下游插件输出流中间（用户预期行为）。

## 验证方式

- 全量 `go test ./Plugins ./common` + race；build/vet/gofmt。
- 冒烟 `-msc -h 127.0.0.1 -p 445`：done 行 cost ≈ masscan 时长（~10s 量级）、明显小于总 `scan done` cost；done 行出现在输出流中段而非末尾。
- 真机由用户验收。

## 决策记录

- 2026-11-14 用户报告"done 在任务结束才打印"；定案与 BL-MSC-003 同构（stderr/管道 EOF = masscan 退出为唯一时点）。
