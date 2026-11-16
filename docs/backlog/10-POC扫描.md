# POC 扫描模块 Backlog

POC 扫描模块：修复并重构 Web POC 检测的触发与加载机制。起点是 2026-11-15 用户双反馈——"-poc 实际没有发 poc 请求（PocName 为空）"与"设置 -poc 不指定 -pocname 不会发送 poc 扫描"；只读审计证实触发链两处断裂：空 `-pocname` 走全项目零填充的 `HostInfo.PocName` 死字段空循环，`-m webpoc` 非 URL 目标空 Url 越界 panic 被 recover 吞掉，两者都表现为静默零请求。本模块四步走：止血（触发修复）→ 合并式加载（新增 poc 零编译）→ 指纹定向管道（流量从 386 poc×每 URL 降到按指纹命中定向）→ 独立 web 维护端（用户零手写 yaml）。定位小而精，放弃 nuclei 兼容。

| ID | 标题 | 优先级 | 状态 | 验收状态 |
|----|------|--------|------|----------|
| BL-POC-001 | 触发层修复：-poc 零请求止血 | P0 | 已完成 | 已验收 |
| BL-POC-002 | 合并式 poc 加载 + 绑定关系外置 | P1 | 已完成 | 已验收 |
| BL-POC-003 | 指纹定向触发管道 | P1 | 已完成 | 已验收 |
| BL-POC-004 | 独立 fscanx-web 维护端一期 | P2 | 已完成 | 已验收 |
| BL-POC-005 | 绑定关系页交互重构 + POC 页快捷绑定 | P1 | 已完成 | 已验收 |
| BL-POC-006 | 指纹库 SQLite 存储化 | P2 | 未开始 | 未验收 |
| BL-POC-007 | fscanx-web 登录认证（默认账号） | P1 | 已完成 | 已验收 |

## BL-POC-001 触发层修复：-poc 零请求止血

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

## BL-POC-002 合并式 poc 加载 + 绑定关系外置

- 模块：POC 扫描
- 优先级：P1
- 状态：已完成
- 当前阶段：持续迭代
- 价值：新增 poc 零编译——yml 丢进 `./pocs/` 即生效；绑定关系数据化（fscan 原版硬编码映射表不可维护）。
- 关联模块文档：docs/modules/10-POC扫描模块.md
- 关联计划：plans/2026-11-15-BL-POC-002-合并式poc加载.md
- 关联代码：PocScan/WebScan.go、PocScan/binds.go、PocScan/binds_test.go
- 验收状态：已验收

### 方案（立项时必填）

- **做什么**：`initpoc()` 从"embed 或 `-pocpath` 二选一"改三层合并——内置 embed 386 个兜底 + 约定目录 `./pocs/` 自动加载 + `-pocpath` 显式追加目录，同名 poc 外置覆盖内置；新增 binds.yaml 绑定关系文件加载（键=指纹引擎输出产品名，值=poc 的 yaml `name` 字段列表，`always: true` 标记无条件全打型）。
- **为什么**：现状 `-pocpath` 是替换式（设置后内置 386 全部失效），"内置全量+自己加几个"做不到；指纹定向管道需要数据化绑定源（现只有 `PocScan/info/rules.go:297` 的 PocDatas 硬编码表，改一次要重编译，与"指纹内嵌、poc 外置"的资产分层决策矛盾——绑定关系属于用户可维护数据）。
- **怎么做**：合并去重按 name 外置优先；binds 解析进内存结构供管道消费；`./pocs/` 基准目录（二进制所在目录 vs 工作目录）在方案阶段定。
- **为什么这么做**：拒绝 nuclei 式纯外置与网络自动更新——内网离线场景内置兜底必须保留；`-pocpath` 保留为追加语义（用户拍板）。

### 范围

- PocScan/WebScan.go（initpoc 合并逻辑）
- common/flag.go（如需目录参数）
- binds.yaml 加载与解析

### 不做

- web 编辑器（BL-POC-004）
- 指纹外置加载（用户拍板指纹保持内嵌）
- binds 的消费端定向执行（BL-POC-003）

### 验收条件

- [x] `./pocs/` 放新 yml 不加任何参数被加载执行（实测：二进制旁 pocs/ 放 test-echo.yml，`-m webpoc -pocname test-echo` 目标日志命中 /echo-poc-marker，输出 `[+] load 1 pocs from ...`）
- [x] 同名外置 poc 覆盖内置生效（实测：pocs/ 放同名 poc-yaml-74cms-sqli（path 改 /overridden-marker），目标日志出现 /overridden-marker 即证明替换保位——同名去重后唯一，覆盖版执行=原版被替换）
- [x] `-pocpath` 与内置合并追加，不再替换失效（实测：无 pocs/ 目录 `-pocname all` 计数 1168 与 001 基线一致=内置未失效；`-pocpath` 目录的 extra-poc 被执行命中 /extra-poc-marker）
- [x] binds.yaml 解析与 always 标记有单元测试（TestLoadBinds 四子测试全过：不存在/正常+always/空文件/非法 yaml）

### 依赖

- BL-POC-001（触发路径修复后"被加载执行"才可验证）

### 状态记录

