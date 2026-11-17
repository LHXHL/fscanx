# CHANGELOG

## 2026-11-17

- BL-POC-009 新建资产双位置通道补齐（用户质疑新建只有单一"保存到外挂"按钮）：产品册 catalog_add 支持 target=built-in（出厂源文本追加+三重防护+.bak 备份）；POC 新建/外置转内置放开（embed 无名→按 name 解析仓库源文件：不存在=创建/同名一致=编辑未编译文件/异内容=拒绝防覆盖，既有守卫全保留）；前端新建/编辑统一双位置 radio + POC 保存区文案白话化。测试四条新增全 PASS、BL-POC-008 过时断言按语义演进更新，五包全绿。
- BL-POC-009 写回文本手术重构 + 外挂条目转正 + 文案白话化（用户三项裁决）：①转正通道——外挂条目 target=built-in 走追加语义（出厂源追加+外挂同名行清理+origin 翻转），编辑弹窗双位置对外挂条目开放；②写回改文本级手术——顶层条目边界切块+逐块 yaml 解析匹配（块顶层是序列，解析目标须切片取首元素）+命中区间字节替换，注释/排版/未动条目零漂移，三重防护（定位失败报错不写/全文解析校验/原子写），API 端到端实测注释保留+原文前缀零漂移；修复此前"写回重排全文件格式丢注释"缺陷（曾致出厂册头注释丢失，已手工恢复）；③文案全改白话——出厂/外置→代码内置/外挂文件（来源列带生效方式 tooltip，POC 列表来源列同步），保存按钮/确认框/hint 跟随。测试三条新增全 PASS、三视角回归 ALL PASS 零 console。
- BL-POC-009 别名完整性裁决闭合 + 旧绑定关系页退役（用户裁决/指令）：致远系 7 条写法（fingers 5+goby 2）补录出厂册「用友 Seeyon OA」别名区（fingers+5/goby 源新建+2）——resolve 实测全 alias-of、全量精筛候选 46→39、防回归锁 TestCatalogSeeyonZhiyuanCoverage（出厂册 embed 需重编译对已部署扫描器生效）；心通达OA 裁决非通达产品不补录（维持未归属，候选永久排除）；删除 views/Binds/Binds.vue + api/binds/index.ts（binds API 死代码），listProductsApi 迁 api/finger，三视角 11 项回归 ALL PASS。
- BL-POC-009 指纹视角操作能力补齐（用户验收反馈）：指纹页行内"归属"弹窗落地方案承诺的"指定归属/自立标准产品/顺手绑定"——挂为别名（候选=标准产品列表 + 确认框映射改显示名文案[铁律5]）、自立标准产品（别名自动挂该写法，成功后刷新归属判定切已归属态）、已归属态测试名单 tag closable 移除 + 追加绑定；操作全部走 catalog external 整条写回与标准产品页同表。headless 归属链 6 项 + 三视角回归 11 项 ALL PASS 零 console 错误，测试残留清理回纯出厂态。修复：自立后未刷归属判定致弹窗分支不切（headless 实测抓住）。
- BL-POC-009 Phase 5-6（web 前端三视角 + 全量验收实测，置已完成+待验收）：**标准产品视角**新页 Catalog.vue（条目列表：标准名/别名 tag 带源 tooltip/测试名单/来源/missing_alias 警示标，客户端搜索分页；弹窗 CRUD——新建强制挂别名+候选辅助 el-select-v2（近万指纹写法虚拟化）、别名区按四源行编辑、前后端双层防死条目守卫；编辑出厂条目 target 双目标 radio 对齐 BL-POC-008 先例）；**指纹视角** Finger.vue 加产品归属列（批量 catalog_resolve 三态：标准产品/「某某」的别名/未归属警示 + 所属产品绑定测试预览，owner 按引擎 ToLower 输出形态 lower 键匹配）；**POC 视角** Poc.vue 绑定弹窗改产品册语义（候选=标准产品带缺别名标注、读条目→追加 pocs→整条写回 external、tag closable 解绑）。resolve 跨源兜底改造：Find（blocked 与引擎一致）+ FindAny 跨源（ehole 独有写法不误报）、self 优先防自指。验收实测：新增行为测试四条（TestWeaverSampleEndToEnd 泛微端到端正向+反向隔离 / TestPocNameSubstringGuard -pocname Contains 护栏 / TestMigratedBindEquivalence 迁移发包等价 / TestBoundPocExactIndex 前轮）；API 实测 built-in 写回别名零丢失、external 覆盖整条携带+origin 翻转+外置注入 resolve、守卫两态、出厂回落；产物版 headless 三视角 11 项 ALL PASS 零 console 错误；vet+test 五包全绿。意外发现：once.Do(initpoc) 耗尽+cleanup 恢复 nil 的测试时序坑变体（三轮实证注释在案）、Git Bash curl 中文编码损坏假阴性（中文参数一律走文件）。别名完整性全量实测（7722 写法三态）筛出 5 条致远系 fingers 写法真漏收候选+"心通达OA"歧义**待用户裁决**（清单 plans/scripts/）。文档五件套同步 + CHANGELOG 重复条目清理。
- BL-POC-009 Phase 4（web 后端产品册）：catalog.db（SQLite entries 表）+ 二进制旁 catalog.yaml 快照双轨（哈希吸收手改，同 binds 模式）；API 五端点 /api/catalog_list|_add|_edit|_delete|_resolve——合并视图（出厂基线读 -root 源文件优先防陈旧基底 + 外置覆盖 + 墓碑过滤 + missing_alias 警示）、防死条目守卫（空别名且不在指纹输出名全集拒绝）、built-in 写回 -root 出厂源文件（存在性/出厂条目/改名冲突三守卫 + 原子写）、批量指纹归属判定（出厂⊕外置归一表三态：alias-of/self/unassigned）；存量迁移 migrateLegacyBinds（binds.db 普通行原名立条、墓碑行出厂别名保留、binds.yaml 吸收、.migrated 改名防重）；main.go 路由切换 binds_*→catalog_*、菜单"绑定关系"→"标准产品"。新增 7 条测试全过（CRUD/墓碑三件套/吸收/迁移幂等/防死条目守卫/合并视图/归属判定）。

