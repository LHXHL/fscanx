# BL-POC-009 产品册：标准产品体系 + 三视角维护端改造

- 模块：POC 扫描
- 优先级：P1
- 状态：已完成
- 当前阶段：待验收
- 价值：修复绑定关系静默失效（中文名标准产品的出厂绑定从未发出、界面绑定可绑到引擎不会输出的名字上且无提示），并把"产品叫什么（别名）+ 测什么（测试名单）"统一为一个产品对象，以三视角（指纹 / POC / 标准产品）维护——绑定操作前置到资产编辑现场。
- 关联模块文档：docs/modules/10-POC扫描模块.md
- 关联计划：plans/2026-11-17-BL-POC-009-产品册.md
- 关联代码：mylib/finger/resources/catalog.yaml、mylib/finger/alias/alias.go、PocScan/binds.go、PocScan/WebScan.go、Plugins/catalog_inject.go、cmd/fscanx-web/api_catalog.go、cmd/fscanx-web/catalog_db.go、web/apps/admin/src/views/Catalog/Catalog.vue、web/apps/admin/src/views/Finger/Finger.vue、web/apps/admin/src/views/Poc/Poc.vue
- 验收状态：待验收

### 方案（立项时必填）

- **做什么**：
  1. **数据层合并**：出厂两文件（识别引擎的别名归一表 `mylib/finger/resources/aliases.yaml`——"各指纹手册的不同写法→标准名"映射；POC 预置名单 `PocScan/binds-seed.yaml`——"产品→测试名单"出厂兜底）合并为一份**出厂产品册** `catalog.yaml`，条目 = 标准名 + 别名（alias）+ 测试名单（pocs）。放识别引擎侧（归一职责本属引擎，出厂别名与指纹库版本配套演进）；POC 侧跨层读取 pocs 段（既有引用方向，无反向依赖）。
  2. **出厂产品册重刷**：产品键改标准名；`weaver` 拆三个子产品（e-cology / 云桥e-bridge / e-office）各挂各的测试（现 4 个 poc 混挂一条）；每条目别名区收录该产品在指纹库的真实写法；重刷对照表给用户人工核对后固化；转换脚本 gen_binds_seed.go 退役（出厂产品册转人工维护）。weblogic/thinkphp 等现有**已生效**英文键保持命中语义（重刷时核实其标准名形态）。
  3. **外置产品册**：SQLite 主存 + yaml 快照双轨（复用 BL-POC-005 的 binds.db 模式——DB 主存、文件快照、哈希比对吸收手改），启动时拆分注入：别名段经 `alias.AppendAliases`（引擎现成 API，现无调用方）注入归一表，pocs 段并入绑定合并。**存量迁移**（审查 H1/M3 修订）：扫描器侧旧 `binds.yaml` **兼容读取**（product 值按原样转为自标准名条目参与匹配，文件本身不落盘替换）；web 端 binds.db 与种子数据**就地升级**为新 schema（product→标准名字段 + 别名区），归一不明的条目原名保留 + 警示标记，**迁移前后对同一目标的定向发包集合保持一致**。
  4. **web 端三视角改造**（用户 2026-11-17 设想拍板）：**指纹视角**（现指纹库页升级：每条指纹显示产品归属——标准产品 /「某标准产品」的别名 / 未归属警示——+ 所属产品绑定测试预览；可指定归属、自立标准产品、顺手绑定）；**POC 视角**（现 POC 管理页升级：编辑器加"绑定了哪些标准产品"清单，可增删）；**标准产品视角**（新页取代现绑定关系页：总览全部标准产品——标准名/别名/测试名单/来源/警示——可新建关系、取消关系、新建标准产品（**必须至少挂一条真实指纹写法做别名，否则拒绝保存**，防死条目）、出厂条目编辑双目标保存（写回内置源 catalog.yaml 下次编译生效 / 落外置覆盖立即生效，同 BL-POC-008 POC 编辑器模式））。三视角操作**同一张关系表**（标准产品→测试名单），绑定候选 = 标准产品列表（挑产品而非输名字）。
  5. **执行层精确索引**：绑定名单路径按 poc 名精确索引取用（建 name→poc map），修掉 `filterPoc` Contains 子串扩张导致的重复执行。