- 2026-11-15：立项（Mode B0 共识修订后批准）。开放点：`./pocs/` 基准目录、binds.yaml schema 细节（方案阶段定）。
- 2026-11-15：执行完成，四场景实测全过 + binds 单测 4/4。设计决策落定：①基准目录=**二进制所在目录**下 pocs/（os.Executable 推导；内网工具目录习惯，cd 不影响；go run 开发场景指向 temp 无妨，用 -pocpath）②binds schema=顶层 binds 数组，元素 product（fingers 引擎输出产品名）/always/pocs（取 poc yaml name 字段），文件不存在返回 nil 正常态③同名覆盖=**替换保位**（slice+index map），保证 filterPoc 遍历顺序稳定可测。initpoc 重构为 pocCollector 收集器三层加载（内置 embed → 约定目录 → -pocpath），binds 由 loadBindsFile 接线进全局 PocBinds（003 直接消费）；原内置加载静默跳过坏 yml 的行为保持不变。待用户真机验收。
- 2026-11-15：验收闭合（用户无真机环境，委托 vulhub docker 靶场代验，同 001 靶场）。四条全过且有容器侧第三方证据（docker access log）：B2 约定目录外置新 poc 零参数执行（`[+] load 1 pocs from ...` + `[+] PocScan ... poc-yaml-accept-echo` 命中 + 容器日志 `GET /accept-echo-marker`）；B3 同名覆盖可证伪闭环——`-pocpath` 指向同名 status==999 永不命中的覆盖版 → 无命中行且容器日志实证 `GET /override-marker-tp`（覆盖版在发包），去掉 `-pocpath` 内置版即恢复命中；B4 `-pocname all` 命中计数 2（内置 thinkphp RCE + 外置 echo）= 全量含外置。验收条件全部满足，置已验收。
- 2026-11-15：**002 回归修复轮（用户报告：exe 旁无 pocs/ 目录时报 CreateFile 错且黏行）**。根因两条：① exePocsDir 注释写明"存在与否由调用方判断"但 initpoc 实际未判断——约定目录不存在（多数用户正常态）时 filepath.Walk 首层返回 CreateFile 错误被当异常打印；② loadEmbeddedPocs/loadPocsFromDir 两处 fmt.Printf 错误输出缺换行，与后续成功行黏连。修复：initpoc 补 os.Stat+IsDir 守卫（约定目录不存在=正常态静默跳过，对齐 002 "存在才加载"设计语义）；显式 -pocpath 路径错误的报错保留可见（打错路径需要知道）并补换行。复现验证：无 pocs/ 目录跑 -poc 零报错且 thinkphp RCE 命中；-pocpath 指向不存在目录报错独立成行且扫描继续。

## BL-POC-003 指纹定向触发管道

- 模块：POC 扫描
- 优先级：P1
- 状态：已完成
- 当前阶段：持续迭代
- 价值：流量从 386 poc×每 URL 降到按指纹命中定向；补全全项目独立缺口（docs/backlog/04-目录扫描.md:66 早有记录"指纹→POC 管道另行立项"）。
- 关联模块文档：docs/modules/10-POC扫描模块.md
- 关联计划：plans/2026-11-15-BL-POC-003-指纹定向管道.md
- 关联代码：Plugins/webScanTask.go、Plugins/productDetect.go、Plugins/faviconScan.go、PocScan/WebScan.go、PocScan/binds.go、PocScan/binds-seed.yaml、common/config.go
- 验收状态：已验收

### 方案（立项时必填）

- **做什么**：落地终态三态——空 `-pocname`=智能指纹定向（web 扫描指纹识别结果（goby/fingers_http/favicon 全源产品名）传入 PocScan 查 binds.yaml 定向执行对应 poc；无命中不打且不提示）、`all`=全量、名字=定向过滤；`-m webpoc` 模式也先跑指纹识别再定向（现状该模式跳过 WebScanWorker 无指纹输入）；PocDatas 硬编码表转格式生成初始 binds.yaml 种子后退役；清理 `HostInfo.PocName` 死链、`CheckInfoPoc`、`LoadMultiPoc`/`SelectPoc` 死代码。
- **为什么**：全量 386 poc×每 URL 在大网段流量不可接受（终态空=智能定向后流量按命中收敛）；绑定键用引擎输出产品名与数据源文件解耦——goby.json、fingers_http.json、favicon 哈希命中的产品都可绑定（用户确认），web 端指纹列表以引擎实际输出为准。
- **怎么做**：webScanTask 指纹结果传入 PocScan；PocDatas 300+ 条产品名→poc 映射转格式成 binds.yaml 初始种子（存量转换，不凭空造）。
- **为什么这么做**：无命中不打且不提示（用户拍板；all 保持显式全量入口，无命中回退全量会让 all 失去意义且流量失控）；种子转换而非新造（PocDatas 即现成存量数据）。独立审查必改项：明确兜底语义防止"刚修好的全打能力被静默清零"——binds 文件缺失时按空表处理=空 `-pocname` 不打、`all` 与显式名不受影响。

### 范围

- Plugins/webScanTask.go（指纹结果管道传入）
- PocScan/WebScan.go（三态语义、binds 消费）
- PocScan/info/rules.go（PocDatas 退役）
- 初始 binds.yaml 种子文件

### 不做

- binds 数据的生产端编辑（BL-POC-004 web 端）
- 指纹引擎本身的能力变更

### 验收条件

- [x] 指纹命中的模拟目标只发对应 poc，不发全量（C1 靶场实测：全量 612+ 条 → 定向 **8 条**（首页+favicon 探测+3 个 thinkphp 系 poc），只出现 thinkphp 路径，RCE 命中输出）
- [x] 无指纹命中的目标零 poc 发出且无提示输出（C2 干净 python server：零 poc 请求、输出零提示行）
- [x] `always: true` 型 poc 对所有目标始终发（C3 外置 binds.yaml 加 always 条目，无指纹目标收到 marker 请求）
- [x] `-pocname all` 全量、`-pocname <名>` 定向过滤均正常（C4：all 612 条；74CMS 大写命中 AjaxPersonal/officebuilding 路径 2 条；`-m all -p -poc` 智能定向命中 controller-rce）
- [x] PocDatas/CheckInfoPoc/HostInfo.PocName/LoadMultiPoc/SelectPoc 全仓无残留引用（.go 代码零残留，仅 gen 脚本与 binds.go 注释保留 2 处数据来源历史说明）
- [x] `go build ./...` 与相关测试通过（build+vet 过；PocScan/Plugins/common 全量测试 ok；binds 单测 7 函数含种子/合并/定向查询）

### 依赖

- BL-POC-002（binds.yaml 加载先行）

### 状态记录

