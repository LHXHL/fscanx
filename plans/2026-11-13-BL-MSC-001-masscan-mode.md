# BL-MSC-001 masscan 内嵌联动模式

## 目标

新增 `-msc` 运行模式：fscanx 自己查找并启动 masscan，实时消费其扫描结果，复用 `--std` 扫描管线，并在终端底部显示 masscan 进度，消除两个进程抢标准输出的字符混杂。

## 当前状态

- 2026-11-13：立项完成，已按两轮独立评审意见修订；第二轮 6 条 MED（参数边界、445 验收空过、临时文件清理、`-hf` 逗号列表、`-margs` 优先级、`-msc` 派生初始化）已处理。
- 2026-11-13：实现完成，本地验证通过，item 置 `已完成/待验收`。待用户在真实终端确认底部进度视觉与 Ctrl+C 清理路径。

## 执行计划

1. 在 `common/flag.go` 增加 `-msc/-mr/-mp/-margs` 及互斥/必填校验（放在 `common.Parse` 之前，错误以非零码退出）：`-msc` 拒绝与 `-std/-dp/-u/-uf` 同用，`-margs` 拒绝托管参数；新增 `usesStdinLikePipeline()` 只用于并发模型和 BloomFilter 派生初始化，不设置 `ScanWithStdInput`。
2. 抽出 `ScanFromReader(io.Reader, targetLabel string)`，`ScanFromStdin` 改为薄封装，确保 `--std` 行为不变；main 中 `-msc` 分支先于 `-std`。
3. 新增 `Plugins/masscan.go`：程序查找、文件头识别、目标/端口转换、命令拼装、进程启动与收尾。
   - 目标：CIDR/完整区间原样传递，只展开末段简写；`-h` 按逗号拆分，`-hf` 逐行再按逗号拆分且只读；域名解析全部 IPv4；临时文件正常/失败/中断都删除。
   - 端口：`common.ParsePort` 展开 + 减 `-pn` + 压缩回区间。
   - 信号：Ctrl+C/SIGTERM 杀子进程、清进度、删临时文件后以非零码退出。
4. 新增 `common/progress.go`：stderr 进度行擦除/重绘、TTY/-silent 开关、日志前后协调；改造 `common/log.go` 与 `Plugins/scanner.go`，并把扫描路径的直接打印（smb、ms17010、redis、rdp、webScanTask、wmiexec、portscan 错误分支）接入协调层。
5. 接入 `main.go`：`-msc` 调用新的执行入口，处理退出码与 `Results` 关闭顺序。
6. 补单测：文件头识别（temp 文件 + 注入 GOOS/GOARCH）、目标转换（CIDR/末段简写/localhost/`-hf` 行内逗号/只读）、端口规范化（端口组/全角逗号/`-pn`/压缩）、进度行解析、命令拼装、进度激活时 `common.Println/Printf` 的擦除→日志→重绘顺序、临时文件正常/失败/中断清理。
7. 端到端：真实 masscan 1.3.2 跑一次；用假 masscan 输出 445 结果触发 SMB/ms17010 打印，验证进度行不被撕裂（环境无 445 时记录跳过原因并用协调层单测兜底）；验证重定向、`-silent`、参数拒绝组合和 `-msc` 不读 stdin。
8. 按同步矩阵更新 backlog、模块文档、控制台、CHANGELOG。

## 风险

- masscan 需要 Npcap（Windows）或 raw socket 权限（Linux），失败信息藏在 stderr，必须完整保留尾部供排查。
- 不同版本 masscan 状态行格式略有差异（`remaining` vs `waiting`），解析按宽松正则并保留原始文本。
- 进度行与多协程日志并发：高频结果日志集中在 SaveLog，但 445 等插件路径存在直接打印，必须逐个接入协调层。
- Windows 控制台对 ANSI/`\r` 的处理因终端而异，采用 `\r` + 空格擦除，不使用 ANSI 转义。
- 末段简写区间 masscan 不支持，必须展开；CIDR/完整区间不能展开，否则大网段会内存/临时文件失控。
- `-hf` 现有 `ReadInputFile` 会改写用户文件，不能复用。
- `-pn` 在 `--std` 管线不生效，`-msc` 必须在端口集合层面相减。
- `-dp` 会在 `common.Parse` 里把域名移出 `Info.Host`，`-u/-uf` 不会进入 masscan 管线；必须在 Parse 前直接拒绝这些组合，避免空目标。
- `-margs` 可能绕过托管参数，必须前置拒绝 `-p/--ports/--rate/-iL/--includefile/--exclude/--excludefile`。
- Ctrl+C/SIGTERM 默认不执行 defer，需在 masscan 模式显式接管信号，清理子进程和临时文件。
- `masscan.exe` 被 `.gitignore` 忽略、`masscan_linux` 未纳入版本库且无可执行位，自动化测试不能依赖仓库里的二进制，单测用临时构造的文件头。

