# POC 扫描模块

覆盖 Web POC 检测能力的触发语义、poc 加载机制与维护体验。引擎复用 fscan 原有的 xray CEL 兼容实现（CEL 是一种表达式语言，poc 里用它声明"响应满足什么条件算打中"，如 `response.status == 200 && response.body.bcontains(b"xxx")`）不改写；本模块解决四件事——触发链断裂（`-poc` 开关静默零请求）、内置 embed 加载导致新增 poc 必须重新编译、指纹→POC 定向管道缺失（全项目独立缺口，此前仅目录扫描 backlog 有记录）、yaml/CEL 手写维护门槛高。

## 关键决策（为什么）

| 决策 | 内容 | 否决过什么 |
|------|------|-----------|
| 放弃 nuclei 兼容 | 保留现有 xray CEL 引擎复用；不引入 projectdiscovery nuclei 重依赖，定位小而精 | nuclei 引擎整库嵌入（依赖膨胀、二进制变大、与 02 资源与内存控制的轻量定位冲突，且内网场景不少模板依赖外网 interactsh）；导入期转换器（放弃 nuclei 后无必要） |
| poc 与指纹资产分层 | poc=用户级资产：`./pocs/` 外置目录运行时生效零编译 + 内置源写回（BL-POC-008）；指纹=项目级资产：保持内嵌 `.gz`（用户拍板），web 端条目化 CRUD（SQLite 主存 + 明文快照双轨，BL-POC-006）加重打包按钮（产物供下次编译嵌入） | 指纹外挂目录（用户拍板不做，指纹随版本发布） |
| 触发三态语义（终态，BL-POC-003 落地） | 空 `-pocname`=智能指纹定向（无命中不打且不提示）；`-pocname all`=全量 386；`-pocname <名字>`=定向过滤（大小写不敏感）。BL-POC-001 过渡期空=全量 | 空=全量作为终态（大网段流量不可控）；无命中回退全量（all 失去显式入口意义） |
| 产品册统一产品对象（BL-POC-009） | 出厂 aliases.yaml（别名归一表）与 binds-seed.yaml（预置名单）合并为 catalog.yaml（标准名+别名+pocs），放识别引擎侧；绑定键=标准名，命中依赖别名区收录该产品全部真实指纹写法（"识别输出=标准名"链）；绑定候选=标准产品列表（挑产品而非输名字）；外置层注入扫描器（别名进归一表 + pocs 进绑定合并） | 两文件分立维护（同一产品对象拆两处漂移——出厂英文旧名键对归一后标准名永不命中即其后果）；界面候选名用指纹明文全集（与引擎输出名不同命名空间，绑了不生效无提示） |
| web 维护端独立二进制 | `fscanx-web` 独立发布（用户拍板），扫描器本体不膨胀 | 内嵌 `-web` 子命令（单二进制但扫描器本体增大） |
| 内置 POC 写回编辑（BL-POC-008） | 编辑内置 POC 可选写回 `-root` 下 `PocScan/pocs/`（下次 go build 生效，编译时机用户自行掌握）；目标文件由 embed 库按 name 服务端解析，必须已存在且内容同名（防 -root 指错检出改错文件/凭空造文件），改名写回拒绝；外置覆盖路径保留为即时生效通道 | 自动双写内置+外置（静默分叉）；外置→内置迁移、新建/改名直写内置（不做，需时另立）；go build 自动触发 |

## 关键功能怎么做

> 终态设计随 item 渐进落地（BL-POC-001~009 已全部落地，待验收；进行中方向见 [backlog 索引](../backlog/10-POC扫描.md)：BL-POC-010 并发口径，闭合时本节同步改写）。

### 触发链