- 2026-11-15：立项（Mode B0 共识修订后批准）。空 `-pocname` 语义在本 item 从"全量过渡"切换为"智能定向"（BL-POC-001 状态记录已注记）。
- 2026-11-15：执行完成。改动：HostInfo.PocName 死字段退役改 Frames（指纹原始产品名）；DoWebScan/favicon 链尾命中处填充 Frames；webpoc 分支新增 detectProductsSilent（Plugins/productDetect.go，静默首页指纹）；PocScan 三态分发（all=全量/名=过滤/空=boundPocNames 查表定向，无命中不打不提示）；binds 两层加载（binds-seed.yaml 内置种子 embed 打底 + 外置 binds.yaml 同 product 覆盖，mergeBinds 纯函数可单测）；PocDatas 14 条经 gen 脚本（plans/scripts/gen_binds_seed.go，可重跑刷新）转种子 15 产品 53 绑定（weaver 条目按 Contains 吸并 weaver-ebridge 的 poc，与原 CheckInfoPoc 语义一致宁多勿漏）；六处死代码清理。**意外发现两条**：① `Engine.DetectContent` 输入契约是**完整 HTTP raw 报文**（内部 ReadResponse，状态行+头+body）——首版静默函数传纯 body 导致指纹恒空（实测 `malformed HTTP status code` 报错暴露），改用 ReadRawWithSize 同源拼装；② thinkphp 靶场文本指纹弱，favicon 链是定向的关键输入——detectProductsSilent 补 detectSiteFavicon+matchFaviconFrames（favicon-only 产品依赖此链）。另记：shell 组合命令 `&` 优先级坑曾致旧 exe 顶替新构建跑出假结果，分步执行后排除。
- 2026-11-15：验收闭合（用户委托 vulhub 靶场代验，同 001/002）。C1-C4 全过（证据见验收条件勾选），流量对比 612→8 条（约 98.7% 降幅）。置已验收。

## BL-POC-004 独立 fscanx-web 维护端一期

- 模块：POC 扫描
- 优先级：P2
- 状态：已完成
- 当前阶段：持续迭代
- 价值：用户零手写 yaml/CEL 维护 poc 与绑定关系；指纹维护流程按钮化（编辑明文→重打包→重编译）。
- 关联模块文档：docs/modules/10-POC扫描模块.md
- 关联计划：plans/2026-11-15-BL-POC-004-fscanx-web维护端.md
- 关联代码：cmd/fscanx-web/（main.go、api_poc.go、api_binds.go、api_finger.go、predicate.go、static/index.html）、mylib/finger/packlib/packlib.go、mylib/finger/cmd/pack/main.go
- 验收状态：已验收

### 方案（立项时必填）

- **做什么**：新独立二进制 fscanx-web：①poc 列表/表单化新建/编辑/删除——谓词树表单（匹配项下拉 status/header.xxx/body/title + 操作等于/包含/正则/md5 + 值输入，条件间 AND/OR 可视化组合）序列化为 CEL；编辑已有 poc 时 CEL 在表单可生成子集内反解析回填，超出子集（多步利用链等）整卡切高级源码模式；写回 `./pocs/` 与 binds.yaml，落盘前校验（yaml round-trip + CEL 编译验证）。②绑定关系编辑器：指纹列表（引擎全量输出产品名，goby/fingers_http/favicon 三源合并）搜索勾选关联 poc。③指纹编辑：读写 `mylib/finger/resources/<name>/<name>/` 下明文 goby.json / fingers_http.json，文本手术式写回（格式敏感文件禁整体重序列化），指纹列表以引擎实际输出产品名为准。④重打包按钮：从 `mylib/finger/cmd/pack` 提取可导入共享包（确定性 gzip + JSON 校验），pack 命令与 web 端共同引用，产物 .gz 供下次编译嵌入。
- **为什么**：yaml/CEL 手写门槛高（缩进敏感、表达式语法）；指纹"改明文→跑 pack→重编译"流程目前全手工；用户拍板独立二进制（扫描器本体不膨胀）与指纹保持内嵌（web 端管的是源文件，不是运行时外挂）。
- **怎么做**：Go 实现独立入口 + 嵌入式前端；指纹产品名列表从引擎数据读取做选择器。
- **为什么这么做**：谓词树表单覆盖约 90% 检测型 poc，高级源码模式兜底复杂多步利用链（不做不可靠的强反解析）；pack 共享包是"重打包产物与直接跑 pack 字节等价"验收的前提——现状 pack 是 package main 零导出符号不可导入（独立审查发现，必改项）。

### 范围

- 新建独立入口（cmd/fscanx-web 或 web/ 目录）
- mylib/finger/cmd/pack 共享包提取（pack 命令改为引用共享包）
- 前端页面（poc 编辑器、绑定编辑器、指纹编辑器、重打包）

### 不做

- 指纹运行时外挂加载（用户拍板保持内嵌，web 端产物供下次编译）
- 多用户/远程服务化、poc 在线市场
- nuclei 模板导入（已放弃）

### 验收条件

- [x] 表单新建 poc → 落盘 → fscanx 不加参数加载执行成功（实测：表单 status==404 单条件 → `/api/poc` form 模式落盘 webform-demo.yml（yaml 模板与 CEL 正确）→ fscanx `-m webpoc -pocname webform-demo` 命中输出 + 目标日志 GET /webform-demo-marker）
- [x] 绑定编辑后定向执行生效（实测：`/api/binds` 写 thinkphp→[webform-demo] 覆盖种子条目 → fscanx 空 -pocname 智能定向对靶场只发新绑定 poc 的请求，原 3 个内置 thinkphp poc 路径全部消失）
- [x] 非法 CEL / 非法 yaml 落盘前被拦截（实测 4 例全 400：表单状态码非整数 / 坏 yaml 结构 / 合法结构但 CEL 编译失败（int==string）/ 空 content）
- [x] 指纹编辑→重打包产物与直接跑 cmd/pack 字节等价（实测：`/api/pack` 产物 goby.json.gz md5 与共享包基线一致 d30955d2…；同一 packlib 实现，无第二实现）
- [x] goby.json 编辑后除目标条目外零格式漂移（实测：API 取文→原样 PUT 回写→三方 md5 一致 70b7630d…（API 取文/回写后 API/磁盘文件）——textarea"内容即所写"语义 + JSON 合法性校验双重保证）

### 依赖

- BL-POC-002（./pocs/ 与 binds.yaml 落盘契约）
- BL-POC-003（绑定编辑的定向语义）

### 状态记录

