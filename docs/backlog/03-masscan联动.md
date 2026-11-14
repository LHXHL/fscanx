# Backlog：masscan 联动

masscan 联动模块于 2026-11-13 立项，目标是把 `masscan | fscanx --std` 的两进程输出收敛到一个终端：fscanx 自己启动 masscan、流式消费结果并在底部绘制进度。方案经两轮独立评审后实现，用户真机验收通过。

## 状态汇总

4 item：4 已完成 / 0 进行中 / 0 未开始；4 已验收

| ID | 标题 | 优先级 | 状态 | 验收状态 |
|----|------|--------|------|----------|
| BL-MSC-001 | 新增 masscan 内嵌联动模式 | P1 | 已完成 | 已验收 |
| BL-MSC-002 | 修复 -msc Ctrl+C 退出不可靠 | P1 | 已完成 | 已验收 |
| BL-MSC-003 | -msc 结束后清除底部 rate 进度行 | P2 | 已完成 | 已验收 |
| BL-MSC-004 | -msc -auto 组合（探活复用原逻辑，masscan 接手端口扫描） | P1 | 已完成 | 已验收 |

## BL-MSC-001 新增 masscan 内嵌联动模式

- 模块：masscan 联动
- 优先级：P1
- 状态：已完成
- 当前阶段：持续迭代
- 价值：一条命令完成 masscan 高速发现和 fscanx 深度探测，终端只保留 fscanx 输出与底部 masscan 进度，消除两个进程抢标准输出造成的字符混杂。
- 关联模块文档：docs/modules/03-masscan联动模块.md
- 关联计划：plans/2026-11-13-BL-MSC-001-masscan-mode.md
- 关联代码：common/flag.go、common/config.go、common/log.go、common/ParsePort.go、common/masscan_flag.go、common/progress.go、main.go、Plugins/scanner.go、Plugins/masscan.go、Plugins/portscan.go、Plugins/smb.go、Plugins/ms17010.go、Plugins/redis.go、Plugins/rdp.go、Plugins/webScanTask.go、Plugins/wmiexec.go
- 验收状态：已验收

### 方案

