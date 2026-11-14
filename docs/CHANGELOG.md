# CHANGELOG

## 2026-11-14

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