- **为什么**：调研（2026-11-17 只读审计）发现绑定关系静默失效——出厂预置名单键是英文旧名，识别引擎经别名归一后输出的是标准名，两边对不上时绑定不发一包且无提示；**失效范围为中文名标准产品**（weaver/泛微系、tongda/通达、landray/蓝凌、yongyou/用友系等约 5/15 产品），英文键部分仍生效（seeyon 因标准名"用友 Seeyon OA"含该子串可命中，weblogic/thinkphp 等无别名条目、引擎输出原始英文名可命中）。界面候选名源是两指纹明文 name 全集（api_finger.go listProducts），与引擎输出名（归一后）不同命名空间，绑了不生效无提示。合并为产品册=用户拍板（两个文件本就是同一"产品对象"的两组属性，历史出身不同被拆开）；三视角=用户设想（AI 原单页产品册方案被替换）。
- **怎么做**：按上面五层顺序实施。设计铁律（审查后固化）：
  1. **别名完整性**：绑定命中依赖"识别输出=标准名"，依赖别名区收录该产品全部真实写法——两指纹明文全集（goby 7200 + fingers_http 554）中每个 name 必须要么归一到达某标准名、要么落入显式"未归属清单"（逐条人工确认，与指纹视角"未归属"状态同一数据）；
  2. **整条全量写出**：出厂条目编辑落外置覆盖时必须携带完整条目（别名区+测试名单全量），禁止局部字段覆盖（别名区被清空会连锁导致绑定失效）；
  3. **catalog.yaml 顶层保持现有列表结构**（条目数组）——引擎解析目标是 `[]*Alias` 顶层列表，pocs 作为条目内未知字段由宽松解析忽略；立项后第一验证项即实测此容忍性；
  4. **精确索引仅限绑定名单路径**：`-pocname <子串>` 与 `all` 的 Contains 模糊语义**保持不变**（BL-POC-001 既定承诺，flag 帮助文本明文 "use the pocs these contain pocname"）；
  5. **映射改显示名知情**：把某写法挂为别名后，扫描输出行将显示标准名（归一既有行为，外置层让用户亲手触发）——映射操作确认文案必须写明。
- **为什么这么做**：合并 vs 分立——消除"同一产品对象拆两处维护"的重复与漂移；技术上引擎宽松解析容忍 pocs 字段（待实测）、明文嵌入不走重打包链、消费方向无反向依赖、上游同步向后兼容（上游条目无 pocs 字段读入为空）。三视角 vs 单页——绑定放到资产编辑现场（写指纹时顺手绑、写 POC 时顺手绑），总览页只做全局关系管理。出厂产品册放引擎侧——归一职责本属引擎。**架构决策记录（审查 M4）**：本 item 打破 BL-POC-005 确立的"fscanx 扫描器零改动（DB 只存外置层）"原则——扫描器启动路径新增产品册注入点（外置别名注入归一表 + 外置 pocs 并入绑定合并），理由：别名归一引擎侧不注入则外置映射对识别输出不生效、绑定命中链断裂；影响面：扫描器启动加载路径 + mylib/finger 引擎层 API 接线。绑定命中宽松 Contains 保留（兼容手写部分名如绑"泛微"覆盖泛微系，不破坏现存命中绑定）；否决"指纹合并/新别名机制"（引擎别名归一已存在，缺的是键空间对齐与外置补充通道）；否决旧名→新名智能改名建议（二期）。

### 范围

- mylib/finger/resources/catalog.yaml（新：出厂产品册，合并重刷后替代 aliases.yaml + binds-seed.yaml 双文件）
- mylib/finger/alias/（读 catalog、AppendAliases 接线外置注入）
- PocScan/binds.go（消费 catalog pocs 段、加载合并改造、旧 binds.yaml 兼容读取）
- PocScan/WebScan.go（绑定名单精确索引取用；-pocname/all 路径不动）
- cmd/fscanx-web/catalog_db.go（新：外置产品册 SQLite）+ API 端点（catalog_list/_add/_edit/_delete 按命名规范）+ api_routes.go 菜单（绑定关系→标准产品）
- web/apps/admin/src/views/Finger/Finger.vue（归属列+绑定操作）、Poc/Poc.vue（绑定清单区）、Catalog/Catalog.vue（新标准产品视角页）
- 存量迁移（binds.db schema 就地升级 + 种子吸收）

### 不做

- POC 保留原始 path 打子路径（用户 2026-11-17 拍板不做）
- 目录扫描识别产品回流 POC 测试（择期另立 item，需先评估请求放大）
- 旧名→新名**智能改名建议**（二期；本 item 的"存量迁移"指**数据格式迁移**——旧绑定数据转产品册结构，两者不是一回事）
- 批量导入（整文粘贴多条新增）
- 扫描显示名的展示逻辑变更（别名归一是既有行为，本 item 只让用户可补充映射）
- 指纹库内容治理（归 05 模块）

