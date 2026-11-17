# BL-POC-004 独立 fscanx-web 维护端一期

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