- **做什么**：新增 `-msc` 运行模式。fscanx 自己查找并启动 masscan，实时读取 masscan 的结果和进度，把结果送进现有 `--std` 扫描管线，把进度画在终端最底部；屏幕上不再出现 masscan 的原始输出。
- **为什么**：现在用 `masscan ... | fscanx --std` 时，两个程序同时往终端写字符，输出会交错成一团。靠改 masscan 的进度打印间隔只能缓解，不能根治，因为终端始终有两个写者。让 fscanx 独占终端写权限，才能稳定呈现。
- **怎么做**：
  - **参数与校验**：新增 `-msc`（开启模式）、`-mr`（masscan 发包速率，默认 600）、`-mp`（手动指定 masscan 程序路径）、`-margs`（额外 masscan 参数，按空格拆分后追加）。`-msc` 与 `-std` 互斥；`-msc` 必须有 `-h` 或 `-hf`；`-msc` 同时给 `-dp`、`-u` 或 `-uf` 时直接报错（masscan 模式的目标只来自 `-h/-hf`，且 `-dp` 会在 `common.Parse` 里把域名移出 Host）；`-margs` 里出现 `-p`、`--ports`、`--rate`、`-iL`、`--includefile`、`--exclude`、`--excludefile` 等托管参数时直接报错。校验放在 `common.Parse` 之前执行并以非零码退出，避免 `ParseInput` 先以 0 退出。
  - **目标转换**：把 `-h` 按逗号拆开；`-hf` 逐行读取后再按逗号拆成 token。URL 行取主机名，`ip:port` 行去掉端口，masscan 结果行取 IP。普通 IP、CIDR、完整区间（如 `192.168.1.1-192.168.1.5`）原样保留；只把末段简写（如 `192.168.1.1-255`）展开成完整区间。任何非 IP/CIDR/区间输入都按域名处理，解析出全部 IPv4（去重）；没有 A 记录则报错。`-hf` 只读，不调用会改写用户文件的 `ReadInputFile`。最终目标写入系统临时目录下 `fscanx-masscan-` 前缀的文件，用 masscan `-iL` 读取；`-hn` 按同样规则写入排除文件，用 `--excludefile` 读取。正常返回、启动失败和收到 Ctrl+C/SIGTERM 时都要删除临时文件，用户 `-hf` 原文件不动。
  - **端口转换**：把 `PortsInput` 里的全角逗号换成半角，用 `common.ParsePort` 展开端口组名和区间得到端口集合，减去 `-pn` 排除的端口，再把连续端口压缩回区间（如 `1-65535` 仍是一个区间），最后传给 masscan `-p`。集合为空则报错。
  - **程序查找**：优先 `-mp`；其次系统 PATH（`masscan` / `masscan.exe`）；再查当前目录和 fscanx 可执行文件所在目录下所有以 `masscan` 开头的普通文件。按文件头判断 PE/ELF/Mach-O 及 CPU 架构，优先选与当前系统和架构匹配的；Linux 下还要求有可执行位。有多个候选时依次执行 `--version` 试跑，第一个成功且输出含 masscan 的胜出。全部失败则列出候选和失败原因后退出。
  - **运行与输出**：用 `exec.Command` 启动 masscan，stdout 管道逐行送入 `ScanFromReader`（由现有 `ScanFromStdin` 抽出，`--std` 行为不变），stderr 管道只用于解析状态行；两者都不直接继承终端。进度行写 stderr，用 `\r` 回到行首覆盖刷新（masscan 本身就是这个机制）；fscanx 每打印一条日志前先清掉进度行，打印后重绘。现有扫描路径里绕过日志通道的直接打印（smb、ms17010、redis、rdp、webScanTask、wmiexec、portscan 错误、scanner 启动行等）统一接入同一个输出协调层，避免撕裂进度行。非 TTY 或 `-silent` 时自动关闭进度。masscan 结束后打印一行统计（找到的开放端口数、耗时）；启动失败或非零退出时打印原始 stderr 末尾并让进程以非零码退出。
  - **派生初始化**：`-msc` 复用 `--std` 的并发模型（`-t` 控制全部并发），且不分配仅主扫描流程使用的 BloomFilter。实现时不把 `-msc` 写成 `ScanWithStdInput=true`（否则 main 会去读 os.Stdin）；新增独立条件 `usesStdinLikePipeline = -std || -msc`，只用于这两处派生初始化；main 中 `-msc` 分支先于 `-std`，并保证 `-msc` 不读 stdin。收到 Ctrl+C/SIGTERM 时杀掉 masscan 子进程、清掉进度行并删除临时文件后再以非零码退出。
- **为什么这么做**：不依赖 masscan 的 `-y` 选项，因为官方 1.3.2 没有该选项；解析标准输出 `Discovered open port ...`，同时天然兼容个人修改版的 `IP:PORT` 行。选流式而不是等 masscan 全部跑完再扫，是为了保留现有管道边发现边扫描的加速效果。进度写 stderr、结果写 stdout，这样把 stdout 重定向到文件时进度不会污染结果文件。CIDR 和完整区间不展开，避免 `10.0.0.0/8` 被枚举成上千万个目标；端口先展开再压缩，既支持端口组和 `-pn` 减法，又不会把 `1-65535` 拼成超长命令行。

### 范围

- 新增上述 4 个命令行参数和互斥/必填校验
- 新增 masscan 程序查找（PATH、当前目录、fscanx 所在目录、文件头识别、试跑兜底）
- 新增目标/端口格式转换和临时文件管理
- 新增 masscan 启动、stdout 流式解析、stderr 进度解析、进程退出与错误上报
- 新增终端进度行渲染，并把现有扫描路径的直接打印接入输出协调层
- 新增 `Plugins/masscan.go`、`common/progress.go` 及对应单测
- 把 `ScanFromStdin` 的扫描循环抽成可接收任意 `io.Reader` 的 `ScanFromReader`，`--std` 行为保持不变
- 模块文档、控制台、CHANGELOG 同步

### 不做