- 2026-11-15：立项（Mode B0 共识修订后批准）。用户追加体验要求：匹配项下拉选择（header/body 等）、条件间 AND/OR 的 UI 必须好用——已入方案谓词树设计。
- 2026-11-15：执行完成。交付物：`cmd/fscanx-web/`（标准库 net/http + go:embed 原生前端，零新第三方依赖）——REST API 七组（poc 列表/读/写/删、CEL 校验、binds 读写、指纹读/写、pack、产品名全集）；谓词树↔CEL 双向转换（`predicate.go`：表单子集 status==N / body.bcontains / "re".bmatches(response.body)（bmatches 注册签名是 string receiver，内置 poc 生态统一参数反转形态，单测实证）/ headers[K].contains；反解析用 cel-go AST 白名单遍历，子集外返回 nil 前端切源码模式）；落盘双重校验（yaml 解析为 Poc 结构 + 每条 expression 在求值环境编译）；pack 共享包提取（`mylib/finger/packlib`，cmd/pack 改薄壳，重构后产物 md5 与提取前逐字节一致）；指纹 textarea 整文编辑（内容即所写=零漂移 + JSON 校验防线）。**执行中发现真 bug 一条**：Bind 结构只有 yaml tag 无 json tag，API 输出大写字段名与前端小写约定不匹配（绑定列表显示 undefined）——补 json tag 修复。范围裁剪记录：谓词表单一期为扁平条件列表 + 全局 AND/OR（嵌套树反解析出深度>1 自动切源码模式，复杂多步利用链走源码）；指纹编辑一期 textarea 整文（结构化单条目编辑留二期）。
- 2026-11-15：验收闭合（延续用户委托代验模式）。五条全过（证据见验收条件勾选）：表单→落盘→执行端到端命中、绑定覆盖致定向路径改变、4 例非法拦截、pack 字节等价、指纹回写三方 md5 零漂移。全量 build/vet/test 过（含 PocScan/cmd/fscanx-web/Plugins/common）。置已验收，POC 扫描模块 4 item 全部闭合。
- 2026-11-15：**前端重做轮（用户反馈原生单页体验不合格，指定 vue-element-plus-admin 3.1.0 重做，且未来非 POC 需求也会进 web 端）**。落地：框架 monorepo 入仓 `web/`（裁掉 docs 子应用，pnpm workspace：apps/admin + packages 4 共享包）；Go 响应统一包装 `{code:0,data}`（@vea/request 拦截器契约，writeJSON 单点改造）；新增 `GET /api/routes` 后端动态菜单（vue-element-plus-admin 动态路由 schema，component 'views/Xxx/Xxx' 字符串映射，未来加页面=后端加一条）；免登录（main.ts 注入本地会话，本地维护端无权限模型）；`web/build.sh` 一条龙（pnpm build → dist-pro → cmd/fscanx-web/static embed 真源），dev 模式 vite :4000 代理 /api→7758。三页面迁至 Element Plus（POC 谓词表单/绑定 el-autocomplete 产品下拉/指纹+打包）。回归：验收①③ API 链路实证复过（表单新建 vue-demo 端到端命中、非法拦截），②④⑤ API 语义未变（writeJSON 仅加包装层）；基线框架官方构建先行验证（2.23s）后定制。过程坑三条入记忆：残留旧进程占 7758 致新产物验证假阴性（tasklist 核对后清）；Binds.vue import 路径笔误 bind→binds；vite proxy 原 rewrite 会剥 /api 前缀（删除）。**UI 观感待用户浏览器确认**（本轮原始诉求是体验，链路验收不替代观感验收）。
- 2026-11-15：**重做后登录页问题排查修复轮（用户问"登录密码是什么"暴露）**。本维护端是免登录设计（无密码），用户看到登录页=bug。三根因串行修复：① Root redirect 指向框架残留的 `/dashboard/analysis`（动态菜单无此页）→ 404 → Fallback 兜底壳，改指 `/poc/list`；② 框架模板 `plugins/elementPlus` 仅注册 ElLoading，el-table/el-form 等全被解析为原生元素、scoped slot 被无参调用报 `Cannot read 'row' of undefined`、内容区空白——改 `app.use(ElementPlus)` 全量注册+全量 css（el-table-column slot 统一 `#default="scope"` 官方形态）；③ **生产产物独有**：vite 构建产物含 `_plugin-vue_export-helper-*.js`（下划线开头），`//go:embed static` 按规则排除 `_`/`.` 开头文件 → 嵌入缺失 → 运行时 404 → Vue 应用挂载失败白屏——改 `//go:embed all:static`。③是 dev 模式验证盲区（vite dev 从磁盘服务文件，不经过 embed），教训：embed 前端产物必须以**产物构建版**终验。终验（headless msedge 对 7758 产物版）：`#/poc/list` 直进、POC 表格 390 行、绑定 15 行、指纹页按钮齐全、免登录态显示 local、零 console 错误、零 404；截图 analyze_image 复核布局控件完整。
- 2026-11-15：**用户真机 UX 反馈修复轮（两条）**。① POC 列表点击行右侧无规则内容：根因 `@row-click="(row) => openPoc(scope.row.name)"`——前轮 slot 写法批量替换误伤，el-table 事件回调（参数 row/column/event）无 slot scope，点击即抛 `ReferenceError: scope is not defined`，openPoc 从未执行；修回 `row.name`。② 绑定关系页 POC 列手写 textarea（一行一个 name）UX 差：改为点选下拉（`el-select multiple filterable collapse-tags collapse-tags-tooltip`，选项=listPocsApi 全集 name，Row.pocsText:string → pocs:string[]，save 去 split 逻辑）。产物版 headless 终验：点击首行编辑器正确展示（标题带 poc 名、源码 textarea 719 字符、条件行 1 条）、绑定页下拉选项含全集（首项 poc-yaml-74cms-sqli-1）、零 console/page 错误。
- 2026-11-15：**always 迁移 + 表单多步结构化轮（用户两连问："always 怎么会放置在绑定关系页面？这不是对 poc 设置吗" / "凭什么不能编辑该 poc？你确定没法做到？"）**。**always 迁移**：`Bind.Always` 退役、`lib.Poc` 加 `always` yaml 顶级标记，PocScan 空态 = `boundPocNames(frames)` ∪ `alwaysPocNames()`（遍历 AllPocs）去重执行；旧 binds.yaml 残留 always 键按 yaml 非严格解析忽略不炸；web 端开关移入 POC 编辑器（绑定页删 always 列）。**表单覆盖面扩展**（一期"超出表单子集切源码"的保守裁剪被用户否决）：getPoc 返回多步结构化 steps[]（每步 method/path/headers/body/search/follow_redirects/continue/expression/tree 全字段，字段使用率实测 follow_redirects 63%/body 38%/headers 32%/search 7% 必须全覆盖否则表单保存丢字段改行为）；谓词嵌套树——predicate.go `Node.Conditions []*Node` 递归结构与 `_&&_/_||_` 递归下探**原本就支持任意嵌套**，裁剪的只是前端（新增 CondTree.vue 递归组件：组节点 AND/OR 切换+加条件/加子组，任意深度）；超出谓词子集的步骤（含 set 变量函数调用等）降级为**步骤内 CEL 文本编辑**（不再整卡切源码，仍无需写 yaml）；set/sets 变量折叠面板编辑；groups 形态（31/386 长尾）表单不承载返回 form=false 前端自动源码模式（保存走原文不丢字段）。**表单保存保真设计**：表单管辖字段（name/always/set/sets/rules）从表单值渲染，其余顶级段（detail/groups/manual/transport）从打开时原文（baseYaml）提取拼接——未暴露字段零丢失；yaml 渲染借 yaml.Marshal 小结构保转义（手拼引号在 xml/正则 body 必翻车），落盘前 Unmarshal round-trip 双重校验。**顺带修真 bug 一条**：落盘 CEL 编译校验用裸环境，含 set 变量的 expression（内置 166/386 用 set）必 400——补 `envWithSetVars`（与求值端 lib/eval.go 前缀类型规则同源：randomInt→int、newReverse→Reverse、其余→string）。验证：render_test 单测（渲染 round-trip+段保真+转义）；API 端到端 17/17（多步 poc 全字段回填/嵌套树 round-trip/headers/body/detail 保真/旧 rules 替换/always 落盘/binds 无 always 可写/非法拦截）；**fscanx 实测闭环**：无指纹目标（python 静态服务）+ 空 -pocname → always-demo 每轮必发 `/always-marker` 且 `[+] PocScan ... poc-yaml-always-demo` 命中，同目录其余非 always poc 零请求；UI headless：多步 poc（POST+headers+body+set）表单模式直接承载（advanced 步 CEL 文本+tag）、绑定页无 always 列、零错误。核心包测试全过（mylib/proxy、socks 为既有 internal 包引用失败，与本轮无关）。
- 2026-11-15：**CondTree 回归补丁（用户反馈『点 +子组 没反应』）**：CondTree.vue script 漏写 addCond/addGroup 两个函数（首版 script 不完整，模板引用悬空；构建链是纯 vite build 无 vue-tsc 模板类型检查故漏网，点击抛 ReferenceError 表现为无反应）。补函数（加条件默认 status 空值待填、加子组自带 and 组+初始条件），『+ 子组』文案改『+ 嵌套子组』。产物版 headless 真实点击链验证：点+条件行数 1→2、点+嵌套子组出现嵌套虚线组、组内再点+条件总行 4、CEL 预览只含已填值条件（半成品剪枝是设计行为）、零错误。教训：交互组件的按钮必须交互验证，加载零错误不等于可用。
- 2026-11-15：**多步执行语义 UI 修正（用户问"多步骤怎样才算验证成功"）**。核实 check.go 引擎事实：普通模式 DealWithRules 逐 rule 执行，任一步 expression 为 false 或 search 提取不到即 break 整体失败，**每步（含最后一步）都满足才输出 PocScan 成功行**；`rule.Continue` 仅爆破模式（sets 笛卡尔积 clusterpoc 路径）被消费（内置仅 backup-file/sql-file 2 个 sets 型 poc 使用），普通模式写了也不读。据此修正 UI 三处：步骤区标题加执行语义说明（任一步不满足即停止、每步都满足才算成功）、continue 开关改 isFuzzMode 条件显示（普通模式隐藏，避免无效字段误导，文案改"爆破模式专用：组合命中后继续尝试其余组合并逐条输出"）、search placeholder 补"提取不到则该步即失败"（search 是门槛不是软提取）。产物版 headless 验证：语义说明在、普通模式 continue 隐藏、search 文案已更新、零错误。
- 2026-11-15：**步骤区页签化（用户建议：多步用"步骤 1/步骤 2"子页切换 + 页签 × 删除）**。el-card 纵向堆叠改为 el-tabs（type=card，closable 页签自带 ×，tab-remove 删除）；新增步骤自动跳到新页签；删除后激活页跟随原内容修正（删前页则前移、删当前页则停原位吃掉原下一步）；至少保留一步（× 拦截 + 提示）；打开/新建重置到第一页。交互链 headless 验证：初始单页签、+添加跳新页、各页内容独立、× 删除后页签与激活页正确、单步 × 拦截、零错误。
- 2026-11-15：**编辑器排版纪律轮（用户反馈"字体大小和左右对齐乱乱的"，frontend-design 流程）**。病灶定位：CondTree 等多处控件未传 size=small（32px/14px 混进 small 的 24px/12px）、el-form inline 三套左起点（90px label 悬浮/顶格 flex/collapse 两列）、说明文字 12/13px 混杂。改造：全部控件统一 small（24px 高）；弃 el-form 改自定义 field 行（label 列统一 60px 右对齐）；区标题统一 section-title（13px/500）+ sub 说明（12px/secondary）；说明文字缩进 68px 对齐控件起点；垂直节奏 4px 基数；tabs 高度 34px、collapse 头 36px 对齐密度。量化验收：控件高度集合收敛 [0,24,32]→[24] 单值；label 宽度集合 [60] 单值；左基线度量——基本区 label/步骤区 label/变量区输入/headers 输入/区标题 x 坐标全部 673 同一坐标，hint 741=673+68 精确缩进；零错误。

