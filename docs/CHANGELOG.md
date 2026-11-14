# CHANGELOG

## 2026-11-14

- BL-DIR-001 与 BL-MEM-003 Mode F 验收闭合（用户明示决策）：DIR-001 接受"17 项单测 race 全过 + 三轮独立审查闭环 + 修复带回归测试"作为行为级修复回归证据（豁免复扫）、-msc 路径接受代码审查覆盖；MEM-003 接受"交叉编译验证 + mmap 失败自动回退 ReadAll"作为运行时证据；验收状态置已验收并从控制台移除，本会话三项全部闭合
- BL-001（-dp 域名端口绑定）Mode F 验收闭合：四条验收条件真机实测补证（baidu.com 对照 -dp 开/关与 -h/-hf 两入口，独立审计复跑证据吻合），验收状态置已验收并从控制台移除；同轮修正设计总览 04 行与 04 模块文档 mermaid 图的判定闸过时口径（404/5xx→状态码 ≥300 全短路）
- 目录扫描第三轮审查复验收口（BL-DIR-001）：复验抓到 noteSiteOK 移位引入的回归——≥300 短路（先于 body 读取）路径执行不到清零，间歇超时+快 404 交错的站点 3 次非连续超时即误熔断；短路 return 前补 noteSiteOK（头快速到达即存活），补 TestFast4xxClearsFailCounter 回归
- 目录扫描第三轮独立审查修复（BL-DIR-001，热修代码整体复审）：修复 body 黑洞熔断不可达（旧条件 len(body)==0 永假）及修复后暴露的计数清零交互（noteSiteOK 从"响应头到达"移至"body 完整"，否则黑洞路径 consecFails 永远振荡在 1）；progressLoop 纳入收尾 join（堵 close(Results) 后 send 窗口）；模块文档阈值残留、聚合器释放补 seedFps、死代码清理；补黑洞接线与双熔断交织回归测试。race 全量 2 轮过
- 目录扫描诊断画像输出注释（BL-DIR-001 真机复测通过，用户决策）：慢站点画像行完成排查使命暂时注释（统计与函数保留，dirscan.go 内恢复注释块即可启用）；进度快照与 abort 熔断提示为功能行为保留
- 目录扫描连续慢熔断与画像判定修正（BL-DIR-001 二轮真机画像定性）：120.77.216.208 实证"慢而成功"型拖尾（avg 5.1s/条、70 slow、仅 1 failed——失败熔断抓不住，单站 355s 拖死收尾）——新增连续 3 条 >2s 慢路径熔断（与失败熔断对称，快路径清零），该类站点收敛到 ~15s；画像判定改纯路径耗时总和（多站点轮询的排队假象行消失）
- 目录扫描慢站点画像计时修复与升级（BL-DIR-001 首轮真机画像回收）：修复 defer 参数注册时求值致单路径耗时恒 0 的统计 bug；画像增加 avg 均值与 failed 失败条数（区分"慢成功"与"间歇失败"型拖尾——真机 120.77.216.208 单站 353s≈5s/条拖尾收尾、熔断因偶发成功清零抓不住，为下轮治理选型提供数据）
- 目录扫描慢站点诊断画像（BL-DIR-001 真机反馈"最后几十个特别慢"）：每路径耗时统计，站点扫完总耗时超 10s 打一行画像（url/路径数/最慢单路径/慢路径条数）——定位"慢而成功"型收尾拖尾（熔断只拦连续失败拦不住这类），快站点零输出不刷屏
- 目录扫描注册门槛入口拦截（BL-DIR-001 用户洞察"根路径都拿不到响应为何还目录扫描"）：DoWebScan 传出 body 读取健康位，首页链（初次/https重试/跳转）全部 body 黑洞的站点不再注册——同一服务器目录路径必然同样超时，从源头省掉纯超时浪费；任何一次 body 完整即健康（跳转后正常不误伤），首页行输出不受影响；熔断保留兜底中途变死站点
- fingers 指纹库裁剪为 hash-only（BL-FP-003，用户拍板）：首轮清理后误报仍偏高，删除全部文本特征指纹——3770 条纯文本整删、124 条混合指纹修剪文本特征只留 favicon/body 哈希，4289→519（favicon 518 + body 哈希 1）；gz 由 115KB 降至 11KB。此后 fingers 库只接受 hash 类指纹，文本指纹由 goby 引擎（logic 布尔表达式）承担
- README 重构（BL-DOC-001，用户三决策：重组+精简/大自述迁档/更新记录留尾部）：592→324 行，按读者问题重组为定位→功能一览→快速开始→使用案例（10 场景）→参数速查（以 common/flag.go 70 flag 为真源）→维护指南→更新历史→关于；2025.11.25 大自述整体迁 docs/releases/2025-11.md（含 socks5 误报谜题、内存实测，原文保留）；剔除已失效的 fingerprint.txt 说明；删除 resources 工作副本目录内两份无引用历史 gz
- 新增纯 Go 指纹重打包工具（BL-FP-002，用户拍板保留 gz 体积+显式打包脚本）：`go run ./mylib/finger/cmd/pack` 替代 gzip 外部命令（Windows 无内置 gzip），compress/gzip 标准库跨平台；json.Valid 前置校验防打包坏 JSON；mtime=0 + BestCompression 确定性输出（内容不变则 gz 字节不变）；README 维护说明与 resources/cmd.txt 同步改写
- fingers 指纹库误报清理首轮（BL-FP-001，用户拍板档3激进+2字中文公司名全删）：删除六类"极可能误报"指纹 60 条（通用功能短语 everything/web user login、HTTP 标准头 alt-svc/authenticate、厂商名报具体产品 huawei/Cisco/Juniper/金山等、同词撞车 juniper→2产品与 X-Powered-Cms 裸头→3产品、冷门词典词 guppy/pothos、知名产品词典词 harbor/zeppelin/express）+ 2 字中文公司名 13 条；奇安信设备/金山设备两条混合指纹仅删弱特征值保强特征。4349 → 4289，validate 全量通过；359 组大小写重复指纹属数据冗余非误报，另行立项
- 目录扫描站点熔断与进度快照（BL-DIR-001 真机反馈"部分网段扫描很久"）：排查确认超时已跟随 -wt 无缺陷、端口不存活站点不注册；根因为半死站点（头能回/body 黑洞）注册后每路径烧满超时、单站点串行 70×9s 拖死收尾——加连续 3 次失败熔断（跳过剩余路径并提示，阈值经用户由 5 调低），最坏拖尾收敛到 3×超时；新增每 30s 且有新进展的进度快照行（时间驱动不刷屏，快扫描不可见）
- 目录扫描 4xx 全屏蔽（BL-DIR-001 真机反馈，用户决策）：401 经取舍价值也低（仅极少数产品靠响应头特征可识别）——短路简化为状态码 ≥300 一刀切（3xx/4xx/5xx 全丢，仅 2xx 参与识别）；补 401 带 WWW-Authenticate 特征仍零输出的回归用例
- 目录扫描空标题规则收紧（BL-DIR-001 真机反馈，用户决策）：200 + 空标题的路径不算标题差异（"None" 是占位符），退化为纯指纹差——全部指纹为首页已见则丢、有新指纹才输出，消除"首页 (nginx,真实标题) vs 路径 (nginx,None) 空页"类噪声；补自适应回归用例
- 目录扫描输出体验（BL-DIR-001 真机反馈）：-dir 开启时跳过普通模式 `[*] ok: 1/1` 结尾行（Printf 直写会插队到 Results 通道排队的目录行前，体感错乱）；站点完成进度行经实测反馈打印过多已撤销；详细功能说明迁至模块文档「使用行为说明」节
- 目录扫描屏蔽 403（BL-DIR-001 真机反馈，用户决策）：全站 403 的 WAF/安全设备使每条路径都产出"403+Server 头指纹"噪声——403 加入短路不参与指纹识别与输出（401 保留）；补 /denied 回归用例
- 修复目录扫描 302 误报（BL-DIR-001 真机验收反馈）：拒绝跟随的 302 响应本身被指纹引擎识别（Location 头含 "thinkphp" 字样误报 thinkphp）——3xx 一律短路丢弃，跟随成功的落地响应才进判定；补带指纹 302 的回归用例
- 新增指纹导向目录扫描 `-dir`/`-dirf`（BL-DIR-001，P0）：轮询环调度器按"(站点,路径)对"交错探测（单站点在途≤1，流量均摊不轰单点）、两阶段停机（Seal→Drain）兼容渐进注册；四道判定闸（404/5xx/3xx 短路→302 同源同前缀跟随→指纹非空→(指纹,标题)呈现差集）免疫两类软 404 并保住"同产品后台入口"场景；站点聚合输出同构 Product 行（url 带路径、下游零适配），聚合器站点级生命周期；内置 70 条字典 go:embed（-dirf 覆盖），默认关闭
- 配套改造：DoWebScan 增加呈现返回值（全链收集首页(指纹,标题)并集）、WebScanWorker 成功后按原始 URL 注册（err 不注册）、Scan/ScanFromReader 含 -m icmp/portscan 四处收尾接入 SealAndDrain
- 单测 11 项（race）抓出并修复 4 个实现 bug：dirWorker 缺 recover、RegisterSite 持锁 return 死锁、loadDirPaths Once 重置叠加、nil seeds 误杀合法站点；reviewer（opus）复验抓出第 5 个：SealAndDrain 未等 worker 拖尾（全量 race 多轮 FAIL + LogWG 顺序契约缺口），以 workers.WaitGroup 修复后 APPROVE
- 实测：真实二进制 -dir 冒烟（隐藏路径目标 3 findings：/seeyon/、/admin 301 跟随保留原始路径、/jeecg-boot/）；无 -dir 与 HEAD 输出逐行一致（零侵入）；-std 管线冒烟正常退出