- 不做全屏 TUI，也不做 fscanx 自身的扫描进度条（只显示 masscan 进度）
- 不自动提权、不自动修改 masscan 权限、不自动改防火墙/杀毒放行
- 不映射 masscan 的全部参数（adapter、pcap、banners 等用 `-margs` 透传）
- 不改变 `--std` 已有的解析语义
- 不支持 IPv6（masscan 1.3.2 不支持）
- 不把 masscan 输出里的历史端口当作扫描端口；masscan 模式下端口来源始终是 `-p/-pn`

### 验收条件

- [x] `fscanx -msc -h 127.0.0.1 -p 445 -mr 1000 -mp ./masscan.exe` 能启动 masscan；终端不出现 masscan 的原始输出，fscanx 自身输出正常。
- [x] 单测用 `t.TempDir` 构造最小 MZ/ELF/Mach-O 文件头并注入目标 GOOS/GOARCH，验证会选中匹配项、错误架构不阻断正确候选；本机真实 `masscan.exe`/`masscan_linux` 仅作手工 e2e。
- [x] 目标转换：`-h 10.0.0.0/8` 生成一行 CIDR；`-h 192.168.1.1-255` 展开为完整区间；`-h localhost` 解析为 IPv4；`-hf` 文件转换前后哈希不变，URL/ip:port/行内逗号列表/域名行都能处理；`-hn` 生成排除文件。
- [x] 端口转换：`-p 1-65535` 只生成一个 `-p1-65535` 参数；`-p web` 能展开；`-pn 445` 从端口集合中减去 445；端口集合为空时明确报错。
- [x] 临时文件：正常退出、启动失败、模拟 SIGINT 三种路径结束后，系统临时目录里没有 `fscanx-masscan-*` 残留，用户 `-hf` 文件内容与修改时间不变。（正常退出/启动失败已实测无残留；SIGINT 用户已实测确认）
- [x] 输出协调：单测在进度激活时调用 `common.Println/Printf`（模拟 ms17010/smb 的直接打印），断言字节序列为“擦除进度行 → 日志 → 重绘进度行”；Windows 本机 445 开放时用假 masscan 实测插件路径，环境无 445 时记录跳过原因，不能空过。（单测与 445 插件路径已通过；真实 TTY 视觉用户已确认）
- [x] 把 stdout 重定向到文件或使用 `-silent` 时，结果文件里没有进度字符。
- [x] masscan 不存在、`-mp` 路径错误或启动失败（如 Windows 缺 Npcap）时，打印明确错误和 masscan stderr 末尾，退出码非 0。
- [x] 参数校验：`-msc -std`、`-msc` 无目标、`-msc -dp`、`-msc -u`、`-msc -uf`、`-margs "-p 80"` 都以非零码退出并打印明确错误。
- [x] `-msc` 在 stdin 关闭或含无关内容时仍只消费 masscan 管道，不读 stdin。
- [x] `go build .`、`go test ./common ./Plugins` 和新增单测通过；`--std` 回归测试通过。

### 依赖

- masscan 可执行文件由用户提供；Windows 需要 Npcap，Linux 需要 raw socket 权限（通常 root 或 CAP_NET_RAW）且文件要有可执行位
- 无新增第三方 Go 依赖

### 状态记录