### 验收条件

- [x] 泛微样例目标：出厂产品册条目（拆分后 e-cology 子产品）定向发出对应测试包，**且不发出其它子产品（云桥/e-office）的测试包**（改前：weaver 键零发包；反向约束防"一锅端"原状蒙混过验）——TestWeaverSampleEndToEnd（httptest 端到端，真实出厂绑定 frame=泛微 e-cology，正向命中+反向隔离断言）
- [x] 引擎对合并文件解析零影响：同一目标识别输出与合并前基线一致（含 pocs 未知字段容忍实测——立项后第一验证项）——catalog_tolerance_test.go 回归锁 + verify_catalog_merge.go 归一零回归（116 条映射 0 不符）
- [x] 别名完整性：两指纹明文全集每个 name 要么归一到达某标准名、要么落入显式未归属清单，未归属清单逐条人工确认（人工核对产物落 plan 记录）——7722 写法全量三态实测（alias-of 45 / self 12 / unassigned 7665，清单落 plans/scripts/alias_coverage_report.txt）；46 条候选按"与绑定条目别名包含关系"精筛人工分辨（alias_suspects_precise.txt）：5 条致远系 fingers 写法真漏收候选 + 1 条"心通达OA"歧义**待用户裁决补录**，其余为既定裁决不收（vmware 同名异产品等）或子串巧合误报
- [x] 绑非标准名走"挑标准产品 / 新建（强制挂别名）"交互；新建条目别名区为空被拒绝保存——Catalog.vue 候选辅助（指纹写法全集下拉）+ 前后端双层守卫；守卫拒绝/放行例外均 API 实测（假名拒绝、指纹全集名放行）
- [x] 出厂条目编辑双目标：写回内置源（catalog.yaml 更新、重编译后生效实测）/ 落外置覆盖（立即生效、引擎注入实测）；整条全量写出（别名区不丢，改 pocs 后别名区字节级核对）——API 实测：built-in 写回原别名零丢失+新别名写入+pocs 零丢失；external 覆盖快照落盘+别名区整条携带+origin 翻转+外置别名注入 resolve 实测 alias-of
- [x] 存量迁移：扫描器兼容读取旧 binds.yaml（文件字节不变）+ web binds.db 就地升级；迁移前后对同一目标的定向发包集合一致；归一不明条目带警示标记——TestMigrateLegacyBinds（数据级：普通行原名立条/墓碑行别名保留/.migrated 防重/幂等）+ TestMigratedBindEquivalence（httptest 发包级等价）+ missing_alias 警示标
- [x] 互为前缀的两个测试名只执行一次（构造样例验证精确索引）；`-pocname <子串>` 显式过滤命中数与改前一致（模糊语义不变护栏）——TestBoundPocExactIndex（前缀对只发 1 次）+ TestPocNameSubstringGuard（Contains 语义对照锁定）
- [x] 三视角页面产物版 headless 交互链零 console 错误；go build/vet/test 全绿——产物版 headless 11 项 ALL PASS + console 错误 0（three_views_result.txt）；vet+test 五包全绿（PocScan/Plugins/cmd/fscanx-web/mylib/finger/alias）
- [x] 文档五件套同步 + check-sync 无 HIGH

### 依赖

- 无前置（BL-POC-001~008 全部已验收；005 的 binds.db 存储模式与 006 的指纹 CRUD 模式可复用）

### 状态记录

