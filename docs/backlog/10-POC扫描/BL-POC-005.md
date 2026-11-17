# BL-POC-005 绑定关系页交互重构 + POC 页快捷绑定

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