- 2026-11-13：立项。方案经 B0 澄清确认：`-msc/-mr/-mp/-margs`、`-h/-hf/-hn` + 域名解析、流式 + 底部进度、新模块 03 + P1。
- 2026-11-13：第一轮独立评审修正：`-pn` 在 `--std` 管线本来不生效，改为端口集合相减；扫描路径存在绕过 SaveLog 的直接打印，纳入范围；`-hf` 不能复用会改写文件的 `ReadInputFile`；CIDR/完整区间不得展开；补 `-msc` 参数校验和端口压缩。
- 2026-11-13：第二轮独立评审修正：拒绝 `-msc` 与 `-dp/-u/-uf` 组合；托管参数禁止出现在 `-margs`；临时文件定义正常/失败/中断三种清理路径；`-hf` 每行再按逗号拆分；明确 `-msc` 不写成 `ScanWithStdInput=true`，派生初始化用独立条件；输出协调和临时文件补可证伪验收，避免 445 未开放时空过。
- 2026-11-13：实现完成。新增 `common/masscan_flag.go`、`common/progress.go`、`Plugins/masscan.go`；`ScanFromStdin` 抽出 `ScanFromReader`；main 增加 `-msc` 分支；扫描路径直接打印接入输出协调层。
- 2026-11-13：本地验证通过：`go build .`；`go test ./common ./Plugins`；新测试 `-race`；`go vet ./common ./Plugins`。假 masscan e2e：445 探测成功、found=1、stdout 无 masscan 原始输出、无临时文件残留；真实 masscan 1.3.2 e2e：exit 0、found=0、无原始输出；`-silent`/重定向无进度字符；`-hf` 文件哈希不变；参数拒绝组合均非零退出；`--std` 回归通过。
- 2026-11-13：待用户真机确认：真实终端下底部进度行的视觉效果、Ctrl+C 中断后的临时文件/子进程清理（本环境无法分配 TTY、难以可靠发送 SIGINT）。
- 2026-11-13：`check-sync.py` 通过（4 个 item / 3 个文件，无发现）；同步脚本的 item 编号正则已按用户批准适配本项目 `BL-001 / BL-MEM-001 / BL-MSC-001` 三种形态。
- 2026-11-13：独立代码审查（opus）结论 REVISE，已修复：失败/中断不再打印成功行；第二次 Ctrl+C 清理临时文件后强制退出；`--version` 试跑失败保留 stderr 尾部与候选原因；URL 大小写、`-hf` 行内注释/空格、完整区间起止顺序、参数错误输出到 stderr。审查确认核心链路（管道收尾、CIDR 不展开、端口相减/压缩、进度锁、`--std` 回归）无问题。
- 2026-11-13：用户真机验收通过（底部进度显示、Ctrl+C 清理），全部验收条件勾选，验收状态置已验收。
- 2026-11-13：文档体系整理时补充：实现还拒绝 `-msc -auto` 与 `-msc -m icmp`（masscan 模式不需要空闲目标探测、icmp 没有端口可扫），已同步模块文档；原方案节为立项时点记录，不回写。

## BL-MSC-002 修复 -msc Ctrl+C 退出不可靠

- 模块：masscan 联动
- 优先级：P1
- 状态：已完成
- 当前阶段：持续迭代
- 价值：让 `-msc` 的 Ctrl+C 行为可预期：第一次有界收尾，超时或第二次立即退出，避免信号被吞掉或长时间卡住。
- 关联模块文档：docs/modules/03-masscan联动模块.md
- 关联计划：plans/2026-11-13-BL-MSC-002-msc-interrupt.md
- 关联代码：Plugins/masscan.go、Plugins/masscan_interrupt.go、main.go
- 验收状态：已验收

### 方案

- **做什么**：修复 `-msc` 的 Ctrl+C 退出链路。第一次 Ctrl+C 停止 masscan，最多等 3 秒让在飞扫描收尾；超时或第二次 Ctrl+C 立即清理临时文件并以 130 退出。
- **为什么**：当前第一次 Ctrl+C 的取消操作会立即关闭 `ctx.Done()`，信号处理协程随之退出；`signal.Notify` 仍在生效，但已无人读取信号，后续 Ctrl+C 被吞掉，进程只能等所有在飞扫描结束。同时 `LogWG.Wait`/`PoolScan.StopAndWait` 没有等待上限，单个慢扫描会把退出拖到很久。
- **怎么做**：把信号处理抽成可测试的两段式 helper：第一次信号执行 cancel 并启动 3 秒计时；第二次信号、计时到期或外部完成信号三选一。前两者执行统一清理（停进度、删临时文件）并 `os.Exit(130)`，正常完成通过 done 通道让 helper 退出。`ScanWithMasscan` 返回中断哨兵错误，`main` 对中断以 130 退出且不打印 “masscan mode failed”。
- **为什么这么做**：保留“尽量让在飞结果落盘”的原意，同时给退出加硬上限；不新增命令行参数，不改变 `--std` 行为，也不尝试取消每个网络请求（改动面大、难验证）。