- 2026-11-17：立项（Mode B：用户批准合并方案与三视角设想后 committed）。独立审查（sonnet agent，12 项代码事实核对全属实+2 隐含断言修正）REQUEST_CHANGES 修订后落盘：H1 动机断言"出厂绑定从未生效"改为中文名产品限定（失效约 5/15，英文键部分生效）+ 迁移验收改可证伪口径（定向发包集合一致）；H2 补 filterPoc 双职责护栏（精确索引仅限绑定名单路径，-pocname/all 模糊语义不变）+ 验收补护栏项；M1 验收补反向约束（不发其它子产品包）；M2 别名完整性口径改"全集归一到达或未归属清单"；M3 迁移语义写明（扫描器兼容读取文件不动 / web DB 就地升级）；M4 补 catalog 顶层列表结构铁律 + 打破"扫描器零改动"原则的架构决策记录；L1 不做清单区分格式迁移与智能改名建议。
- 2026-11-17：**Phase 0-2 执行完成（扫描器侧产品册消费链打通）**。①铁律第一验证项实测通过（yaml.v3 宽松解析容忍 pocs 字段，catalog_tolerance_test.go 回归锁）；②出厂产品册固化——gen_catalog 工具合并重刷（50 条目：weaver 拆 e-cology/云桥e-bridge/e-office 三子产品+用友 U8 OA 独立立条，seeyon/通达/蓝凌 pocs 挂中文标准名=这批出厂绑定从死变活，12 组漏收别名收录、失效别名"泛微-移动办公OA"清理、9 个英文自标准名条目保活），用户裁决用友U8 三写法收进 U8 OA（goby 特征 /indexbusiness.jsp 判定 yyoa），核对表 7754 写法引擎实测（两次口径修正：Compile ToLower 输出形态、注入草稿后核对），旧两文件删除、gen_binds_seed.go 退役、gen_catalog.go 使命完成转档案；③引擎/扫描器接线——embed 指 catalog.yaml+CatalogData 导出，binds.go catalogSeed 按 name+pocs 投影独立解析（alias 包结构零改动），绑定名单路径精确索引（buildPocRequest 提取，-pocname/all 模糊语义未动），外置 binds.yaml 兼容读取天然满足。**新增行为测试两条 PASS**：TestCatalogSeedWeaverSplit（拆分正向+反向约束）、TestBoundPocExactIndex（互为前缀只发 1 次，httptest 端到端命中）。意外发现三条入 plan（check.go worker panic 死锁隐患归 010 顺带修/测试三大环境坑/全量既有失败三处与本轮无关）。go build/vet 全绿；相关包测试全绿。Phase 4-6（web 后端+前端三视角+验收）待续。

- 2026-11-17：**Phase 5-6 执行完成（web 前端三视角 + 全量验收实测），置已完成+待验收**。①**Phase 5 前端**：新建标准产品页 Catalog.vue + api/catalog（列表：标准名/别名标签带源 tooltip/测试名单/来源/missing_alias 警示标；搜索分页客户端化；弹窗 CRUD——新建候选辅助 el-select-v2（指纹写法全集近万项虚拟化坑前科复避）、别名区按四源行编辑、防死条目前端守卫；编辑出厂条目 target 双目标 radio 对齐 BL-POC-008 先例）；Finger.vue 加产品归属列（批量 catalog_resolve 三态：标准产品/「某某」的别名/未归属警示，tooltip 说明归一语义）+ 所属产品测试预览列（owner lower 键匹配——resolve 的 owner 是 alias.Compile ToLower 引擎输出形态）；Poc.vue 绑定弹窗改产品册语义（候选=标准产品列表带缺别名标注、读条目→追加 pocs→整条写回 external、tag closable 解绑）。②**resolve 跨源兜底改造**：原实现直接查 Map[it.Source] 忽略 blocked 语义且无跨源兜底——改 Find（blocked 一致）+ FindAny 跨源兜底（ehole 独有写法在 fingers_http 页查看不应误报未归属），self 优先（标准名自收录形态避免 alias-of 自指，TestCatalogResolve 断言此顺序）。③**验收实测**：新增行为测试四条全 PASS——TestWeaverSampleEndToEnd（httptest 端到端正向+反向隔离）、TestPocNameSubstringGuard（-pocname Contains 护栏）、TestMigratedBindEquivalence（迁移发包等价）、TestBoundPocExactIndex（前轮已有）；API 实测 built-in 写回（别名零丢失/新写入/pocs 零丢失，源文件备份还原）/external 覆盖（快照落盘/别名区整条携带/origin 翻转/外置别名注入 resolve alias-of）/守卫两态（假名拒绝/指纹全集名放行）/出厂回落；产物版 headless 三视角 11 项 ALL PASS 零 console 错误（分页断言/popper 首屏虚拟渲染/绑定按钮 disabled 三处脚本缺陷修正后，three_views_result.txt 在案）。④**意外发现**：测试全局时序坑变体——once.Do(initpoc) 全局耗尽后首调不再加载绑定 + 前序测试 cleanup 恢复 PocBinds=捕获值(nil)，必须显式 once.Do(initpoc)+loadBindsFile()（单跑/合跑/全包三轮实证，注释在案）；Git Bash curl -d 中文编码损坏致 resolve 全 unassigned 假阴性（UTF-8 Python 请求复测正常，检索类工具中文参数一律走文件）。⑤**别名完整性全量实测交裁决**：7722 写法三态实测（goby 7198/fingers 524：alias-of 45、self 12、unassigned 7665——绝大多数为出厂册未收录产品的正常自标准名态），46 条候选按"与绑定条目别名包含关系"精筛人工分辨——**5 条致远系 fingers 写法（用友致远oa/致远OA-A6/A8/致远oa A6/A8）真漏收候选 + "心通达OA"歧义，待用户裁决补录**（fingers 路径下 seeyon 条目 11 个出厂测试对这些写法不发）；vmware 系 13 条同名异产品（Phase 1 既定裁决不收）、nexus/PHP/SAS/ace 等子串巧合误报不收。清单落 plans/scripts/（alias_coverage_report.txt + alias_suspects_precise.txt）。go vet+test 五包全绿。UI 观感留用户浏览器确认。

