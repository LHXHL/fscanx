# 模块：masscan 联动

## 模块定位

负责 fscanx 与 masscan 的进程内协作：查找 masscan、转换目标与端口格式、启动进程、实时读取结果和进度、协调终端输出。不包含 masscan 自身的探测逻辑，也不改变 fscanx 的端口探测与协议识别插件。

## 现状基线

- 现有 `--std` 模式（Plugins/scanner.go）支持从标准输入逐行读取 `ip:port`、masscan 屏幕输出、masscan `-oL` 文件内容、URL 和域名；`common/config.go` 里的两个正则负责识别 masscan 的两种文本格式。
- 旧用法是用户自己用管道连接两个程序。两个进程同时写终端，输出会交错；个人修改版 masscan 通过拉长进度打印间隔来缓解，但没有消除第二个写者。
- 终端输出目前集中在 common/log.go 的 SaveLog 单协程里，结果通过 Results 通道进入；但扫描路径仍有绕过它的直接打印（smb.go、ms17010.go、redis.go、rdp.go、webScanTask.go、wmiexec.go、portscan.go 的错误分支，以及 scanner.go 的启动行），这些打印会撕裂底部进度行。
- `--std` 管线的 `PortProbeSingleOnStd` 不检查 `-pn`，端口排除只在批量扫描路径生效；`-msc` 不能假设 `-pn` 会自动生效。
- 现有 `ReadInputFile` 会先原地去重并覆写用户输入文件，还会把 URL 和 `ip:port` 写入全局扫描列表，不能直接用于 masscan 目标文件转换。

## 关键设计决策

- 不依赖 masscan `-y`：官方 1.3.2 没有该选项；统一解析标准 `Discovered open port ...`，并兼容 `IP:PORT` 行。
- 流式而非批处理：masscan 还在跑的时候就把结果送进扫描管线，保留原有管道边发现边扫描的加速效果。
- 终端所有权：进度行写 stderr 并用 `\r` 原位覆盖；结果日志写 stdout；每条日志前后由输出协调层擦除/重绘进度。非 TTY 或 `-silent` 关闭进度。
- 程序发现顺序：`-mp` → PATH → 当前目录 → fscanx 所在目录；先按文件头匹配系统和架构，多个候选再逐个 `--version` 试跑。
- 目标格式转换：masscan 不支持末段简写区间（实测 `192.168.137.230-232` 报 `bad IP address/range`），所以只展开这种简写；CIDR 和完整区间原样传递，避免把 `10.0.0.0/8` 枚举成上千万行。
- 端口格式转换：先展开端口组和区间得到端口集合，减去 `-pn`，再压缩回区间；这样既支持 `-p web`，也不会把 `1-65535` 拼成超长命令行。
- `-hf` 只读：不复用会改写用户文件的 `ReadInputFile`，单独实现只读转换；每行先按逗号拆成 token，再逐项分类。
- 参数边界：`-msc` 与 `-std`、`-dp`、`-u`、`-uf` 互斥；`-margs` 不能出现 fscanx 托管的参数（`-p/--ports/--rate/-iL/--includefile/--exclude/--excludefile`），避免绕过端口相减和速率设置。`-margs` 只用于 fscanx 未托管的 masscan 选项（如 `--wait`、`--adapter-ip`、`--ttl`、`--banners`）；端口用 `-p`、速率用 `-mr`、目标用 `-h/-hf`、排除用 `-hn`。
- 派生初始化独立：不把 `-msc` 写成 `ScanWithStdInput=true`（否则 main 会去读 os.Stdin），只用 `usesStdinLikePipeline = -std || -msc` 控制并发模型和 BloomFilter。
- 临时文件生命周期：目标/排除文件放系统临时目录，正常返回、启动失败、Ctrl+C/SIGTERM 都删除；用户 `-hf` 原文件不动。

## 关键功能怎么做

### 启动流程

1. 在 `common.Parse` 之前校验 `-msc` 与 `-std/-dp/-u/-uf` 互斥、`-h/-hf` 至少一个、`-margs` 不含托管参数；错误时打印原因并以非零码退出。
2. 查找 masscan；转换目标（`-h`/`-hf`，逐行再按逗号拆分）和排除项（`-hn`）到临时文件；展开、相减、压缩端口。
3. 拼装参数：`--rate <mr> -p<ports> -iL <targets> [--excludefile <exclude>] [margs...]`。
4. 启动进程，stdout 交给 `ScanFromReader`，stderr 交给进度解析器；两路都不继承终端。
5. masscan 结束后等待 stderr 读完，清掉进度行，打印统计；非零退出时打印 stderr 末尾并以非零码退出。

### 进度渲染

masscan 的状态行原本就写到 stderr、以 `\r` 结尾并用空格补齐，终端会把同一行反复覆盖，所以看起来“钉”在底部。fscanx 解析这些状态行（`rate: ...-kpps, ...% done, ... remaining/waiting ..., found=N`），清洗后交给进度模块重绘。进度模块记录上一次显示的宽度，擦除时用 `\r` + 等宽空格 + `\r`，不依赖 ANSI 转义，兼容 Windows 控制台。日志输出前后调用擦除/重绘，因此日志出现在进度行上方。扫描路径里绕过 SaveLog 的直接打印也必须接入同一个协调层，否则在 445 等目标上会撕裂进度行。

### 失败处理

- 找不到 masscan：列出搜索过的位置和候选文件，退出非零。
- 候选文件架构不符、没有可执行位或不能执行：记录原因，继续尝试下一个。
- masscan 启动失败或非零退出：打印 stderr 尾部（如 Windows 缺 Npcap、Linux 无权限），退出非零。
- 域名没有 IPv4 记录、目标文件为空、端口集合为空：明确报错并退出非零，不能静默扫空。
- 收到 Ctrl+C/SIGTERM：杀掉 masscan 子进程、清掉进度行、删除临时文件，再以非零码退出。
- `-silent` 或输出被重定向：不渲染进度，只保留文件日志。

## 相关 backlog

- BL-MSC-001 新增 masscan 内嵌联动模式（docs/backlog/03-masscan联动.md）

## 变更记录

- 2026-11-13：BL-MSC-001 立项，设计 masscan 内嵌联动模式。
- 2026-11-13：第一轮独立评审修正：`-pn` 改为端口集合相减、`-hf` 只读转换、CIDR/完整区间不展开、直接打印纳入输出协调、补参数校验与端口压缩设计。
- 2026-11-13：第二轮独立评审修正：拒绝 `-msc` 与 `-dp/-u/-uf` 组合；`-margs` 禁止托管参数；临时文件定义正常/失败/中断清理；`-hf` 行内逗号拆分；`-msc` 派生初始化不复用 `ScanWithStdInput`。
- 2026-11-13：BL-MSC-001 实现完成：新增 `-msc/-mr/-mp/-margs`、程序查找与文件头识别、目标/端口转换、双管道执行、stderr 进度解析与输出协调、信号与临时文件清理；单测和真实/假 masscan e2e 通过，真实终端视觉与 Ctrl+C 路径待用户确认。
- 2026-11-13：独立代码审查修复：失败/中断不打印成功行；第二次 Ctrl+C 清理临时文件后退出；试跑失败保留 stderr 与候选原因；`-hf` 容忍行内注释/空格、URL 大小写不敏感、反向区间提前拒绝、参数错误写 stderr。
- 2026-11-13：用户真机验收通过，BL-MSC-001 闭合，控制台移除该项。