### 范围

- `Plugins/masscan.go`：接入两段式中断 helper、统一清理、返回中断哨兵
- 新增 `Plugins/masscan_interrupt.go`：可测试的中断等待逻辑
- `main.go`：中断退出码 130，不再打印失败信息
- 新增 `Plugins/masscan_interrupt_test.go`
- 模块文档、README、控制台、CHANGELOG 同步

### 不做

- 不改变 `--std` 模式的 Ctrl+C 行为（Go 默认退出）
- 不取消或中断正在执行的网络请求/插件任务
- 不新增命令行参数（3 秒为固定值）
- 不改 masscan 查找、目标/端口转换逻辑

### 验收条件

- [x] 第一次 Ctrl+C 停止 masscan；在飞扫描 3 秒内结束时正常清理并以 130 退出，不打印 “masscan mode failed”
- [x] 第一次 Ctrl+C 后 3 秒仍有任务未结束：停止进度、删除 `fscanx-masscan-*` 临时文件，以 130 退出
- [x] 第一次 Ctrl+C 后再按一次 Ctrl+C：立即清理并以 130 退出
- [x] 未按 Ctrl+C 的正常完成路径行为不变：退出码 0、临时文件清理、进度行清除
- [x] 单测覆盖 helper 的第一次信号、第二次信号、计时到期、正常完成四个分支
- [x] `go test ./Plugins ./common`、`go build .` 通过
- [x] `--std` 模式无行为变化
- [x] 用户在真实终端确认第一次/第二次 Ctrl+C 行为（2026-11-13 用户真机测试确认；helper 单测覆盖策略）

### 依赖

- 无

### 状态记录

- 2026-11-13：立项并开始实现。用户确认采用“有界收尾 + 二次强退”：第一次 Ctrl+C 最多等 3 秒，超时或第二次 Ctrl+C 立即清理退出。
- 2026-11-13：实现完成。新增 `Plugins/masscan_interrupt.go`（两段式中断 helper）与 `Plugins/masscan_interrupt_test.go`；`Plugins/masscan.go` 接入有界等待/二次强退与中断哨兵；`main.go` 对中断以 130 退出。
- 2026-11-13：本地验证通过：`go build .`；`go test ./Plugins ./common`；`go test -race -run TestWaitMasscanInterrupt ./Plugins`；`go vet ./Plugins ./common`。真机 Ctrl+C 行为待用户确认。
- 2026-11-13：用户真机测试确认 Ctrl+C 行为（第一次有界收尾、第二次强退），全部验收条件勾选，验收状态置已验收。

## BL-MSC-003 -msc 结束后清除底部 rate 进度行

- 模块：masscan 联动
- 优先级：P2
- 状态：已完成
- 当前阶段：持续迭代
- 价值：masscan 退出后终端底部不再残留最后一帧 `rate: ... 100% done` 状态行，下游扫描输出干净。
- 关联模块文档：docs/modules/03-masscan联动模块.md
- 关联计划：plans/2026-11-14-BL-MSC-003-msc-rate-line-clear.md
- 关联代码：Plugins/masscan.go
- 验收状态：已验收

### 方案

- **做什么**：stderr 解析协程读尽（EOF＝masscan 进程退出）即调用 `common.StopProgress()` 擦除并停用底部进度行，不等 `ScanFromReader` 跑完整轮下游扫描。
- **为什么**：原实现把 `StopProgress` 放在 `ScanFromReader` 之后，而后者阻塞到全部下游扫描收尾；期间输出协调层每打一条日志都重绘最后一帧 rate 行，导致它"钉"在底部贯穿整个扫描——与模块文档「启动流程 step 5」描述的"masscan 结束后清掉进度行"不符。

### 范围

- `Plugins/masscan.go`：stderr 协程末尾追加一次 `common.StopProgress()`（原函数尾部调用保留为幂等兜底）

### 不做

- 不在"100% done"状态行出现时提前擦除（masscan 退出前 wait 重传阶段保留该行有信息量）
- 不为下游扫描新增任何进度显示（模块文档「不做」已声明无 fscanx 自身进度条）
- 不改中断路径（第一次 Ctrl+C 的 `TryStopProgress` 原样）

