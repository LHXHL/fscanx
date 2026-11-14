# BL-MSC-004：-msc -auto 组合（探活复用原逻辑，masscan 接手端口扫描）

## 目标

`-msc -auto` 可组合：-auto 大网段探活维持原逻辑产出存活 C 段，masscan 以 `-iL` 消费留存于 `.msc/` 的目标文件完成端口扫描，结果进现行 -msc 下游管线。

## 当前状态

- 2026-11-14 方案定稿并获用户批准（三决策：.msc/ 时间戳名；0 存活提示后退出 0；仅 -auto 组合留存）。实现中。

## 方案（定稿八决策）

1. 探活原样调用 `AutoScanBigCidr`，只消费返回值（`a.b.c.0/24` 列表，恰为 masscan -iL 格式，零转换）；不支持网段（返回 nil）→ 报错非零退出。
2. 组合后跳过现行 -auto 后续的"存活 C 段全 IP ICMP 扫描"阶段（masscan SYN 扫覆盖 IP 级+端口发现）；`-np` 在此组合无作用。
3. 入口拆分：`ScanWithMasscan` 拆出核心 `runMasscanTargets(targetsFile, targetCount, keepTargets)`；新增 `ScanWithMasscanAuto(Info)`；main.go 三行分支。
4. 留存文件：`.msc/msc-targets-<yyyyMMdd_HHmmss>.txt`（同秒冲突 `-2` 后缀），目录懒创建（仅此组合），不自动清理，不写注释行；终端打印路径与 C 段数。
5. 中断清理（cleanupInterrupted）排除留存文件；普通 -msc 临时文件行为不变（用后即删）。
6. 校验矩阵：删 `-msc`×`-auto` 互斥；新增 `-msc -auto` 必须 `-h`、拒绝 `-hf`（探活目标只来自 -h，静默忽略不如显式报错）；0 存活 → 提示退出 0（合法扫描结果）。
7. 探活后释放 BloomFilter（该路径无后续 ICMP 阶段）并 GC（对照 scanner.go:340-343）。
8. `-mr` 默认 600 不改，README 提示大网段建议调高。

## 风险

- 探活阶段（masscan 启动前）无两段式 Ctrl+C——与普通 -auto 一致，维持原逻辑的代价。
- /8 极端下目标文件最多 65536 行（MB 级内），masscan 可承受。
- ValidateMasscanMode 含 os.Exit 不可单测——校验矩阵靠二进制手动验证（拒绝路径均快速安全退出）。

## 验证方式

- 单测：留存写入器（t.Chdir→t.TempDir 下验证目录懒创建/内容逐行/同秒 `-2` 命名）。
- 手动：`-msc -auto`（无 -h）拒绝；`-msc -auto -hf x` 拒绝；`-msc -auto -h 1.2.3.4`（不支持网段）非零退出；合法组合进入探活提示。
- 全量 `go build`/`go vet`/`go test ./Plugins ./common` + 新增用例 race。
- Reviewer（sonnet）独立验证 + deslop。
- 真机：`-msc -auto -h <真实网段>`（验收点：留存文件行数=存活 C 段数、masscan 正常消费、Ctrl+C 不删留存文件）——留用户验收。

## 决策记录

- 2026-11-14 用户三拍板：目录 `.msc/`（否决 .fscanx/）；0 存活提示退出 0（否决写空文件）；仅 -auto 组合留存（否决全部 -msc 留存）。
- 探活不动 AutoScanBigCidr 一行（用户明示"维持原先逻辑"）；组合语义=masscan 只接手探活后的端口扫描。