## BL-POC-005 绑定关系页交互重构 + POC 页快捷绑定

- 模块：POC 扫描
- 优先级：P1
- 状态：已完成
- 当前阶段：持续迭代
- 价值：绑定维护主流程可用性——浏览态零干扰（下拉只在显式编辑意图后出现）、添加/编辑走显式表单弹窗、行数增长可翻页、POC 维护免跳页直接绑定当前 POC；binds 存储层 SQLite 化后单条 CRUD、种子绑定可编辑可删除（墓碑）、手改文件自动吸收。
- 关联模块文档：docs/modules/10-POC扫描模块.md
- 关联计划：plans/2026-11-15-BL-POC-005-绑定UX与POC页快捷绑定.md
- 关联代码：web/apps/admin/src/views/Binds/Binds.vue、web/apps/admin/src/views/Poc/Poc.vue
- 验收状态：已验收

### 方案（立项时必填）

- **做什么**：绑定页表格从全行内联编辑改**只读展示**（产品名文本 + POC 名 tag），交互收敛为三个显式动作：添加绑定（弹窗表单）、行编辑（同一弹窗回填）、删除（确认即删）；产品名 autocomplete 与 POC 多选只存在于弹窗内，浏览态点击不弹任何下拉（用户拍板：编辑态才出现候选）。增删改确认即写盘外置 binds.yaml（批量保存按钮随未保存态消失而移除）；客户端分页（默认 20/页）；产品名候选排序改精确>前缀>子串 + 命名空间语义提示（绑定命中=Contains(指纹产品名, 绑定产品名)，写引擎不会输出的名字不生效）；同名产品在添加/编辑时拦截（与引擎 MergeBinds 按 product 去重语义对齐）。POC 页编辑器头部加"绑定产品"按钮（选中 POC 后可用）：弹窗展示该 POC 已绑产品（只读 tag）+ 多选新增，确认 GET→合并→PUT 写盘。
- **为什么**：用户两轮实测反馈——①无翻页；②"添加绑定"是往表尾插空行不可发现，应改表单；③产品名输入框聚焦即弹"与当前产品名不一致"的候选（el-autocomplete 聚焦触发 + 子串模糊过滤全集），语义困惑且浏览态不该有交互控件；④POC 页无快捷绑定入口。
- **怎么做**：纯前端改造（Binds.vue 只读表格+分页+内联 el-dialog CRUD、Poc.vue 绑定按钮+弹窗），后端零改动（GET/PUT /api/binds 全量覆盖契约不变，POC 页快捷绑定前端完成合并后 PUT）。
- **为什么这么做**：浏览态与编辑态分离是用户明确要求（点击即弹下拉莫名其妙）；弹窗化 CRUD 后不再存在未保存态，批量保存按钮成为死 UI 故移除，确认即写盘消除"改了没保存"坑；否决服务端分页与单条绑定 API（数据量=种子 15 行+用户增量，PUT 全量契约已存在且为单用户本地维护端）；候选排序精确>前缀>子串从根上缓解"跳出不相关名字"观感而非仅靠文案解释。