## 2026-11-13

- 日志管道 IO 优化（BL-MEM-003）：去掉每条结果一次的 fl.Sync()（WriteString 直写内核页缓存，进程终止不丢已写行）、Results 通道加 256 缓冲吸收毛刺；不引入 bufio（用户态缓冲进程死即丢，负优化）
- qqwry.dat 装入改造（BL-MEM-003）：unix 经 syscall.Mmap 按需映射（无归属地查询时 RSS 近零，失败回退 ReadAll），windows 直接 ReadAll；加载失败打印提示并降级，不再吞错误
- 死代码清偿（BL-MEM-003）：删除 mylib/simplehttp 整包（676 行）、icmp.go.bak、portscan.go/webScanTask.go 注释旧实现约 340 行、main.go MemStats 调试块、scanner.go 恒空 alivePortResList 与 ScanUrl 死标签、webScanTask.go 的 targetInfo=nil 与 resp.Body 双重 Close；净 -1319 行，扫描行为与输出格式零变化（新旧输出对比实测一致）
- 修复 -msc 的 Ctrl+C 退出（BL-MSC-002）：第一次信号停止 masscan 并最多等 3 秒，超时或第二次信号清理临时文件并以 130 退出；中断不再打印 masscan mode failed
- BL-MSC-002 用户真机测试确认 Ctrl+C 有界退出/二次强退，验收状态置已验收
- README 更新记录补充 masscan 内嵌联动模式 `-msc` 的内容与用法，并校正进度开关与 `-margs` 托管参数的描述
- 文档体系按新版 project-control-plane 重整：新增 `docs/设计总览.md`、`docs/当前实现总览.md`、`docs/modules/01-命令行参数模块.md`；backlog 补模块演进叙事与状态汇总表
- 控制台改为“模块总览 + 活动 Backlog”，BL-001 保持待验收；修正 02 backlog 指向不存在的 `docs/后续开发事项.md`，并同步 masscan 模块文档中 `-msc -auto`/`-msc -m icmp` 互斥说明
- 新增 masscan 内嵌联动模式 `-msc`（BL-MSC-001）：fscanx 自动查找并启动 masscan，stdout 流式复用 `--std` 扫描管线，stderr 解析进度并在终端底部原位刷新；新增 `-mr/-mp/-margs`，目标支持 `-h/-hf/-hn` 与域名解析，端口支持端口组/区间/`-pn` 相减
- 抽出 `ScanFromReader` 并统一扫描路径直接打印到输出协调层，进度行与日志不再交错；非 TTY 或 `-silent` 自动关闭进度
- 新增单测：文件头识别、目标/端口转换、masscan 状态行解析、托管参数拒绝、进度擦除/重绘顺序、临时文件只读/清理；`--std` 行为保持不变
- masscan 模式失败/中断不再打印“scan done”成功行；参数组合错误改到 stderr；第二次 Ctrl+C 强制清理临时文件并退出；`-hf` 容忍行内注释/空格、URL 大小写不敏感、反向区间提前拒绝
- 文档校验脚本 check-sync.py 支持本项目三种 item 编号（BL-001 / BL-MEM-001 / BL-MSC-001），修复原正则只认 BL-数字-数字导致的“0 item”空跑
- BL-MSC-001、BL-MEM-001、BL-MEM-002 用户验收通过，验收状态置已验收并从控制台移除；BL-001 仍待验收

## 2026-09-23

- 修复 ReadRawWithSize 池污染数据竞争（BL-MEM-001，P0）：所有返回路径拷贝后返回、defer 调整为先 Reset 后 Put、删除 DoWebScan 对共享 client.Timeout 的冗余赋值（兼数据竞争）
- POC 响应体读取限长（BL-MEM-002，P1）：getRespBody 对原始 body 与 gzip 解压后分别限制 4MB（超限静默截断，防全量读入与解压炸弹）；redis readreply 限长 16KB；go.mod 移除 pond 误标的 // indirect
- 新增回归测试：ReadRawWithSize 并发池隔离与错误路径拷贝语义（-race）、getRespBody 限长与 gzip 解压防护、readreply 16KB 上限
- 修复两个阻塞整包测试的既有测试 bug：ms17010_test.go 在 err 检查前解引用 nil conn 导致 panic；qqwry_test.go 在 qqwry.dat 缺失时改为跳过而非 panic

## 2026-06-11

- 新增 `-dp` 命令行选项：启用后对纯域名目标（如 `baidu.com`）自动组合 `-p` 指定的端口生成 `http://域名:端口` 和 `https://域名:端口` 格式的 URL