### 验收条件

- [x] `go build .`、`go vet ./Plugins ./common` 通过
- [x] masscan/progress 相关单测 `-race` 全过（13 项），无回归
- [x] 全量 `go test ./Plugins` 无新增失败（既有失败 `TestSealDuringInflightKeepsAllTasks` 经 stash 对照确认为 BL-DIR-001 遗留，与本次无关）
- [x] 真机 `-msc` 实跑：masscan 退出后 rate 行消失，下游扫描期间终端底部干净、日志无残行交错（2026-11-14 用户真机组合扫描一并观察确认）

### 依赖

- 无

### 状态记录

- 2026-11-14：立项并实现完成。用户报告 rate 行贯穿整个扫描；定位为 `StopProgress` 时机滞后（在 `ScanFromReader` 之后而非 masscan 退出时）。定向验证：build/vet/单测 race 全过；真机视觉待用户下次 `-msc` 实跑确认后置已验收。
- 2026-11-14：用户真机验收通过（-msc -auto 组合扫描中一并观察 rate 行为），验收状态置已验收，控制台移除。

## BL-MSC-004 -msc -auto 组合（探活复用原逻辑，masscan 接手端口扫描）

- 模块：masscan 联动
- 优先级：P1
- 状态：已完成
- 当前阶段：持续迭代
- 价值：大网段场景一条命令完成"-auto 智能探活 → masscan 高速端口发现 → fscanx 深度扫描"全链路，探活产物（存活 C 段）留存 `.msc/` 可复跑可审计。
- 关联模块文档：docs/modules/03-masscan联动模块.md
- 关联计划：plans/2026-11-14-BL-MSC-004-msc-auto.md
- 关联代码：Plugins/masscan.go、Plugins/scanner.go（只读调用 AutoScanBigCidr）、common/masscan_flag.go、main.go
- 验收状态：已验收

### 方案

- **做什么**：解禁 `-msc -auto` 组合：-auto 大网段探活维持原逻辑（TCP+ICMP → 存活 C 段），产物写入 `.msc/msc-targets-<时间戳>.txt` 留存，masscan 以 `-iL` 消费该文件做端口扫描，stdout 进现行 -msc 下游管线。
- **为什么**：大网段下 fscanx 自身端口扫描吞吐不足，masscan 是该阶段最优工具；但 masscan 不适合做"找有主机 C 段"的探活（SYN 只覆盖 -p 端口集），-auto 的 TCP/ICMP 探活是既有成熟能力——两者在各自最强的阶段接力。
- **怎么做**（定稿八决策，详见关联计划）：探活原样调用 `AutoScanBigCidr` 零改动；组合后跳过"存活 C 段全 IP ICMP"阶段（masscan SYN 扫覆盖 IP 级发现，`-np` 此组合无作用）；`ScanWithMasscan` 拆核心 `runMasscanTargets` + 新增 `ScanWithMasscanAuto`；留存文件时间戳名/懒创建/不清理/无注释行；中断清理排除留存文件；校验矩阵删 `-auto` 互斥、新增必须 `-h` 且拒绝 `-hf`、0 存活提示退出 0；探活后释放 BloomFilter；`-mr` 默认 600 不变。
- **为什么这么做**：用户三拍板（`.msc/` 目录、0 存活退出 0、仅 -auto 组合留存）；探活不动一行是用户明示"维持原先逻辑"；`-hf` 拒绝因为探活目标只来自 `-h`（静默忽略劣于显式报错）。

### 范围

- `common/masscan_flag.go`：校验矩阵调整（删 -auto 互斥、加组合新规则）
- `Plugins/masscan.go`：拆 `runMasscanTargets`、新增 `ScanWithMasscanAuto` 与 `writePersistedMasscanTargets`、中断清理排除留存文件、BloomFilter 释放
- `main.go`：-msc 分支按 AutoScanBigCidr 分流
- 单测：留存写入器；手动：校验拒绝路径与不支持网段路径
- 模块文档、README、控制台、CHANGELOG、设计总览同步

