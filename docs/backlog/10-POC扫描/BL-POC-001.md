# BL-POC-001 触发层修复：-poc 零请求止血

- 模块：POC 扫描
- 优先级：P0
- 状态：已完成
- 当前阶段：持续迭代
- 价值：`-poc` 开关恢复真实发包——当前用户开启 `-poc` 不指定 `-pocname`、以及 `-m webpoc` 非 URL 输入路径均静默零请求，POC 功能整体失效。
- 关联模块文档：docs/modules/10-POC扫描模块.md
- 关联计划：plans/2026-11-15-BL-POC-001-poc触发修复.md
- 关联代码：PocScan/WebScan.go、Plugins/webScanTask.go、common/Parse.go
- 验收状态：已验收

### 方案（立项时必填）

- **做什么**：四处最小修复——①空 `-pocname` 分支从遍历 `HostInfo.PocName`（恒空 slice，循环 0 次）改为直接执行全量（`filterPoc("")` 现成语义返回全部 386 个）；②`-pocname all` 归一化为空走全量（防止 all 被当子串过滤，误打出名字含 all 的 poc 子集）；③`filterPoc` 的 `strings.Contains` 改大小写不敏感；④webpoc 分支补空 Url 构造（复用 `WebScanWorker` 的构造逻辑提取为 `ensureURL`），消除 `buf[:3]` 越界 panic。
- **为什么**：用户双反馈触发审计，证实 `HostInfo.PocName` 全项目零填充（唯一写点 scanner.go:401 透传空值，唯一读点 WebScan.go:29 在死分支内）；README.md:246-247 承诺 `-pocname` 是"按名称过滤"语义（过滤=不带则全扫）。独立审查另发现：`-m webpoc` + 非 `-u` 目标时 portscan.go:193 构造的 HostInfo 无 Url，webpoc 分支（webScanTask.go:251-254）跳过唯一 Url 构造者 `WebScanWorker`，`PocScan()` 对空 Url 做 `strings.Split` 取 `buf[:3]`（WebScan.go:23-24）越界 panic 被上游 recover 吞掉——用户反馈"实际没发 poc 请求"最可能即此路径，故并入本 item 止血。
- **怎么做**：`PocScan()` 删 else 死循环，`EqualFold(PocName, "all")` 时置空后统一 `Execute`；`filterPoc` 双侧 ToLower 比较；`webScanTask.go` 把 Url 构造段提取为 `ensureURL(targetInfo)` 供 `WebScanWorker` 与 webpoc 分支共用。
- **为什么这么做**：一行级修复恢复 `filterPoc("")` 本就存在的全量语义（设计意图未失，是链路断了）。空=全量是**过渡语义**——指纹定向管道（绑定关系数据加载后才有智能定向输入）落地时切换为终态三态。否决"顺手清理 `HostInfo.PocName` 死字段、`CheckInfoPoc`/`PocDatas` 硬编码表、`LoadMultiPoc`/`SelectPoc` 死代码"——统一归管道 item 退役，本 item 不夹带（最小必要修改）。

### 范围

- PocScan/WebScan.go（PocScan 分支重构、filterPoc 大小写、all 归一化）
- Plugins/webScanTask.go（Url 构造提取 ensureURL + webpoc 分支调用）
- common/Parse.go（ParseScantype 补 webpoc case——意外发现顺手修复，见状态记录）
- plans、CHANGELOG

### 不做

- 指纹定向管道与终态三态语义切换（BL-POC-003——需要 binds.yaml 数据加载先行）
- `HostInfo.PocName` 死字段、`CheckInfoPoc`/`PocDatas` 硬编码表（rules.go:297）、`LoadMultiPoc`/`SelectPoc` 死代码清理（BL-POC-003 统一退役）
- 合并式加载与 `./pocs/` 外置目录（BL-POC-002）

### 验收条件

- [x] `-u <目标> -m webpoc` 不带 `-pocname`，本地 http 目标日志确认发出 poc 请求（实测 1168 请求，此前零请求）
- [x] `-h 127.0.0.1 -m webpoc`（非 URL 输入路径，端口走 Webport）不 panic 且发出请求（实测 1189 请求；注：`-p` 显式端口被 `-m` 模式端口覆盖属 fscan 系既有设计，故按 Webport 内端口验证）
- [x] `-pocname` 传大小写变体（如 74CMS）能命中 poc-yaml-74cms-*（实测 6 请求，server 日志路径确为 74cms 系列 poc）
- [x] `-pocname all` 发出全量请求且不会被误过滤为"名字含 all"子集（实测 1168，与空 `-pocname` 同量级）
- [x] `go build ./...` 与 `go test ./PocScan/... ./Plugins/...` 通过（实测 PocScan/lib ok、Plugins ok、common ok，vet 通过）

### 依赖

- 无

### 状态记录

- 2026-11-15：立项（Mode B0 五轮共识 + 独立审查 16 项事实核对、3 必改项修订后用户批准）。终态三态语义（空=智能定向、all=全量、名=定向）本 item 先落空=全量过渡；webpoc 空 Url 越界由独立审查发现并入（止血不完整等于没止血）。
- 2026-11-15：执行完成，本地五路径实测全过（空=1168、IP 输入=1189、74CMS 大写=6、all=1168、`-m all -p -poc`=1192；目标为 python http.server 计数 server 日志行）。改动四处：WebScan.go、webScanTask.go、Parse.go（见范围）。**意外发现两条**：① `-m webpoc` 落 ParseScantype default 拿 PluginPortMap 调度键 1000003 当扫描端口（永远非法），webpoc 对 ip 输入从未工作过——已顺手修复（加 case 对齐 web/webonly 用 Webport），属触发层范畴；② `-pa` 追加在 ParseInput 执行、ParseScantype 模式端口覆盖在其后，`-m <模式> + -pa` 组合的追加会被覆盖——fscan 系既有设计问题，本 item 未动。调试插曲：ensureURL 首版漏"Url 非空直接返回"守卫，把 `-u` 输入的正确 Url 用 Ports 占位值重写（实测 -u 路径仍零请求暴露），补守卫后全过——教训：提取函数时外层条件必须随行。待用户真机验收。
- 2026-11-15：验收闭合（用户无真机环境，委托 vulhub docker 靶场代验）。靶场 vulhub thinkphp/5-rce（vulhub/thinkphp:5.0.20，127.0.0.1:8080）。五条全过且端到端命中真漏洞：A1 用户原始场景 `-h -p 8080 -m all -poc` 输出指纹行 `[+] Product ... [*thinkphp]` + 漏洞命中 `[+] PocScan ... poc-yaml-thinkphp5-controller-rce`；A2/A3 空 pocname 与 IP 输入路径均命中；A4 `-pocname ThinkPHP` 大写命中（旧代码必漏）；A5 all 命中。验收条件全部满足，置已验收。