- BL-POC-003 终态已落地（2026-11-15，含 001 触发修复）：`PocScan()` 三态分发——`-pocname all`=全量（`filterPoc("")` 返回全部）；显式名=过滤（大小写不敏感）；**空 `-pocname`=指纹智能定向**：`HostInfo.Frames`（指纹原始产品名，DoWebScan 文本指纹 + favicon 链尾命中 + webpoc 分支 detectProductsSilent 静默首页探测三路填充）查 `PocBinds` 定向执行，无命中不打且不提示（`all` 是唯一全量入口）。webpoc 分支经 `ensureURL`（Url 为空才构造）补 Url；`ParseScantype` 有 `case "webpoc"` 用 Webport。遗留：`-m <模式> + -pa` 追加被模式端口覆盖的顺序问题未动（001 状态记录有记）。
- 实测流量对比（vulhub thinkphp 靶场）：全量 612+ 条 → 智能定向 8 条（约 98.7% 降幅）。

### poc 加载

- BL-POC-002 已落地（2026-11-15）：`initpoc()` 三层合并——内置 `go:embed` 386 个兜底 → **二进制所在目录** `pocs/` 自动加载（存在才加载并打印一行；用二进制目录而非工作目录：内网工具目录习惯，cd 不影响）→ `-pocpath` 显式追加目录。同名 poc 后加载覆盖先加载（外置修正内置），**替换保位**（slice + index map）保证 `filterPoc` 遍历顺序稳定。坏 yml 沿用静默跳过；`-pocpath` 目录空加载时打印提示。新增 poc 流程：yml 丢进二进制旁 `pocs/` 即生效，零编译。

### 产品册（catalog：标准名 + 别名 + 测试名单，BL-POC-009）

- **数据模型**：一个标准产品 = 条目 `{name, vendor?, product?, alias: {指纹源: [写法]}, pocs: [poc名]}`——"产品叫什么"（alias，识别引擎把各指纹源写法归一到标准名输出）与"测什么"（pocs，指纹命中后定向执行的 POC）统一为一个对象。出厂产品册 `mylib/finger/resources/catalog.yaml`（50 条目，顶层列表结构——引擎解析 `[]*Alias` 的前提，pocs 作为条目内未知字段由宽松解析忽略）随二进制编译嵌入；旧 `aliases.yaml` + `binds-seed.yaml` 双文件已合并退役，weaver 拆三个子产品（e-cology/云桥e-bridge/e-office）各挂各的测试。
- **扫描器侧消费**：`catalogSeed()` 按 name+pocs 投影独立解析出厂绑定兜底（alias 字段宽松忽略，alias 包结构零改动）；绑定命中依赖"识别输出 = 标准名"——alias 区写法经 `alias.Compile` 归一（注意：条目名 ToLower，引擎输出为小写形态）。外置产品册 = 二进制旁 `catalog.yaml`（与出厂同构）：`Plugins.InitEngineAliases` 在 main 早期注入——alias 段经 `AppendAliases` 注入归一表（指纹识别前硬时序）、pocs 段经 `SetExternalCatalog` 缓存并入 `loadBindsFile`（MergeBinds 同 name 整条覆盖、异名追加）；外置 catalog.yaml 存在时不再读旧 binds.yaml（兼容兜底）。
- **执行层精确索引**：绑定名单路径按 poc 名建 `name→poc` map 精确取用（修 filterPoc Contains 子串扩张的重复执行——名字互为前缀的两个 poc 只执行被绑定的那个）；**`-pocname <子串>` 与 `all` 的 Contains 模糊语义保持不变**（BL-POC-001 承诺，TestPocNameSubstringGuard 护栏锁定）。
- **web 端外置层存储**：`catalog.db`（SQLite entries 表，二进制旁）+ `catalog.yaml` 快照双轨（哈希吸收手改，同 binds.db 模式）。API 五端点：`/api/catalog_list`（合并视图：出厂基线读 -root 源文件优先防陈旧基底 ⊕ 外置覆盖，墓碑过滤 + missing_alias 警示标）、`/api/catalog_add`（**防死条目守卫**——别名区为空且名字不在指纹输出名全集拒绝保存：识别永远报不出该名，绑定不会命中；名字在全集内放行=自标准名形态）、`/api/catalog_edit`（original 定位 + **target 双目标**：built-in 写回 -root 出厂源文件重编译生效 / external 落 DB 立即生效；整条全量写出铁律——别名区+测试名单全量禁止局部覆盖）、`/api/catalog_delete`（出厂条目落墓碑：别名归一保留、绑定失效）、`/api/catalog_resolve`（批量归属判定三态：alias-of/self/unassigned，指定源优先 + FindAny 跨源兜底 + blocked 语义与引擎一致）。旧 binds_* 端点路由退役（handler 留作 binds_db_test 历史回归）。
- **存量迁移**：`migrateLegacyBinds` 首启一次——binds.db 普通行原名立条（识别输出原名 Contains 命中，行为不变）、墓碑行转出厂别名保留+空 pocs、binds.yaml 吸收、`.migrated` 改名防重；**迁移前后定向发包集合一致**（TestMigratedBindEquivalence httptest 发包级等价锁）。
- **消费端** `boundPocNames(frames)`：命中产品条目 pocs 按表序去重；PocScan 并上 `alwaysPocNames()`（yaml `always: true` 无条件全打型）后精确索引执行。

