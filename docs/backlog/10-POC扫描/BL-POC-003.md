# BL-POC-003 指纹定向触发管道

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