- 2026-11-17：**指纹视角操作能力补齐（用户验收反馈：方案承诺"可指定归属、自立标准产品、顺手绑定"首轮只交付了只读归属列）**。Finger.vue 加行内"归属"按钮 + 归属管理弹窗：未归属态两路操作——挂为已有标准产品别名（目标产品 el-select-v2 候选=标准产品列表；确认框写明"映射后扫描输出将归一显示为标准名"——铁律 5 落位）/ 自立标准产品（标准名预填该写法可改，别名自动挂该写法满足防死条目；成功后 refreshOwnership 刷条目+归属判定，弹窗切已归属态可顺手绑定——首版只刷条目不刷 verdict 致分支不切，headless 实测抓住修复）；已归属态显示测试名单（tag closable 移除）+ 追加绑定（POC 候选近万 el-select-v2 虚拟化）。全部操作走 catalog_add/catalog_edit external 整条写回，与标准产品页同一张关系表。headless 归属操作链 6 项 ALL PASS 零 console 错误（自立→分支切换→绑定→挂别名确认框→归属列刷新），三视角全套 11 项回归 ALL PASS；测试残留清理后回纯出厂态。脚本坑两枚入案：el-select-v2 渲染根类是 .el-select（选择器必须复合形态，.el-select-v2 单选 count=0）、脚本须自立前幂等清同名残留（前轮中断条目会让自立被"已是标准产品"拒绝，分支不切）。

- 2026-11-17：**别名完整性裁决闭合（用户两项裁决）+ 旧绑定关系页退役**。①致远系 7 条（fingers 5 + goby 2，前轮报 5 条系漏数 goby 源修正）用户确认补录——出厂册「用友 Seeyon OA」别名区 fingers +5（用友致远oa/致远OA-A6/A8/致远oa A6/A8）、goby 源新建 +2（致远OA-A6/A8），resolve 实测 7 条全部 alias-of「用友 seeyon oa」，全量精筛候选 46→39 条；防回归锁 TestCatalogSeeyonZhiyuanCoverage（写法缺失即报"出厂测试对该指纹路径静默失效"）。出厂册为 embed——已部署扫描器需重新编译生效，web 端读 -root 源文件即时可见。②心通达OA（fingers/goby 各 1）用户裁决「和通达OA不是一个产品」→ 不补录维持未归属（识别按原名输出、不触发通达 5 测试），候选清单永久排除并标注。③用户指令删除旧绑定关系页——views/Binds/Binds.vue + api/binds/index.ts（binds 四 API 死代码）删除，listProductsApi 迁 api/finger（语义归属指纹域，后端 handler 本在 api_finger.go），Catalog.vue import 切换；三视角 11 项回归 ALL PASS 零 console 错误。④go test/vet（PocScan/alias/cmd/fscanx-web）全绿。

- 2026-11-17：**写回文本手术重构 + 外挂条目转正通道 + 界面文案白话化（用户三项裁决）**。①用户指出"出厂/外置"等命名难懂并明确其心智模型（web 维护指纹/POC→改代码源文件等 go build；外挂目录运行时生效）——三项裁决落地：**转正通道补上**（catalogEditBuiltIn 对 original 非出厂条目走追加语义：写出厂源+删外挂同名行+origin 翻转，UI 编辑弹窗双位置对外挂条目开放）；**写回改文本级手术**（catalogEntryStartRe 顶层边界切块+逐块 yaml 解析匹配[块顶层是序列,解析目标必须切片取首元素——diag 实测抓出]+命中区间字节替换，注释/排版/未动条目字节零漂移；三重防护：定位失败报错不写/写回前全文解析校验/原子写；API 端到端实测：注释保留+原文整体前缀零漂移+外挂行清理+origin 翻转）；**文案全改白话**（出厂/外置→代码内置/外挂文件，来源列带生效方式 tooltip，保存按钮/确认框/弹窗 hint 同步）。测试三条新增（注释保留零漂移/转正/撞名拒绝源文件字节不动）全 PASS；三视角回归 ALL PASS 零 console；Poc.vue 来源列"内置/外置"formatter 保留（POC 域另有既有文案，仅产品册域改白话）。ZZZCMS 为用户自建外挂条目保留不动。

