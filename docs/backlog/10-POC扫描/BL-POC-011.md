# BL-POC-011 web 维护端 UX 整理：文案 tooltip 化 + 框体尺寸统一

- 模块：POC 扫描（fscanx-web 维护端）
- 优先级：P2
- 状态：已完成
- 当前阶段：持续迭代
- 价值：把"每次打开页面都要读一遍的长说明"变成"需要时才悬浮查看的提示"，页面更干净；弹窗/表格/按钮尺寸规格统一，三页观感一致
- 关联模块文档：docs/modules/10-POC扫描模块.md
- 关联计划：plans/2026-11-17-BL-POC-011-web-ux-polish.md
- 关联代码：web/apps/admin/src/views/Catalog/Catalog.vue、web/apps/admin/src/views/Finger/Finger.vue、web/apps/admin/src/views/Poc/Poc.vue、web/apps/admin/src/views/Poc/CondTree.vue
- 验收状态：已验收

### 方案

- **做什么**：三个业务页（标准产品/指纹/POC）+ 四个业务弹窗做纯展示层整理：①长文案按三分法处理——操作后果警示保留一行内联、字段/背景解释改问号图标 el-tooltip（复用 ContentWrap 组件的 title+message 悬浮范式）、重复废话直接删除；②尺寸规格统一——四弹窗宽度统一 680px、表单 label 列宽三页同值、按钮统一 small、分页统一 small、表格统一带 border、卡片 shadow=never、页头操作区间距统一 8px；③清理 Finger.vue 重复定义的 .dlg-field 样式块（同文件两段同名样式，后段 100px 把前段 60px 盖掉——CSS 后写的同名规则覆盖先写的，这是三页 label 列宽不一致的根因）。
- **为什么**：BL-POC-008/009 两轮快速加功能（内置 POC 写回编辑 + 标准产品体系三视角维护端）在三页留下 20+ 处平铺直出的长说明文字（页顶 alert、弹窗内 hint、行内 sub 小字）与 4 种弹窗宽度（560/600/680/720）、两套 label 宽度（60/100px）、混排按钮尺寸；用户 2026-11-17 提出：删冗长文字改悬浮提示、统一框体。
- **怎么做**：不换主题不换字体不引入依赖，Element Plus 2.14.4 现有能力内完成：页顶 alert → 卡片标题旁问号 tooltip；弹窗 hint → label 旁问号 tooltip 或删除；统一规格落三页 scoped 样式对齐 + 删除重复块；构建链 `bash web/build.sh`（pnpm 构建产物嵌入 cmd/fscanx-web/static go:embed 真源）→ `go build ./cmd/fscanx-web`。
- **为什么这么做**：tooltip 化而非全删——部分文案（如"绑定命中依赖识别输出=标准名，别名区要收录全部真实写法"）是防错知识，删了用户会绑出永不命中的死条目；警示类保内联——后果性信息（如"写回内置源需重新编译才生效"）藏进悬浮会变成"没看到的错误提示"。弹窗宽度取单一 680px（用户批准，四值的中位）：观感最整齐，代价是归属/绑定两个轻弹窗留些空白。否决重做视觉主题（超出本次诉求、回归风险大）；否决抽公共组件大改三页结构（以 ContentWrap 作范式参考，但不强推页面重写）。

### 范围

- Catalog.vue / Finger.vue / Poc.vue（含 CondTree.vue 所在 POC 编辑器区）的模板文案与 scoped 样式
- 四个业务弹窗：标准产品新建/编辑（Catalog）、指纹条目编辑（Finger）、产品归属（Finger）、POC 绑定标准产品（Poc）
- 统一规格：弹窗 width=680px、.dlg-field label 宽三页同值、el-button size=small、el-pagination size=small、表格 border、卡片 shadow=never、页头操作区间距 8px
- 构建产物重嵌入（cmd/fscanx-web/static）+ go build 验证

### 不做

- 不动后端 Go API / 数据契约 / 交互逻辑与流程
- 不动 Dashboard/Login/模板演示页（vue-element-plus-admin 模板自带）
- 不换 UI 主题、字体、配色，不引入新依赖
- 不做响应式/移动端适配改造

### 验收条件