- BL-POC-009 对抗式复审轮（用户指令"假设审计有遗漏，换角度排查"）：①出厂 catalog.yaml 归一零回归机械验证补位（此前只验解析容忍）——初版 3 条不匹配经分辨为 2 条脚本口径缺陷（同键冲突按文件序后者覆盖，新旧行为一致）+1 条预期删除，修正口径后 116 条映射 0 不符（verify_catalog_merge.go 留档）；②fingerprinthub/ehole 压缩源首次逐写法核对（ehole 字段为 cms 非 name，首轮查询假阴性）——按归一键比对筛出**真漏收 7 项**并补录固化：ecology-oa（/wui/ 特征实证）、ecology、泛微 OA、**致远OA（=Seeyon 异名，seeyon 11 个出厂测试对 ehole 路径全失效）**、Tongda、通达OA、齐治堡垒机；③loadBindsFile 重复打印删除；gen_catalog 改造为以固化 catalog.yaml 为基底的增量补录工具（迁移首跑自动优先旧表，幂等）。

- BL-POC-009 Phase 3b（扫描器外置产品册读取+引擎注入）：新增 CatalogEntry/ParseCatalogEntries 投影类型与外置缓存；Plugins.InitEngineAliases 在 main 早期读二进制旁 catalog.yaml（与出厂同构）注入识别引擎归一表（指纹识别前硬时序）并转交 loadBindsFile 合并 pocs 段（同 name 整条覆盖/异名追加）；外置 catalog.yaml 存在时不再读旧 binds.yaml。测试：外置覆盖/追加/出厂兜底三分支、注入归一实测、坏 yaml 显式报错。

- **CheckMultiPoc panic 死锁修复（用户指令"越早越好"，BL-POC-009 执行轮意外发现提前处置）**：recover 与 wg.Done 从 goroutine 顶层/循环体下沉到任务级 `runPocTask` defer——原实现任一 poc 触发引擎 panic 即 worker 死亡 + Done 跳过 → wg.Wait 永久挂死（该目标 poc 阶段卡死）；顺带补 workers≤0 守卫（-num 0 或测试进程零值时 tasks<- 零消费者永久阻塞）。新增防回归测试两条 PASS（runPocTask(nil,nil) 必 panic 场景 wg.Wait 正常返回；workers=0 快速返回）。


- BL-POC-009 Phase 0-2（扫描器侧产品册消费链）：出厂两文件合并固化 catalog.yaml（50 条目，weaver 拆三子产品+用友 U8 OA 立条、12 组漏收别名、失效别名清理，用户核对裁决），旧 aliases.yaml/binds-seed.yaml 删除、gen_binds_seed.go 退役；引擎 embed 改指 catalog.yaml（CatalogData 导出），binds.go 改产品册 name+pocs 投影解析（alias 包结构零改动），绑定名单路径按名精确索引（-pocname/all 模糊语义不变）。新增行为测试：weaver 拆分正反向断言、互为前缀 poc 精确索引只发 1 次（httptest 端到端）。意外发现 CheckMultiPoc worker panic 死锁隐患（recover 在 goroutine 顶层致 wg 永久挂）记入 plan，归 010 顺带修。


- 立项 BL-POC-009（产品册：标准产品体系 + 三视角维护端改造）与 BL-POC-010（POC 执行并发合并与流量口径）。起因：用户五问调研（同一 POC 绑多指纹重复打击 / 多指纹调同一 POC 去重 / URL 带 path 是否拼接 / POC 并发控制 / 目录扫描识别产品是否回流 POC），只读审计证实——绑定键名空间脱节（出厂预置名单英文旧名 weaver 等 vs 引擎归一输出标准名"泛微 e-cology"等，中文名产品出厂绑定静默失效约 5/15；web 端候选名源=指纹明文名而非引擎输出名，绑了不生效无提示）、智能定向逐名串行执行（每名一个短命 30-worker 池实际并发 1）、全量模式峰值 tn×num=1800 并发无提示、POC 入口剥 path 打根路径（用户拍板不做）、目录扫描产品不回流 POC（择期另立）。009 方案（用户批准）：出厂 aliases.yaml + binds-seed.yaml 合并为产品册 catalog.yaml（标准名+别名+pocs，放引擎侧，宽松解析容忍 pocs 字段待实测）、出厂重刷（weaver 拆三子产品、别名区完整性核对）、外置产品册 SQLite+yaml 双轨（AppendAliases 注入归一表）、web 端三视角（指纹视角显示产品归属/POC 视角绑定清单/标准产品视角总览+新建标准产品强制挂别名）、执行层精确索引（-pocname/all 模糊语义不变）。010 方案（用户拍板）：命中名单一轮并发、智能定向温和并发 8、启动打印最坏瞬时并发口径、不上全局闸门。独立审查（sonnet agent，12 项代码事实核对+2 隐含断言修正）REQUEST_CHANGES 全处置：失效范围限定（英文键 seeyon/weblogic 等部分生效非全失效）、filterPoc 双职责护栏、验收反向约束、迁移可证伪口径、catalog 顶层列表结构铁律、打破"扫描器零改动"原则的架构决策记录、010 基线定义与 009 原语复用声明。
- backlog 10-POC扫描 单文件拆分为目录形态（437 行+2 新 item 必超 500 行阈值）：001~008 已验收 item 迁至 docs/backlog/10-POC扫描/ per-item 文件，索引文件降级为演进叙事+状态汇总+item 表；控制台同步（10 item：8 已完成/2 未开始）、模块文档过时注记修正（"BL-POC-001 进行中，002~004 未开始"→当前真实状态）。