### web 维护端（fscanx-web，独立二进制，BL-POC-004 已落地 + 前端重做）

- 前端为 **vue-element-plus-admin 3.1.0**（`web/` monorepo：apps/admin + packages 共享包，pnpm workspace，裁掉 docs 子应用）。构建 `bash web/build.sh`（pnpm build → dist-pro → `cmd/fscanx-web/static` go:embed 真源）后 `go build ./cmd/fscanx-web`；开发模式 `cd web && pnpm dev:admin`（vite :4000，/api 代理到 7758，不 strip 前缀）。未来非 POC 功能加页面：Go `api_routes.go` 加一条菜单 + `src/views/` 加组件即可（动态路由 schema：component 'views/Xxx/Xxx' 映射）。
- Go 后端：标准库 net/http，响应统一 `{code:0,data}` 包装（@vea/request 拦截器契约，错误走 200+`{code:1,message}` 信封使指导性错误文本可达前端）；登录认证（BL-POC-007）：默认账号 admin/admin123（`cmd/fscanx-web/auth.go` 常量），登录签发内存会话 token（重启失效），authMiddleware 对 `/api/*`（除登录）校验 Authorization，无效返 `{code:401}` 信封→前端拦截器自动登出回登录页；hash 路由无需 SPA fallback。启动 `fscanx-web -addr 127.0.0.1:7758 -root <仓库根>`；poc/binds 读写扫描器部署目录（二进制旁 `pocs/`、`binds.yaml`，运行时生效），**内置 POC 可选写回内置源**（`-root` 下 `PocScan/pocs/`，下次编译生效，BL-POC-008），指纹/pack 读写仓库根。
- poc 表单编辑：谓词树（匹配项下拉 status/body/header + 操作等于/包含/正则 + header 名，全局 AND/OR）序列化为 CEL；编辑已有 poc 时 CEL 在表单子集内反解析回填（`predicate.go` cel-go AST 白名单），超出子集（嵌套树、多步利用链）自动切源码模式。落盘双重校验：yaml 解析为 Poc 结构 + 每条 expression 在求值环境编译（rules 与 groups 内 rule 均编译；groups 形态无顶层 rules 亦可保存）。bmatches 生态形态是参数反转 `"pattern".bmatches(response.body)`。**保存目标选择（BL-POC-008）**：内置来源行默认写回内置源（目标文件名由 embed 库按 name 服务端解析，存在性/内容同一性/改名三守卫 + temp+rename 原子写），外置行/新建仅外置；同名内置/外置并存按 (source,name) 定位互不串写；getPoc(source=built-in) 编辑视图读仓库最新源（防写回后重载 embed 陈旧基底二次覆盖），列表保持 embed 真源。
- 三视角维护（BL-POC-009，取代 BL-POC-005 绑定关系页）：**标准产品视角**（新页 `views/Catalog/Catalog.vue`，菜单"绑定关系"→"标准产品"）：条目列表（标准名/别名 tag 带源 tooltip/测试名单/来源出厂·外置/missing_alias 警示标）+ 客户端搜索分页 + 弹窗 CRUD——新建强制挂别名（候选辅助 el-select-v2 从指纹写法全集挑选 + 按四源 goby/ehole/fingerprinthub/fingers 行编辑别名区；POC 多选同虚拟化——候选近万项普通 el-select 全量渲染会卡弹窗，BL-POC-008 性能前科），编辑出厂条目带 target 双目标 radio（写回内置源重编译生效 / 落外置覆盖立即生效，BL-POC-008 保存目标先例）。**指纹视角**（Finger.vue）：产品归属列（批量 catalog_resolve 三态——标准产品/「某某」的别名/未归属警示 tooltip 说明归一语义）+ 所属产品绑定测试预览列 + **归属操作弹窗**（行内"归属"按钮：未归属态可挂为已有标准产品别名（确认文案写明映射改显示名——铁律 5）或自立标准产品（别名自动挂该写法，天然满足防死条目）、自立成功弹窗切已归属态可顺手绑定 POC；已归属态显示测试名单（tag closable 移除）+ 追加绑定）；resolve 的 source 按引擎源名映射（goby 页→goby、fingers_http 页→fingers）。**POC 视角**（Poc.vue 绑定弹窗）：候选=标准产品列表（挑产品而非输名字，缺别名条目标注"绑了不命中"）、读条目→追加 pocs→整条写回 external、已绑 tag closable 解绑。
- 指纹条目编辑（BL-POC-006）：条目列表（goby/fingers_http 切换+搜索+分页）+ 单条结构化表单（goby：name/focus/logic/rule[label,feature,is_equal]；fingers_http：name/focus/link/tag/favicon md5/mmh3）+ 新增/复制/删除。存储 finger.db（SQLite 主存，(file,id) 复合主键——重名条目各自独立）+ 明文 JSON 快照双轨：**自研 JSON 数组扫描器逐字节切分模板**（前缀/逐间隙分隔符/元素原文字节），重生成保证未动条目字节零漂移；哈希吸收手改（文件被手改→以文件为准全量吸收）；schema 判定降级原文模式（default_port/favicon.path/异形嵌套等超 schema 形态字段零丢失）；删除即消失、手改加回=吸收复活（无墓碑——权威单层）。生效模型不变：明文→pack→重编译。
- 界面文案与框体纪律（BL-POC-011，新页面跟随）：长说明按三分法收敛——操作后果警示内联一行、字段/背景解释进问号 tooltip（全局 Icon mdi:help-circle 悬浮范式；百字长提示加 popper-class=help-tip 限宽 360px，popper 传送 body 故须全局类）、废话删除；规格统一：业务弹窗 width=680px、.dlg-field label 60px 右对齐、控件/分页 size=small、表格 border、卡片 shadow=never、head-actions gap 8px。否决：重做视觉主题、抽公共弹窗组件重写三页结构（成本超诉求）。
- 二期候选：均已落地——指纹结构化条目编辑（BL-POC-006）、谓词表单嵌套树 UI（BL-POC-004 迭代轮 CondTree.vue）、内置 POC 写回（BL-POC-008）。

## 机制与引擎复用边界

- `PocScan/lib` 全套（CEL 求值、请求执行、dnslog、响应体 4MB 限长）不改；本期只动触发层（`WebScan.go` 的 PocScan/Execute/filterPoc/initpoc）与管道层（`webScanTask.go`）。
- 输出契约不变：命中行格式 `[+] PocScan <url> <poc名> <详情>`（`PocScan/lib/check.go:41`）。
