# BL-DIR-003：dirscan worker 改 -tn 控制 + abort/progress 不落盘

## 目标

1. 目录扫描 worker 数量由 `-tn`（WebScanThreadNum）控制，与 web 扫描并发一致。
2. `[-] dirscan abort` 与 `[*] dirscan progress` 行仅打印终端，不写入 result.txt（量大）；`-silent` 时完全静默。

## 当前状态

- 2026-11-14 实现完成并验证：本机 slow-abort 冒烟实证 abort 行独立成行打终端、result.txt 零 abort/progress；全量测试绿。

## 方案

- worker 数：新增 `dirWorkerCount()`（读 `common.WebScanThreadNum`，≤0 回退 16——单测未跑 flag 时为零值，回退保证测试行为与历史一致且防 0 worker 卡死排空）；spawn 循环改用它。taskCh 缓冲容量维持 16 不随动（包级 var 初始化早于 flag.Parse，且调度器单协程、缓冲深度只影响发送阻塞时机）。
- 日志路由：两处 abort 与一处 progress 由 `common.LogSuccess`（进 Results→result.txt+终端）改 `common.Printf`（仅终端）+ `!common.Silent` 守卫（保持 -silent 静默语义；原 LogSuccess 在 -silent 下不打印终端但**仍写文件**，改后 -silent 时两者皆无）。
- `dirscan enabled`/`dirscan done`（每轮各一行）保留落盘；Product 聚合行落盘不变。

## 风险

- -tn 默认 60 > 原 16：目录扫描全局并发上限提高（单站点在途≤1 门闩不变，实际并发仍 ≤ min(tn, 站点数)）；用户明示要求一致。
- Printf 不自动补换行（LogSuccess 经 SaveLog 补）——首版漏 `\n` 已被冒烟抓到（abort 行与 done 行连排），三处补 `\n` 后复验独立成行。

## 验证方式

- 单测：dirscan 全组（TestDir*/TestSeal*/TestScheduler/TestAggregator/TestWorkerPanic）+ 全量 `go test ./Plugins ./common` 绿；gofmt/vet/build 绿。
- e2e 冒烟（本机 python http.server）：快扫（70 路径 404）→ result.txt 有 enabled/done、无 progress/abort；-silent 全静默。慢站点（根快、路径 4s）→ slow-abort 触发，终端独立成行 `[-] dirscan abort ... 3 consecutive slow paths (>2s), skip 67 paths`，result.txt 计 0。

## 决策记录

- 2026-11-14 用户两指令：worker 由 -tn 控制保持一致；abort/progress 仅打印不写 result.txt（"太多了"）。
- taskCh 容量不随 -tn 动：包级初始化早于 flag.Parse 取不到值，懒建引入 nil 风险不值当——容量只影响调度器阻塞时机，与吞吐无关。