## 2026-11-16

- BL-POC-008（内置 POC 写回编辑）与 BL-POC-006（指纹结构化 CRUD）用户真机实测验收闭合，控制台移除；POC 扫描模块 8 item 全部已验收。

- 性能修复（用户报告：POC 页点"绑定产品"卡顿 7-8 秒）：排查证实服务端三 API 合计 <150ms（binds_list 2-12ms/products 50-73ms/pocs 47-52ms），瓶颈在弹窗挂载——`el-select multiple` 把产品名全集（goby 7200+fingers 554 去重后近万项）全量渲染成 el-option DOM，headless 实测点击→弹窗 15.58s。换 Element Plus 虚拟化选择器 `el-select-v2`（options 数组只渲染可视区），实测点击→弹窗 **0.15s**；过滤/多选/确认写盘功能链复测通过，POC 页 11 项回归零破坏。

- 指纹库 SQLite 化 + 结构化条目 CRUD（BL-POC-006，用户提出"指纹库直接加载原始 json 修改体验极差"）：指纹页从 textarea 整文升级为**条目列表（file 切换/搜索/分页）+ 单条结构化表单 + 新增/删除/复制**；存储层 finger.db（SQLite 主存，(file,id) 复合主键——fingers_http 30 组重名 name 不构成身份）+ 明文 JSON 快照双轨，**自研 JSON 数组扫描器逐字节切分**（前缀/逐间隙分隔符/元素原文字节——json.Decoder InputOffset 前视吞间隙不可用），重生成保证未动条目字节零漂移，真实库 7200/554 条种子后 md5 与基线一致；哈希吸收手改（文件被手改→以文件为准全量吸收）；schema 判定降级原文模式（default_port/favicon.path/异形嵌套等长尾形态字段零丢失）；API /api/finger_list|_add|_edit|_copy|_delete，旧整文 GET/PUT /api/finger 退役；生效模型不变（明文→pack→重编译，pack 管线零改动）。Reviewer（sonnet 10 findings 1C/1H/2M/6L）全处置——C 间隙越界 panic（add/copy 路径）实测复现后修复+回归锁；H 编辑缺吸收对齐修复。验证：单测全绿、真实库全链实测（round-trip 四类条目/增删复制/手改吸收复活/pack 三方 md5 全等/坏 JSON 显式报错）、UI headless 14/14 零 console 错误、终态真实文件恢复基线。置已完成+待验收（UI 观感待用户浏览器确认）。
- 内置 POC 写回编辑（BL-POC-008，用户提出"想修改内置 poc、下次 go build 自然带上"）：POC 编辑器保存目标可选**内置源/外置**——内置来源行默认写回 `-root` 下 `PocScan/pocs/<embed 真实文件名>`（下次编译生效，编译时机用户自行掌握），外置覆盖路径保留立即生效；目标文件名由 embed 库按（与 yaml 校验一致的）name 服务端解析，三守卫（存在性/内容同一性/改名）防 -root 指错检出改错文件；同名内置/外置并存按来源定位互不串写，getPoc(source=built-in) 编辑视图读仓库最新源防陈旧基底二次覆盖；groups 形态（31/386 无顶层 rules）放行保存且 groups 内 rule 过 CEL 编译（修复原"rules 为空"校验使 31 行任何目标均不可保存）；错误面统一 {code:1,message} 信封、temp+rename 原子写；**顺带修复存量 bug**——来源列 el-table formatter 签名错位（第三参实为 cellValue）致来源列恒显"内置"。新增 api_poc_target_test.go（source 三语义/身份回落/四守卫/groups 放行/原子写）。验证：单测全绿、沙箱 API 实测（写回/三类拒绝/双行双向不串写/坏 yaml+CEL 拦截源文件不破坏）、重编译 embed 携带新内容 grep 证实后源树字节级恢复、UI headless 产物版 11/11 零 console 错误。Reviewer（sonnet 10 findings 1H/3M/6L）全处置。置已完成+待验收（UI 观感待用户浏览器确认）。
- BL-POC-005（绑定关系页交互重构 + binds SQLite 化 + POC 页快捷绑定）与 BL-POC-007（登录认证 admin/admin123）用户浏览器实测验收闭合，控制台移除；POC 扫描模块 6 已验收 / 1 未开始（BL-POC-006 指纹 SQLite 化草案待定优先级）。
- 新增 docs/fscanx-web-部署运行手册.md：构建（前端先行铁律——go:embed 产物顺序）、部署目录布局（两二进制同目录联动）、登录（admin/admin123 与 token 重启失效语义）、binds.db/binds.yaml 双轨数据模型（手改吸收/墓碑注意事项）、典型使用流程、开发模式、安全边界与故障排查表；设计总览文档导航收录。
- fscanx-web 登录认证（BL-POC-007，用户指令"现在需要登录，搞一个默认账号admin/admin123"）：免登录临时形态退役（main.ts 不再注入本地会话）——后端 /api/user/login 校验默认账号 admin/admin123 签发 crypto/rand 内存会话 token（进程重启失效），/api/user/logout 销毁 token；authMiddleware 对 /api/*（除登录）校验 Authorization 头（接受裸 token 与 Bearer 前缀），无效返回 200+{code:401} 信封——复用前端拦截器既有自动登出链路（permission.ts 守卫零改动）。附带收益：认证门封死跨站无预检盲写面。验证：单测 7/7、headless 产物版登录全链 9/9（未登录重定向/错误提示/登录/加载/刷新保持/登出/伪造 token 自动登出）、curl 重启失效与授权写/无 token 写 401。方法学教训：go build 必须在 web/build.sh 之后（embed 真源同步先行），否则二进制嵌旧前端假失败。登录页观感待用户浏览器确认。
- 绑定关系页交互重构 + binds 存储层 SQLite 化 + POC 页快捷绑定（BL-POC-005，用户两轮 UX 反馈驱动 + 范围扩展拍板）：**绑定页浏览态/编辑态分离**（原表格全行内联编辑、点击产品名即弹候选下拉莫名其妙——改只读展示+每行编辑/删除按钮，下拉只在弹窗表单内出现；添加绑定从表尾插空行改弹窗表单；客户端分页 10/20/50；产品候选排序精确>前缀>子串+命名空间语义提示文案；同名产品拦截与引擎 MergeBinds 去重语义对齐）。**存储层 SQLite 化**：binds.db（modernc.org/sqlite v1.29.10 纯 Go，Go1.20 兼容）主存外置层 + binds.yaml 快照双轨——扫描器零改动（启动时照旧读 种子⊕binds.yaml）；同步机制=单向权威+手改吸收（每请求前比对文件 sha256 与 meta，不一致→全量吸收→重生成；文件缺失按 DB 重生成防误删；坏 yaml 显式报错）；API 改造 /api/binds_list、/api/binds_add（upsert-append）、/api/binds_edit、/api/binds_delete（按页面接口命名规范），PUT /api/binds 退役；**墓碑机制**修复 BL-POC-004 时代隐藏缺口——种子起源行（仅存在于 embed 种子）经合并视图编辑/删除会静默失效（界面成功引擎侧无效），现落 {product, pocs:[]} 墓碑条目使种子绑定真实失效，合并视图过滤墓碑。**POC 页快捷绑定**：编辑器头部"绑定产品"弹窗（已绑产品 tag 只读展示+多选新增），免跳绑定关系页。错误消息改 200+{code:1,message} 信封（原非 2xx 纯文本被 axios 丢弃，指导性错误到不了用户）。验证：单测 19/19（含墓碑三件套/幂等删除/handler 回归）、UI headless 产物版 27/27 零 console 错误、fscanx 实测闭环（绑定 D-link-DSL-2640B→poc 后扫描器对命中目标仅发该 poc 请求）。Reviewer 轮（sonnet 档 10 findings 1H/2M/7L）逐条修复复验全过。UI 观感待用户浏览器确认。

## 2026-11-15

- 修复 exe 旁无 pocs/ 目录时的误报与黏行（BL-POC-002 回归）：initpoc 补约定目录存在性守卫（不存在=正常态静默跳过，原 filepath.Walk 首层 CreateFile 错误被当异常打印）；两处 poc 加载错误输出补换行（原与后续成功行黏在一行）；显式 -pocpath 路径错误保持可见并独立成行。
- always 迁移至 POC 自身属性 + 表单多步结构化（BL-POC-004 迭代轮，用户两连 UX 反馈驱动）：**always 从 binds.yaml 条目字段迁至 poc yaml 顶级标记**（Bind.Always 退役，lib.Poc 加 always；用户指出 always 是 poc 属性而非绑定关系属性，概念错位）。PocScan 空 -pocname 智能定向 = 指纹命中 ∪ always 标记去重执行（alwaysPocNames 遍历 AllPocs）；旧 binds.yaml 残留 always 键非严格解析忽略不炸；web 端开关移入 POC 编辑器。**表单模式覆盖多步 rules 与嵌套条件**（一期"超出表单子集整卡切源码"裁剪被否决）：getPoc 多步结构化（每步 method/path/headers/body/search/follow_redirects/continue 全字段，覆盖 follow_redirects 63%/body 38%/headers 32%/search 7% 使用率），前端 CondTree 递归组件任意深度 AND/OR 嵌套（后端谓词递归原本就绪），超出谓词子集的步骤降级步骤内 CEL 文本编辑（仍无需写 yaml）；表单保存保真——表单管辖字段渲染 + 未暴露顶级段（detail/groups/manual）从打开时原文拼接零丢失；groups（31/386 长尾）form=false 自动源码模式。顺带修真 bug：落盘 CEL 编译校验裸环境对含 set 变量 expression（内置 166/386）必 400，补 envWithSetVars 与求值端同源声明。验证：渲染单测 round-trip+段保真、API 端到端 17/17、无指纹目标空 -pocname 实测 always poc 必发必中且其余 poc 零请求、UI headless 多步 poc 表单承载零错误。

- fscanx-web 前端重做后两轮修复迭代（BL-POC-004）：**登录页问题轮**（用户问登录密码暴露——本维护端免登录无密码，看到登录页=bug）三根因串行修复：Root redirect 指向框架残留 /dashboard/analysis 不存在页改指 /poc/list；框架模板仅注册 ElLoading 致 el-table 等组件全被解析为原生元素渲染空白，改 app.use(ElementPlus) 全量注册；**go:embed 默认排除下划线开头文件**，vite 产物 _plugin-vue_export-helper-*.js 漏嵌入致生产版白屏（dev 模式盲区，终验必须跑产物构建版），改 //go:embed all:static。**UX 反馈轮**（用户真机使用反馈两条）：POC 列表点击行右侧无内容——根因 @row-click 回调被批量替换误伤成 scope.row.name（el-table 事件回调无 scope，点击抛 ReferenceError 静默失败），改回 row.name；绑定关系页 POC 列从手写 textarea（一行一个 name）改为点选下拉（multiple+filterable+collapse-tags，选项=POC 全集）。产物版 headless 终验：点击行编辑器正确展示（源码 yaml+条件表）、绑定下拉含全集、零错误零 404。

- fscanx-web 前端重做（BL-POC-004 追加轮，用户反馈原生单页体验不合格）：换用 vue-element-plus-admin 3.1.0（web/ monorepo 入仓，pnpm workspace，裁掉 docs）。Go 侧三点配套：响应统一包装 {code:0,data}（@vea/request 拦截器契约，writeJSON 单点）、新增 GET /api/routes 后端动态菜单（未来加页面=后端加一条配置）、免登录。web/build.sh 一条龙构建（dist-pro → static embed），dev 模式 vite :4000 代理。三页面迁 Element Plus。回归验收①③ API 链路实证通过（vue-demo 表单端到端命中、非法拦截），②④⑤ API 语义未变；UI 观感待用户浏览器确认。

- BL-POC-004 独立 fscanx-web 维护端执行完成并验收闭合（延续委托代验）：新增 cmd/fscanx-web（标准库 net/http + go:embed 原生前端，零新依赖）——poc 表单化编辑（谓词树下拉组合生成 CEL，子集内反解析回填表单、嵌套/多步自动切源码模式）、落盘双重校验（yaml 解析 + 每条 expression 求值环境编译，4 例非法实测全拦截）、绑定编辑（合并视图 + 产品名下拉 + round-trip 校验）、指纹 textarea 整文编辑（内容即所写=零漂移，实测回写三方 md5 一致）与一键重打包。pack 共享包提取（mylib/finger/packlib，cmd/pack 改薄壳，重构后产物与提取前逐字节一致，web 重打包 md5 与命令行一致）。谓词层实证修正：bmatches 注册签名是 string receiver，生态统一参数反转形态 `"re".bmatches(response.body)`。执行中发现并修复 Bind 缺 json tag 致 API 字段名与前端不匹配的真 bug。五条验收全过（表单新建端到端命中/绑定覆盖改变定向路径/非法拦截/pack 字节等价/指纹零漂移），POC 扫描模块 4 item 全部闭合。二期候选：嵌套树 UI、指纹单条目编辑。

- BL-POC-003 指纹定向触发管道执行完成并验收闭合（用户委托 vulhub 靶场代验）：空 -pocname 从"全量过渡"切换为终态三态——all=全量、显式名=过滤、空=指纹智能定向（HostInfo.Frames 三路填充：DoWebScan 文本指纹/favicon 链尾命中/webpoc 分支 detectProductsSilent 静默首页探测，查 PocBinds 定向执行，无命中不打且不提示）。binds 两层加载：binds-seed.yaml 内置种子 go:embed（PocDatas 14 条产品经 gen 脚本转出 15 产品 53 绑定快照）打底 + 外置 binds.yaml 同 product 覆盖（mergeBinds 纯函数）。HostInfo.PocName 死字段/CheckInfoPoc/PocDatas/LoadMultiPoc/SelectPoc 六处退役清理。意外发现两条入状态记录：DetectContent 输入契约是完整 HTTP raw 报文（首版传纯 body 指纹恒空，ReadRawWithSize 同源拼装修复）；thinkphp 靶场文本指纹弱、favicon 链是定向关键输入（静默函数补 favicon 探测）。实测：定向 612→8 条（98.7% 降幅）只发 thinkphp 系且 RCE 命中、无指纹目标零 poc 零提示、always 条目生效、all/显式名回归正常；全量测试过。README/模块文档同步终态语义。

- BL-POC-001/002 验收闭合（用户无真机环境，委托 vulhub docker 靶场 thinkphp/5-rce 代验）：001 五条全过且端到端命中真漏洞——`-h -p 8080 -m all -poc` 输出指纹 `[*thinkphp]` + `[+] PocScan poc-yaml-thinkphp5-controller-rce`，空 pocname/IP 输入路径/大小写变体/all 均命中；002 四条全过且有容器 access log 第三方证据——约定目录外置 poc 零参数执行（`[+] load 1 pocs` + marker 请求实证）、同名覆盖可证伪闭环（status==999 覆盖版在发包且无命中行，去 -pocpath 内置版即恢复）、all 全量含外置命中计数 2。两 item 置已验收，从控制台移除。

- BL-POC-002 合并式 poc 加载执行完成：initpoc 重构三层合并——内置 embed 386 个兜底 → 二进制所在目录 pocs/ 自动加载（os.Executable 推导，存在才加载打印一行；选二进制目录而非工作目录：内网工具目录习惯，cd 不影响）→ -pocpath 从替换语义改追加（内置不再失效）。同名 poc 后加载覆盖先加载且替换保位（slice+index map），filterPoc 遍历顺序稳定可测。新增 PocScan/binds.go：Bind{product/always/pocs} 绑定关系 schema 与 LoadBinds 加载器（binds.yaml 放二进制旁，initpoc 接线进全局 PocBinds 供 003 消费，文件不存在=正常态）。实测四场景全过：约定目录新 poc 零参数执行（命中 marker+load 提示）、同名覆盖（overridden-marker 执行=替换保位生效）、无 pocs 目录纯内置 all=1168 与 001 基线一致、-pocpath 追加 poc 执行；TestLoadBinds 4/4；全量回归 PocScan/Plugins/common 全 ok。README 参数表同步（all 语义、pocs/ 约定目录、-pocpath 追加）。待用户真机验收。

- BL-POC-001 触发层修复执行完成：`-poc` 恢复真实发包，四处最小修复——WebScan.go 空 -pocname 删 HostInfo.PocName 死循环（全项目零填充恒空转零请求）改直接全量执行、-pocname all 归一化（防被当子串过滤）、filterPoc 双侧 ToLower 大小写不敏感；webScanTask.go 提取 ensureURL（Url 空才构造）供 WebScanWorker 与 webpoc 分支共用，消除 webpoc 非 URL 输入 buf[:3] 越界 panic 被 recover 吞掉的零请求；Parse.go ParseScantype 补 case "webpoc" 用 Webport（意外发现：webpoc 此前落 default 拿插件调度键 1000003 当扫描端口，永远非法，-m webpoc 对 ip 输入从未工作过）。本地五路径实测（python http.server 计数）：空 -pocname 1168 请求、-h ip -m webpoc 1189、-pocname 74CMS 大写 6、all 1168、-m all -p -poc 1192；build+vet+PocScan/lib/Plugins/common 测试全过。待用户真机验收。

- 新立 POC 扫描模块（BL-POC-001~004）：用户双反馈"-poc 实际没有发 poc 请求（PocName 为空）/ 设置 -poc 不指定 -pocname 不发送 poc"触发只读审计，证实触发链两处断裂——空 -pocname 走全项目零填充的 HostInfo.PocName 死字段空循环零请求；-m webpoc 非 URL 输入路径空 Url 在 WebScan.go buf[:3] 越界 panic 被 recover 吞掉同为零请求。终态设计（Mode B0 五轮共识）：触发三态（空 -pocname=指纹智能定向、all=全量、名字=定向过滤；放弃 nuclei 兼容保持小而精）+ 合并式加载（内置 386 兜底 + ./pocs/ 外置同名覆盖 + -pocpath 追加）+ 独立 fscanx-web 维护端（poc 谓词树表单 + 指纹明文编辑与 pack 共享包重打包）。独立审查 16 项事实核对 13 项全一致，3 必改项（binds 无命中兜底语义、webpoc panic 并入 001、pack 需提取共享包）已修订入 item。

- README 与当前实现总览同步近几天功能：功能表补 favicon 图标抓取识别与 focus `[*产品名]` 红色高亮、目录字典 70→76 条；rdp 案例修正截屏描述（黑块已修，NLA 目标免登录截屏属设计限制不适用）；维护指南补 icon_hash.html 哈希工具与 focus 打标方法；更新历史加 2026.11.15 版本级条目；实现总览补 faviconScan.go 行与 log.go focus 红色、核对日期 11-13→11-15。

- focus 产品终端红色高亮（BL-FOC-006）：`[+] Product` 行指纹段内 `[*产品名]` token 整体红色打印（FgRed，与 `[+]` 漏洞行同语义），其余 token 白/青交替不变。common/log.go 新增 `printFpsTail`：`Contains(fps, "[*")` 门控——无 focus 行走原 Println 路径逐字节不变（零性能影响，着色全在 SaveLog 打印 goroutine）；命中按 `, ` 分 token 逐段 Print（全走 color.Output，Windows conhost 翻译路径不变）。落盘文件/JSON 写原始串不经渲染，保持纯文本；`-nocolor` 不受影响。单测 4 用例（无 focus 逐字节回归/focus 红码包裹/段首段尾含版本/空段 3 段 DC banner 行不 panic），build+vet+common/Plugins/finger 全量测试通过；用户真机验收通过，闭合。

- BL-FP-005、BL-FOC-005 用户真机验收闭合（华府站输出 `[*若依-ruoyi], [*若依管理系统]` 双引擎带星号 + 未命中零干扰），从控制台移除；本日累计闭合 8 item，26 item 中 21 已验收（另：FOC-006 同日闭合，累计 9 item，27 item 中 22 已验收）

- BL-FP-005、BL-FOC-005 用户真机验收闭合（华府站输出 `[*若依-ruoyi], [*若依管理系统]` 双引擎带星号 + 未命中零干扰），从控制台移除；本日累计闭合 8 item，26 item 中 21 已验收

- favicon 哈希命中路径的 focus 传递（BL-FOC-005）：fingers 库 focus 指纹经 favicon 哈希命中后输出 `[*产品名]`——补齐"双引擎均支持 focus"在 favicon 子路径的缺口（fingers 文本路径 fingers.go 三处与 goby 路径本就正确，唯 favicon 哈希在编译为 hash→name map 时丢失 focus）。3 处最小改动：FaviconsEngine 加 FocusNames 并行表（按指纹名记，md5/mmh3 通用）、fingers 编译循环填充、引擎聚合拷贝；HashMatch 命中后置 IsFocus。验证三层：favicon 包单测 / 生产引擎取证 MatchFavicon(华府图标)=`[*若依管理系统]` / 真机扫 www.huafurenjia.cn 输出 `[*若依-ruoyi], [*若依管理系统]` 双引擎带星号。插曲：复扫 demo.ruoyi.vip 一度误判回归，实为用户已将库哈希改为华府站点图标（基线漂移非代码缺陷）。附带：icon_hash.html 增加指纹片段生成（4 空格缩进与库文件逐字符一致，name/focus/tag 可填，一键复制）。

- icon_hash.html 哈希工具双 bug 修复 + 若依指纹假哈希修正（BL-FP-005 真机验收反馈根因）：用户实测 www.huafurenjia.cn 不报"若依管理系统"，取证发现工具两处错误——①blueimp-md5 2.19.0 对 Uint8Array 输入走 String() 序列化路径，算出的是"字节十进制逗号串"的 md5（node 三组对照实证，f628... 即此产物）；②mmh3 函数实为 FNV-1a（offset 0x811c9dc5 + prime 0x01000193）冒名，且引擎真语义是 FOFA icon_hash（base64 每 76 字符换行 + 末尾换行后再 murmur3 x86_32，chainreactors encode.Mmh3Hash32 实证）。修复：md5 换 SparkMD5.ArrayBuffer、mmh3 换真 MurmurHash3 x86_32 + standBase64 输入，node 对双图标验证与引擎逐字节一致（含负数 mmh3）。库内若依条目假哈希（grep 确认污染仅 1 条）替换为若依官方 demo 实测真值（md5 e49fd30e.../mmh3 -1231872293），pack 重打包 validate 520 条全过，端到端扫 demo.ruoyi.vip 命中 `[若依管理系统]`（链尾行合并格式正确）。

- favicon 图标抓取与哈希指纹识别（BL-FP-005）：补齐 BL-FP-003 hash-only 指纹库的生产链路——此前 FaviconEngine 哈希表已加载但扫描器从不抓图标、519 条哈希指纹全部闲置。新增 Plugins/faviconScan.go（三层候选：首页 data URI 内联→link 声明路径→/favicon.ico 兜底，预算 2 请求 64KB 上限拿到即停，含 base href 相对解析/非图标 rel 排除/base64 与百分号解码）；DoWebScan 产出行从内部即时输出改为三段拆分由调用方调度（中间行即时、链尾行 favicon 命中分词去重合并后输出，关闭/失败/未命中时逐字节零差异）；新增 `-nofavicon` 参数（默认开启）；清理 scanType 2 死代码。reviewer 独立验证 REQUEST_CHANGES 3 项已修（行尾悬挂逗号/非图标 data URI 占名额/测试补形态）+ deslop + 本地冒烟（python http.server 实证 GET /custom.ico 命中即停）+ 7 测试函数全绿 + Plugins 全量 ok。待用户真机验收。

- BL-RDP-002 用户真机验收闭合（黑块消失），从控制台移除；附带裁定：-screen 空凭据+SSL 设计对 NLA 目标必然失败（认证先于图形阶段，取证 access denied 零 bitmap），属设计限制非回归，用户决策不改造；本日累计闭合 6 item，24 item 中 19 已验收

- RDP 截屏黑块修复（BL-RDP-002）：根因一为能力协商宣告 DSTBLT/PATBLT/SCRBLT 绘制指令但 orders 事件全仓无消费方（解析即丢弃），服务器用指令绘制的区域永无像素——取消宣告退化为纯位图更新；根因二为 Bpp 色深映射错误（15bpp 误映射 1 字节、8bpp 缺失、default 零值无兜底）——修正为 FreeRDP 规范映射 + ToRGBA 按原始位深分支 + 未知位深跳过。Win11 实拍验证无黑块（视觉模型确认）；新增 Bpp/ToRGBA 单测。

- BL-RDP-001 用户真机验收闭合（fscanx 主程序对 Win11/NLA 目标 RDP 爆破命中），从控制台移除；本日累计闭合 5 item，23 item 中 18 已验收

- RDP NLA 认证回归修复（BL-RDP-001，BL-BLD-001 发现移交）：ComputeResponseV2 的 tempBuff 在 AV_PAIR 列表（已含 EOL）后多写 4 零，违反 MS-NLMP 2.2.2.2，NTProofStr/SessionBaseKey 全错——修复前主程序 RDP 爆破对 NLA 目标全废（正确口令也必失败）。三重裁定闭环：规范比对→单测向量恢复真跑 PASS→Win11 真机双对照（正确口令 flag=true / 错误口令 false）。附带：DERT 断言期望按真机实证更新（Version 字段无害演进）、rle CVAL 补长度守卫（C 直译 panic 缺陷）、TestSum 恢复真跑。探针沉淀 plans/scripts/go/rdpprobe、rdpcrack。

- 重构遗留编译与测试债务修复（BL-BLD-001）：5 个构建失败包全修复（cmd 三工具补 fingers import、examples 改 NewWappalyzeEngine、rdpgfx 补回 ChannelSender 字段）+ 4 个测试文件编译修复（engine_test int→string×2、cssp_test 字节字面量、ntlm_test API 更名）+ 14 个测试数据判据 skip 化（数据裁剪自愈判据）。go build ./... 零错误、finger 域 6 包测试全 ok。重要发现：RDP NLA 认证回归嫌疑（ComputeResponseV2 blob 多 4 字节 0，LM 正确 NT 不符，待真机裁定）、DERT 编码漂移、rle 越界——三处真问题测试 skip 注明待专项。

- BL-FOC-001~004 用户真机验收闭合（重点标记 `[*产品名]` 输出正常、若依指纹命中），从控制台移除；本日累计闭合 4 item，21 item 中 17 已验收

## 2026-11-14

- 护网脆弱产品 focus 打标（BL-FOC-004，清单经用户确认）：goby.json 451 条命中插入 focus:true（9 类：Web 危险组件 30/国产 OA 12/ERP 27/VPN 与安全设备 169/视频监控 162/堡垒机运维 15/邮件 28/报表低代码 6/远控 2）+ 用户新增若依指纹（若依-RuoYi，三组 body 特征 OR，focus），条目 7075→7076、focus 452；明细落 plans/2026-11-14-BL-FOC-004-护网打标.md，手术脚本 plans/scripts/tag_hw_focus.py（深度相等验证前置）。执行中发现 git autocrlf+stash 往返曾把 goby.json 物化为 CRLF，已恢复 LF（与 gz 内容字节级一致后手术）。

- goby.json 中文形态化（BL-FOC-003，用户追加需求）：11971 处 uXXXX 转义解码为 UTF-8 原文（raw_decode 逐 token 手术，非字符串区域逐字节保留），转换前后 json.load 深度相等、纯 LF/无 BOM/无尾换行；引擎加载 Len==7075 不变、中文特征抽样匹配命中；重打包 goby.json.gz（2053147 明文 → 310928 gz）。转换工具沉淀 plans/scripts/convert_goby_unicode.py。

- goby 引擎支持内联 focus 字段（BL-FOC-002）：GobyFinger 新增 Focus（json `focus,omitempty`），Match 命中置 frame.IsFocus，与 fingers 引擎语义一致；新建 goby 包单测 2 个（focus 置位/旧格式兼容）PASS，现库加载取证 Len==7075 行为不变；doc/README.md goby 节与 rule.md 补 focus 字段说明。

- focus 重点关注输出符号 ASCII 化（BL-FOC-001，用户反馈 ⚠️ 在 macOS 等终端错位）：U+26A0+FE0F 改为 `*`，输出形如 `[*shiro]`；修正 rule.md:154 失实描述（"输出时带有focus字样"→"指纹名带 `*` 前缀"）；新建 common 包 framework_test.go 断言输出形态（4 用例 PASS）。build/test 失败项经 stash 基线取证均为 pre-existing。

- BL-FP-004 用户真机验收闭合（复扫目标不再报致远互联-OA/Log4j2，真阳性 PHP/Nginx 保留），从控制台移除；本日累计闭合 8 item，17 item 中 13 已验收

- goby 指纹库误报清理（BL-FP-004）：实测 https://120.77.222.29/yyoa/（PHP 软404站）恒报致远互联-OA，根因=请求路径被页面 canonical 标签回显进响应体 + goby 数据转换剥离原始 path/status_code 上下文后"路径探测"退化为"全响应子串匹配"（goby.go:105）。修复：致远 c 特征 `/yyoa/`→`/yyoa/ext/`（6 个 seeyon poc 实证目录）；SmartOA 摘 `/smartoa/` 独立分支；整删 Log4j2（294 规则/272 随机 label，混入 yyoa/NSX/JBoss 等无关条件的 mega-OR 污染条目）与 HIKVISION-视频监控（80 规则/54 随机 label）；Microsoft-HTTPAPI logic 去悬空 label。7077→7075，validate 全过，目标命中由 [PHP,致远互联-OA,Nginx,Log4j2] 收敛为 [PHP,Nginx]。全库三类判据扫描：dirlist 交集高危项清零，低危 44 项保留记清单。

- 收编上轮会话未立项的 goby 弱特征修改（reviewer 独立核对补全清单）：THE_OPEN_GROUP-UNIX/OpenSSL 整删（弱词）、H3C"h3c"/SIEMENS"s7"/致远"用友致远A"+"path=/yyoa"/HARBOR"Harbor"/ThinkPHP"thinkphp" 弱 label 删、致远 `||f`（SY8044 裸词）引用删（本轮补删其死 rule）、SmartOA label b 删、APACHE-Druid 弱词 "Apache Druid" 改写为 submitLogin&&druid.login 特异组合。

- BL-MSC-005 用户真机验收闭合（下游扫描进行中即见 done 行、cost 为 masscan 自身耗时），从控制台移除；本日累计闭合 7 item，16 item 中 12 已验收

- masscan done 统计行提前到 masscan 退出时打印（BL-MSC-005，用户反馈"任务结束才打印"）：ScanFromReader goroutine 化（三条返回路径并入扫描协程，防 close(Results) 后 send panic）、stdout 计数改原子快照、cost 语义修正为 masscan 自身耗时（不再混入下游扫描时长）；与 BL-MSC-003 同类的生命周期缺陷，模块文档"masscan 结束后打印统计"自此与实现一致

- BL-DIR-003 用户真机验收闭合（worker 跟 -tn、result.txt 无 abort/progress 行），从控制台移除

- 目录扫描 worker 改由 -tn 控制并与 web 并发一致 + abort/progress 不落盘（BL-DIR-003，用户两指令）：`dirWorkerCount`（≤0 回退 16，taskCh 容量固定不随动）；两处熔断 abort 行与 30s 进度行改 Printf 仅终端（-silent 完全静默），enabled/done/Product 落盘不变；本机慢站点冒烟实证 abort 独立成行打终端、result.txt 零 abort/progress（顺带补 Printf 缺失的换行）

- BL-MSC-003/BL-MSC-004/BL-DIR-002 验收闭合（用户真机测试通过）：MSC-003 rate 行为在组合扫描中一并确认；MSC-004 的 0 存活与 Ctrl+C 边缘路径以审查级证据闭合；DIR-002 全部为客观机器证据。三项从控制台移除，累计 14 item 中 10 已验收

- 新增 `-msc -auto` 组合（BL-MSC-004）：`-auto` 探活原逻辑零改动筛出存活 C 段，清单留存 `.msc/msc-targets-<时间戳>.txt`（不自动清理，可 `masscan -iL` 复跑），masscan `-iL` 只扫存活段；组合跳过全 IP icmp 验活（`-np` 无作用）、0 存活提示退出 0、`-hf` 显式拒绝、Ctrl+C 不删留存文件；普通 `-msc` 行为零变化（本机冒烟验证）。经三轮独立审查 APPROVE（闭合 0 存活误判/校验短路/简写归一三项缺陷）

- 修复 TestSealDuringInflightKeepsAllTasks 调度竞速误报（BL-DIR-002，仅测试）：断言"消费第 2 任务时 sealed 必已置位"无 happens-before 保障，go1.20/windows 下 seal 协程稳定输掉起跑（孤立探针 50ms 先机对照实证：10/10 过 vs 原测试 5/5 败）；改为有界等待 waitSealedFor(2s)，生产代码零改动（sealed 只约束拒绝注册与排空 close，置位早晚与在途任务无关）

- 修复 -msc 底部 rate 进度行贯穿整个扫描残留（BL-MSC-003）：`StopProgress` 从"全流程结束后"提前到 stderr 读尽（masscan 退出）时，下游 fscanx 扫描期间终端底部不再钉着最后一帧 `rate: ... 100% done`；masscan 相关单测 race 全过，真机视觉待实跑确认

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