### 范围

- web/apps/admin/src/views/Binds/Binds.vue（只读表格、分页、添加/编辑弹窗、删除确认、候选排序与文案）
- web/apps/admin/src/views/Poc/Poc.vue（编辑器头部绑定按钮 + 绑定弹窗）
- bash web/build.sh 产物构建 + go:embed 产物版 headless 终验

### 不做

- 后端 API 变更（GET/PUT /api/binds 契约不变）
- 服务端分页
- POC 页的解绑入口（移除走绑定关系页）
- 产品名强制白名单校验（手输仍允许——指纹明文可自定义扩展，下拉只是候选）

### 验收条件

- [x] 绑定页表格浏览态零交互控件：点击产品名/POC 名/行不出现任何下拉；每行有编辑、删除按钮（headless 实测表格内 input/select/autocomplete 计数=0，20 行均有编辑/删除钮）
- [x] 添加绑定走弹窗表单（产品名 autocomplete + POC 多选），确认后写盘 binds.yaml（实测添加 Nginx→2 poc，GET /api/binds_list 回读一致）
- [x] 行编辑弹窗回填生效（回读验证）；同名产品在添加/编辑时被拦截并提示（实测改名 seeyon→seeyon-edit-check 回读生效后改回；重复添加弹 warning 且不产生第二条）
- [x] 产品名候选排序：输入 nginx 时精确/前缀命中排前（Nginx 类）、纯子串靠后（实测候选序 ["nginx","nginx-admin","neusoft-senginx"]，前缀全部先于子串）；弹窗含命名空间语义提示文案
- [x] 删除：确认后该条从 binds.yaml 消失（实测删除后回读无此产品）
- [x] 行数 >20 时分页出现，切页/改页大小正常；增删改全量写盘不受当前页影响（实测 26 条→2 页，切页/10条每页/重载均正确，增删改不受当前页影响）
- [x] POC 页选中 POC → 绑定产品弹窗（已绑产品只读 tag + 多选新增）→ 确认后 GET /api/binds_list 回读该 product 的 pocs 含此 POC 名（实测 ipTIME-X6003 绑定回读含当前 POC，重开弹窗 tag 展示）
- [x] fscanx 实测闭环：新建绑定（指纹可命中产品 → 新 poc）落盘后重跑扫描器，定向发出新 poc 请求（实测绑定 D-link-DSL-2640B→poc-yaml-dlink-850l-info-leak，假目标页含指纹关键字，扫描器 `[+] load 13 binds` 消费 web 落盘文件，目标访问日志仅 首页+favicon 探测+该 poc 的 POST /hedwig.cgi 一条 poc 请求）
- [x] bash web/build.sh 产物构建成功、go build ./cmd/fscanx-web 通过；headless 浏览器对 7758 产物版验证绑定页/POC 页交互链零 console 错误（go:embed all:static 产物版终验，实测 27/27 零 console/page/HTTP 错误）

### 依赖

- 无（BL-POC-004 已闭合，web 框架在仓）

### 状态记录

