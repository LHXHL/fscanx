# BL-POC-002 合并式 poc 加载 + 绑定关系外置

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