- 2026-11-17：**对抗式复审轮（用户指令：业务逻辑正确/鲁棒/幂等自查）——6 项发现全修复**。①同名双块防御：源文件被手改成两个同名条目（异常态）时只替换首个会静默残留旧块且全文校验发现不了——replaceCatalogEntryInText 改先全量扫描统计同名数，0=数据文本不一致 / >1=异常态，均报错不写（TestCatalogEditBuiltInRejectsDuplicateBlocks 锁定）。②CRLF 行尾适配：git autocrlf 物化 CRLF 的源文件写回新块为 LF 会产生混合行尾——渲染块按源文件行尾风格适配（TestCatalogEditBuiltInCRLFLineEndings）。③墓碑压制内置写回：出厂条目被删（外挂墓碑）后写回内置源，写回成功但被墓碑压制 UI 不可见——found 路径写回成功后精准清理外挂墓碑行（非墓碑覆盖行=合法语义不动）（TestCatalogEditBuiltInClearsTombstone）。④转正幂等：首次转正后再以 target=built-in 编辑同名条目自动走替换语义（baseline 已含）不产生重复条目——语义天然成立+测试锁定（TestCatalogEditBuiltInPromoteIdempotent）。⑤**转正条目窗口期删除缺口**（复审最大收获）：isBuiltInEntry/builtInEntry/builtInTombstone 走 embed 基线而写回基线是 -root 源文件优先——转正条目重编译前 embed 不含 → UI 删除报"不存在"。修复：server 路径出厂判定统一走 baseline（isBuiltInEntryBase/builtInEntryBase/isTombstoneRowBase/builtInTombstone→tombstoneOf 纯函数），迁移路径 insertLegacyBinds 保留 embed 语义。⑥missing_alias 误标：判定只查 len(alias)==0 未排除"自标准名"合法形态——kingsoft/vmware 等出厂条目被误标警示。修复：排除名字在指纹输出名全集的条目（toEntryView 改 server 方法），API 实测误标清零（TestToEntryViewSelfStdNoWarning）。另修 catalog_list build_hint 旧术语文案。转正 hint 补窗口期警示（"在此之前该条目在扫描器中暂不生效"）。五包测试全绿 + 三视角回归 ALL PASS 零 console。

- 2026-11-17：**新建资产双位置通道补齐（用户质疑：为什么新建只有一个"保存到外挂"按钮）**。历史设计（BL-POC-008）把"写内置源"定位为"修改已存在的内置资产"，新建一律仅外挂——按用户两件事模型（web 维护→改代码源文件等 go build）判定不合理，三处补齐：①产品册 catalog_add 支持 target=built-in（catalogAddBuiltIn：出厂源文本追加+三重防护+.bak 备份，不落外挂层，hint 窗口期警示）；②POC 新建/外置转内置放开——embed 无此名不再硬拒，按 name 解析仓库源文件：不存在=创建（下次编译随版本）/同名一致=编辑未编译仓库文件（getPoc built-in 同语义）/同名异内容=拒绝（防覆盖错文件），既有守卫（-root/内容同一性/原子写）全部保留；③前端双位置 UI 统一——Catalog.vue 新建弹窗"保存位置"radio（hint 区分新建直写/转正/编辑三语境）、Poc.vue 编辑器 radio 放开到全部编辑器态（v-if selSource===built-in&&selFile → formLoaded）+保存区文案白话（代码源文件/外挂 pocs/）。测试：POC 新建内置三场景（创建/编辑未编译仓库文件/同名异内容拒绝字节不动）+产品册 add-built-in（注释保留/原文前缀零漂移/外挂层不落行）全 PASS；BL-POC-008 旧断言"embed 不存在→拒绝"按语义演进更新（注释标明）。五包全绿+三视角回归 ALL PASS 零 console。