- 2026-11-15：立项（用户批准 P1，Mode B 修订两轮——v2 按用户拍板把"文案解释下拉语义"升级为"浏览态/编辑态分离 + 编辑按钮"交互模型；独立审查 agent 轮被用户打断，改为主会话基于亲读代码自查 A-H 八项事实声明，全部属实）。
- 2026-11-16：**执行完成 + 范围扩展（用户批准：binds 存储层 SQLite 化纳入本 item，"指纹也可以这样做"记为 006 方向）**。交付三层：①前端交互——绑定页只读表格+分页（10/20/50）+添加/编辑共用弹窗+删除确认，浏览态零下拉；POC 页编辑器头部"绑定产品"弹窗；产品候选排序精确>前缀>子串+命名空间提示文案。②存储层——`cmd/fscanx-web/binds_db.go`：SQLite（modernc.org/sqlite v1.29.10，Go1.20 兼容线）主存外置层 + binds.yaml 快照双轨；同步机制=单向权威+手改吸收（每请求前比对文件 sha256 与 meta，不一致=被手改→全量吸收→重生成；文件缺失按 DB 重生成防误删；坏 yaml 显式报错）；API 改造为 /api/binds_list、/api/binds_add（upsert-append）、/api/binds_edit、/api/binds_delete，PUT /api/binds 退役。③墓碑机制——种子起源行（仅存在于 embed 种子）的编辑/删除：落 {product, pocs:[]} 墓碑条目，引擎 MergeBinds 同名覆盖后空 pocs=绑定失效（boundPocNames 的 add([]) 空操作），合并视图过滤墓碑不展示，扫描器零改动。**意外发现一（重要）**：种子起源行经合并视图编辑/删除会静默失效——此缺口 BL-POC-004 时代即存在（旧 PUT 全量覆盖同样删不掉种子条目，界面看似成功引擎侧无效），本轮墓碑机制修复；单测此前只测外置层 CRUD 未暴露。**意外发现二**：种子命名与指纹库命名空间存在漂移（种子 'seeyon' vs 指纹库 '致远互联-Seeyon-Server'），Contains 大小写不敏感语义下种子名仍可命中，无碍但记录在案。验证：单测 19/19（CRUD/吸收往返/墓碑三件套/幂等删除/handler 级回归）；UI headless 产物版 27/27 零 console 错误；fscanx 实测闭环（见验收条件末条）。
- 2026-11-16：**Reviewer 独立验证轮（sonnet 档，10 findings：1H/2M/7L）→ 逐条修复后复验 27/27 全过**。HIGH 墓碑幂等漏洞修复：重复删除墓碑行会移除墓碑导致种子绑定复活——改为幂等成功保留墓碑；墓碑行作 original 改名同理保留墓碑。MEDIUM×2 修复：①快捷绑定到种子起源产品改为以种子 pocs 为基线追加（原实现单 poc 条目整条覆盖种子绑定）；②错误消息改 200+{code:1,message} 信封（原非 2xx 纯文本被 axios 丢弃成 status code 文案，指导性错误如"手改内容有误请修正"到不了用户）。LOW×5 择机修复：吸收对同名重复 product 后者覆盖（不再主键冲突锁死接口）、dbMutate 注释修正为"自愈靠下一次写操作无条件 flush"、readJSON 强制 application/json Content-Type（跨站无预检写请求被挡）、Poc.vue 部分失败提示"已绑定 x/y 项"、go.mod indirect 标记修正。**方法学记录（非产品 bug）**：headless playwright 信任点击与 Element Plus clickoutside/焦点时序存在竞态（DOM 监听触发但 Vue 处理器偶发不执行，probe9/10 实证），产品代码无涉——测试脚本以"点弹窗标题收尾 popper + 点击后校验效果补点"规避。
- 2026-11-16：验收条件全勾（实测证据见各条），置已完成+待验收。UI 观感（排版/交互手感）待用户浏览器确认后验收闭合。
- 2026-11-16：验收闭合（用户浏览器实测确认测试通过）。置已验收。

## BL-POC-006 指纹库 SQLite 存储化

- 模块：POC 扫描
- 优先级：P2
- 状态：未开始
- 当前阶段：持续迭代
- 价值：指纹维护从"textarea 整文手改 + 全文回写"升级为单条目 CRUD（增删改查按条目落库，免整文搬移）；SQLite 主存 + 明文 JSON 快照双轨后，手改文件自动吸收、重打包管线不变、扫描器零改动（与 BL-POC-005 的 binds 存储化同模式，方向用户已认可）。
- 关联模块文档：docs/modules/10-POC扫描模块.md
- 关联计划：—（未开始）
- 关联代码：cmd/fscanx-web/binds_db.go（同模式参照）、cmd/fscanx-web/api_finger.go、mylib/finger/resources/、mylib/finger/packlib/
- 验收状态：未验收

### 方案（立项时必填）

- **做什么**：参照 binds_db.go 模式把两份指纹明文（goby.json / fingers_http.json）的条目级维护迁入 SQLite（指纹库主存，按数据源分表、条目以 JSON blob 落库保结构零建模成本）：单条目增删改查 API；每次变更后按既有格式重生成明文 JSON（temp+rename 原子写）并记录哈希；读写前哈希对齐吸收手改（与 binds 同机制）；pack 重打包链路消费明文 JSON 不变（产物 .gz 供编译嵌入，扫描器零改动）；web 端指纹页从 textarea 整文编辑升级为条目列表 + 单条目表单/JSON 编辑。
- **为什么**：现状指纹维护是"textarea 整文改 + 整文回写"——单条目修改要人肉定位数千条 JSON 中的目标段，格式敏感（BL-POC-004 曾踩 \u 转义/整体重序列化坑）；条目级 CRUD 后修改面收敛到单条目，格式由重生成器保证。用户在 BL-POC-005 验收轮明确认可同方向（"并且指纹也可以这样做"）。
- **怎么做**：fingerprint.db（条目表 + meta 哈希表）→ 明文 JSON 重生成器（按源格式序列化，格式契约=pack 现行输入）→ 哈希吸收（同 binds）→ API 按页面接口命名规范（/api/finger_list 等）→ 前端条目列表页。
- **为什么这么做**：与 binds 存储化同一分层决策（DB 主存 + 文件快照 + 哈希吸收 + 墓碑语义如需删指纹条目）；pack 管线与扫描器零改动（明文 JSON 仍是 pack 唯一输入）；否决"运行时外挂加载指纹"（用户既定拍板：指纹保持内嵌）。

### 范围

- cmd/fscanx-web/（finger_db.go 存储 + api_finger.go 改造）
- 前端指纹页条目化改造
- mylib/finger 明文 JSON 格式契约固化（重生成器与 pack 输入一致）

### 不做

- 指纹运行时外挂加载（保持编译嵌入）
- 引擎 Match 逻辑变更
- 指纹内容治理（误报清理等归 05 模块）