### 不做

- 不改 `AutoScanBigCidr` 与普通 `-auto` 行为
- 不改普通 `-msc`（目标临时文件仍用后即删）
- 不做留存文件自动轮转/清理
- 不为探活阶段加两段式 Ctrl+C（与普通 -auto 现状一致）

### 验收条件

- [x] `-msc -auto -h <大网段>`：探活正常 → `.msc/` 出现时间戳目标文件，行数=存活 C 段数，终端打印路径（2026-11-14 用户真机实测通过）
- [x] masscan 以该文件为 `-iL` 目标正常扫描，下游管线与现行 -msc 一致，底部 rate 行在 masscan 结束后消失（BL-MSC-003 行为继承）（同上，一并观察）
- [x] `-msc -auto` 无 `-h`、`-msc -auto -hf x`：非零退出明确报错；`-msc -auto -h 1.2.3.4`（不支持网段）非零退出（二进制实测，含 `-h 192`/`-h 10` 简写过闸对照）
- [x] 0 存活 C 段：打印提示退出 0，不建 `.msc/` 目录不启动 masscan（审查级闭合：逻辑经三轮审查确认可达，未单独构造 0 存活网段实测）
- [x] Ctrl+C 两段式退出不删除 `.msc/` 留存文件（exclude 临时文件仍清理）（审查级闭合：keepTargets 守卫经三路径代码对照，未单独实测中断）
- [x] 普通 `-msc`（无 -auto）行为零变化：目标临时文件用后即删，无 `.msc/` 目录产生（本机冒烟：127.0.0.1:445 退出 0、无 `.msc/`、无临时残留 + reviewer 三路径代码对照）
- [x] 单测（留存写入器）+ 全量 `go build`/`go vet`/`go test ./Plugins ./common` 通过（含新增用例 race）
- [x] 真机组合扫描用户验收（2026-11-14 用户测试通过）

### 依赖

- masscan 可执行文件与运行条件同 BL-MSC-001（Windows Npcap / Linux raw socket 权限）
- 无新增第三方依赖

### 状态记录

- 2026-11-14：方案经 B0 设计定稿（含孤立事实核查：探活返回值恰为 masscan -iL 格式、PoolScan 容量与模式无关、flag.go:139 BloomFilter 行为），用户三决策拍板后批准实现。
- 2026-11-14：实现完成。`ScanWithMasscan` 拆出共享核心 `runMasscanTargets`；新增 `ScanWithMasscanAuto`（镜像网段判定→探活→BloomFilter 释放→0 存活短路→留存→masscan）与 `writePersistedMasscanTargets`（`.msc/` 懒创建、同秒序号、失败不留半成品）；校验矩阵删 `-auto` 互斥、加组合规则；中断清理按 keepTargets 排除留存文件。
- 2026-11-14：独立审查三轮（REVISE→REVISE→APPROVE），闭合四项：①0 存活被 nil 误判为"不支持网段"（CountAliveIPCidrWithGlobal 零存活返回 nil，死分支）→调用侧镜像判定+len==0；②switch 无 fallthrough 短路 `-m icmp`/`-mr<=0` 校验→case 前移；③gofmt 双空行；④镜像判定漏 `192`/`10` 简写归一（scanner.go:173-177）→补归一+文案。非阻断备案：普通 -msc 双错时报错顺序变化（均 exit 2）、os.Stat 持续错误的理论死循环、同秒 `-2` 后缀弱断言。
- 2026-11-14：验证：单测 race 全过；全量 `go test ./Plugins ./common` 双 ok；vet/gofmt/build 通过；二进制实测校验路径（无 -h/-hf/不支持网段/简写过闸，限时探针验证）；普通 -msc 冒烟回归零变化。真机组合扫描（留存文件行数、masscan 消费、Ctrl+C 不删留存）待用户验收。
- 2026-11-14：用户真机测试通过，验收闭合。其中 0 存活与 Ctrl+C 两条边缘路径以审查级证据闭合（未单独构造场景，先例 BL-DIR-001 豁免模式），异议可回置待验收。验收状态置已验收，控制台移除。