## 验证方式

- 单元测试：`go test ./common ./Plugins -run 'Masscan|Progress|Target|Port'`
- 构建：`go build .`
- 端到端（真实 masscan）：`fscanx.exe -msc -h 127.0.0.1 -p 445 -mr 1000 -mp ./masscan.exe`，观察终端无 masscan 原始输出、进度行刷新。
- 端到端（假 masscan）：构造一个输出 `Discovered open port 445/tcp on 127.0.0.1` 和状态行的临时程序，`-mp <fake>` 观察本机 445 的 SMB/ms17010 打印不撕裂进度行。
- 重定向：`fscanx.exe -msc ... > out.txt`，确认 out.txt 无进度字符；`-silent` 同样确认。
- 失败路径：`-mp ./not-exist`、`-mp ./masscan_linux`（Windows 下错误架构）观察报错与退出码；`-msc -std`、只给 `-msc` 不给目标、`-msc -dp`、`-msc -u`、`-msc -uf`、`-margs "-p 80"` 观察非零码与明确报错。
- 临时文件：正常退出、启动失败、Ctrl+C 三种路径后检查系统临时目录无 `fscanx-masscan-*` 残留，`-hf` 文件哈希不变。
- stdin 隔离：关闭 stdin 或向 stdin 写入无关内容，`-msc` 仍只消费 masscan 管道。
- 回归：`fscanx.exe -std` 使用样例输入，行为与改动前一致。

## 验证结果（2026-11-13）

- `go build .`、`go test ./common ./Plugins`、新测试 `-race`、`go vet ./common ./Plugins` 全部通过。
- 假 masscan e2e：`Discovered open port 445/tcp on 127.0.0.1` 被扫到，`[+] alive ports(1): 445`，`found=1`；stdout 无 masscan 原始输出；系统临时目录无 `fscanx-masscan-*` 残留。
- 真实 masscan 1.3.2 e2e：exit 0，`found=0`，stdout 无原始输出，stderr 只有 banner。
- `-silent`/stdout 重定向：结果文件无进度字符；`-hf` 文件哈希不变；参数拒绝组合均 exit 2；`--std` 回归通过。
- 待用户实测：真实 TTY 下进度行视觉、Ctrl+C 中断后的子进程/临时文件清理（本环境无 TTY，且难以可靠发送 SIGINT）。
- `check-sync.py` 通过（4 个 item / 3 个文件，无发现）；脚本的 item 编号正则按用户批准适配 `BL-001 / BL-MEM-001 / BL-MSC-001`。
- 独立代码审查（opus）结论 REVISE，已修复 2 条 MED + 6 条 LOW：失败/中断不打印成功行；第二次 Ctrl+C 清理临时文件后退出；`--version` 失败保留 stderr 尾部与候选原因；URL 大小写、`-hf` 行内注释/空格、反向区间、参数错误 stderr。复测失败/成功/参数错误/`--std` 回归通过。
- Deslop（reviewer-only）通过：无死代码、无未使用导出、无可删的透传包装；`common` 与 `Plugins` 各有一处 `-margs` 托管参数校验，属前置校验 + 执行层防御性重复，保留。
- 用户验收（2026-11-13）：真实终端底部进度与 Ctrl+C 清理确认通过，BL-MSC-001 验收状态置已验收并从控制台移除。

## 决策记录

- 2026-11-13：采用 `-msc/-mr/-mp/-margs`；目标映射 `-h/-hf/-hn` 并解析域名；流式扫描 + stderr 底部进度；新模块 03，优先级 P1。
- 2026-11-13：不依赖 masscan `-y`，解析标准 `Discovered open port` 输出；实测官方 1.3.2 无 `-y`。
- 2026-11-13：第一轮评审修订——`-pn` 用端口集合相减；`-hf` 独立只读解析；CIDR/完整区间不展开；扫描路径直接打印统一接入输出协调层；参数校验前置到 `common.Parse` 之前。
- 2026-11-13：第二轮评审修订——拒绝 `-msc` 与 `-dp/-u/-uf` 组合；`-margs` 禁止托管参数；临时文件覆盖正常/失败/中断清理；`-hf` 行内逗号拆分；`-msc` 不设置 `ScanWithStdInput`，派生初始化用独立条件。