### 验收条件

- [ ] 单条目增删改后明文 JSON 重生成与 pack 字节兼容（pack 产物 md5 与手改等价基线一致）
- [ ] 手改明文 JSON 被吸收进 DB（哈希对齐），坏 JSON 显式报错
- [ ] 条目级 API 与 UI CRUD 实测闭环
- [ ] headless 产物版交互链零 console 错误

### 依赖

- BL-POC-005（存储层模式与墓碑语义先例）

### 状态记录

- 2026-11-16：方向立项草案（用户在 BL-POC-005 范围确认时批注"并且指纹也可以这样做"）。优先级 P2 为草拟值，**待用户确认优先级后 committed 转 Mode D**。

## BL-POC-007 fscanx-web 登录认证（默认账号）

- 模块：POC 扫描
- 优先级：P1
- 状态：已完成
- 当前阶段：持续迭代
- 价值：维护端具备真实写能力（binds/poc/指纹改写），从免登录硬注入改为最小认证门——部署端口不再裸奔，写接口有身份门槛。
- 关联模块文档：docs/modules/10-POC扫描模块.md
- 关联计划：plans/2026-11-16-BL-POC-007-web登录认证.md
- 关联代码：cmd/fscanx-web/auth.go、cmd/fscanx-web/main.go、web/apps/admin/src/main.ts、web/apps/admin/src/api/login/index.ts
- 验收状态：已验收

### 方案（立项时必填）

- **做什么**：后端 `/api/user/login`（校验默认账号 admin/admin123，签发 crypto/rand 内存会话 token）+ `/api/user/logout`（销毁 token）；中间件对 `/api/*`（除 login）校验 Authorization token，无效返回 200+{code:401,message} 信封——前端拦截器既有链路自动 logout+回登录页（零前端守卫改动）。前端：登录 API 指向真实端点、删 main.ts 免登录硬注入、logoutApi 指向真实登出。
- **为什么**：用户指令"现在需要登录，搞一个默认账号 admin/admin123"。免登录是 BL-POC-004 期临时形态（main.ts 注入 accessToken:'local'），维护端写能力增强后（005 起含 SQLite 主存 CRUD）认证缺位成为裸奔面。
- **怎么做**：凭据常量 + 内存 token 表（进程生命周期，重启即失效，前端 401 信封自动登出再登录）；token 走既有 Authorization 头（拦截器已发送 userStore.token，接受裸 token 与 Bearer 前缀）。
- **为什么这么做**：本地单用户维护端——否决多用户/角色/密码哈希存储/防爆破限速（成本与威胁不匹配；凭据即配置，改密码=改常量重编译或后续再参数化）；复用 401 信封让前端"会话过期自动登出"链路零改动（permission.ts 守卫与拦截器均已按该契约实现）；静态资源不设防（登录页本身要能加载）。

### 范围

- cmd/fscanx-web/auth.go（新：凭据/会话/登录登出/中间件）+ auth_test.go
- cmd/fscanx-web/main.go（路由注册 + mux 包中间件）
- web/apps/admin/src/api/login/index.ts（真实端点）、src/main.ts（删注入）

### 不做

- 多用户/角色权限模型
- 密码可配置化（flag/env/存储）——后续有需要再参数化
- HTTPS / 防爆破限速（本地回环工具语义）
- 注册流程（框架模板残留组件不接后端）

### 验收条件

- [x] 错误凭据登录被拒并提示；admin/admin123 登录进入且 /api/routes 正常加载（实测 headless：wrongpass 提示"fscanx-web: 用户名或密码错误"，admin/admin123 登录后离开登录页、绑定页 15 种子行加载）
- [x] 未登录直接访问页面路由重定向登录页；登录后刷新保持会话；服务端重启后旧 token 401 自动登出（实测 A1 重定向 /login?redirect=/binds/list；A5 刷新仍在；curl 登录→重启→同 token 调 /api/binds_list 返回 401）
- [x] 未带/伪造 token 调 /api/* 返回 {code:401} 信封（实测伪造 token 注入 localStorage 刷新被清回登录页；curl 无 token 调 binds_add 返回 401——跨站盲写面同步封死）
- [x] logout 后服务端 token 失效（单测 TestUserLogout_InvalidatesToken + headless A6/A7 登出回登录页、直访再被拦）
- [x] go test 单测过（登录成功/失败/Content-Type 拒绝/登出失效/中间件拦截与放行/Bearer 前缀/静态与 login 白名单，7 测试全过）
- [x] bash web/build.sh 产物版 headless：登录全链 9/9 零非预期 console 错误（错误密码的拦截器 throw 属预期行为已排除口径）

### 依赖

- BL-POC-005（写接口是被保护对象；401 信封机制先例）

### 状态记录

- 2026-11-16：用户指令"现在需要登录，那就搞一个默认账号admin/admin123"直接立项（视为范围批准，P1 立即执行）。免登录是 BL-POC-004 期临时形态（main.ts 注入 accessToken:'local'），permission.ts 守卫与拦截器的 401 登出链路本就按真实认证设计，本次只是补上服务端。
- 2026-11-16：执行完成。交付：auth.go（凭据常量 admin/admin123 + crypto/rand 内存会话 token + /api/user/login|logout + authMiddleware：/api/* 除 login 白名单，无效 token 返 200+{code:401} 信封）；main.go 路由+中间件+横幅打印账号；前端登录/登出 API 换真实端点、删 main.ts 注入与未用 import。验证：单测 7/7、headless 产物版登录全链 9/9（重定向/错误提示/登录/加载/刷新保持/登出/再拦截/伪造 token 401 自动登出/零非预期错误）、curl 重启失效与授权写/无 token 写 401。**意外发现（方法学）**：构建顺序坑——go build 先于 web/build.sh 时二进制 embed 旧前端，浏览器拿旧 UI 造成假失败（登录提交打到 /mock/user/login），正确顺序=前端产物先行再 go build；与 go:embed 产物版终验教训同源已入记忆。附带收益：认证门同时封死了此前 reviewer 提到的跨站无预检盲写面。置已完成+待验收。
- 2026-11-16：验收闭合（用户浏览器实测确认测试通过）。置已验收。