- [x] 三业务页无平铺超过两行的常驻说明段落；原长文案语义在对应 tooltip 中可查（"删除文案 → tooltip 落点"映射表记入状态记录，防搬运丢失）
- [x] 弹窗内 15 字以上的解释性 hint 全部 tooltip 化或删除；操作后果警示保留内联且不超过一行
- [x] 四弹窗 width 全部 680px（grep 逐弹窗对照）
- [x] Finger.vue .dlg-field 重复样式块删除；三页 label 列宽同值（对照表验证）
- [x] 业务页全部 el-button 显式 size=small、el-pagination size 统一 small、三个表格统一 border、卡片 shadow=never、页头操作区间距统一 8px
- [x] 回归最小清单实测通过：三页查询/搜索/分页 + 标准产品新建/编辑（含双目标 radio）+ 指纹条目编辑（结构化/原文两模式）+ 归属弹窗三操作（挂别名/自立/追加绑定）+ POC 绑定弹窗 + POC 编辑器保存，行为无回归
- [x] `bash web/build.sh` 通过、`go build ./cmd/fscanx-web` 通过、产物版浏览器实测无白屏无回归（用户确认后闭合）
- [x] docs/CHANGELOG.md、模块文档、backlog 索引、控制台按同步矩阵更新；check-sync 无 HIGH（实测 0 HIGH，2 条 LOW 为 009 合并/退役遗留的历史 item 存量注记，非本轮漂移）

### 依赖

- 无前置依赖（BL-POC-009 已落地的三视角页为改动基底）

### 状态记录

- 2026-11-17：立项。用户批准 P2 立项并执行、弹窗宽度单一 680px 策略。独立评审四维度全 PASS（抽查 5 处代码证据全部命中），5 条验收加固建议已吸收（宽度定死 680、border 定死统一有、间距定死 8px、回归最小清单、文案映射表）。进入 Mode D 执行。
- 2026-11-17：实现完成。改动 4 文件（Catalog/Finger/Poc.vue + packages/styles/src/index.less 全局 help-tip 限宽类——tooltip popper 传送 body，scoped 样式够不到），+135/-85 行。四层验证：①自检——验收 1-5 条 grep 对照全过（四弹窗全 680px、label 三页同值 60px、按钮/分页全显式 small、三表格 border、head-actions gap 8px、无常驻说明段落，剩余 hint 均为允许清单内的条件性后果警示）；②reviewer 独立验收（sonnet code-reviewer，按验收标准逐条采证非"是否完成"）7 条全 PASS，APPROVE 附 1 条 should-fix 已修——schema 锁定警示缩写丢失「或形态」语义（多 favicon 哈希触发场景字段全在 schema 内却被描述为"含表单外字段"，误导排查），补回「或形态（如 default_port、多 favicon 哈希）」；③deslop 扫 diff 无发现（.head-title/.help-icon 三页 scoped 小块重复符合仓库既有每页 scoped 惯例，不抽公共组件）；④回归——`bash web/build.sh`（vite 产物重嵌入 cmd/fscanx-web/static，go:embed all: 前缀在位，产物含下划线 js 正常嵌入）+ `go build ./cmd/fscanx-web` 双过。文案→tooltip 映射表 23 项留档 plans/2026-11-17-BL-POC-011-web-ux-polish.md。遗留移交用户实测：①radio-group 内包 el-tooltip 的原文化模式按钮外观与键盘导航（inject 链理论安全，运行时未测）；②tooltip 观感。回归最小清单与产物版浏览器实测待用户确认，验收状态置待验收。
- 2026-11-17：**用户真机实测通过（产物版），验收闭合**。回归最小清单（三页查询/弹窗 CRUD/归属三操作/绑定/编辑器保存）逐项走过无回归；遗留移交项一并确认：radio-group 内 tooltip 包裹的原文模式按钮外观正常（inject 链判断成立）。意外发现移交：POC 编辑器保存区 radio-group 与两个按钮存在几像素垂直错位（保存区容器为普通 div 非 flex、子元素基线排版所致——本轮前即存在，按钮统一 small 后高度已齐但基线错位仍在；纯视觉微调归 trivial 通道，用户自行修复作为前端排查练习）。
