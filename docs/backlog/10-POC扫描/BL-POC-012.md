# BL-POC-012 文件名可见性：POC 列表展示 yaml 文件名 + 产品册维护文件路径

- 模块：POC 扫描（fscanx-web 维护端）
- 优先级：P2
- 状态：已完成
- 当前阶段：持续迭代
- 价值：POC 内部名与 yaml 文件名经常不一致，按名找不到文件；标准产品条目存在哪个文件里页面看不到——文件可见 + 可搜后，定位文件不再靠猜
- 关联模块文档：docs/modules/10-POC扫描模块.md
- 关联计划：plans/2026-11-17-BL-POC-012-filename-visibility.md
- 关联代码：web/apps/admin/src/views/Poc/Poc.vue、web/apps/admin/src/views/Catalog/Catalog.vue
- 验收状态：待验收

### 方案

- **做什么**：纯前端两文件。①Poc.vue 列表加「文件名」列（内置行=embed 真实文件名、外挂行=pocs/ 目录文件名，超长省略悬浮看全名），搜索框同时匹配名称与文件名，编辑器标题悬浮显示当前打开的文件名，左列卡片 380→430px 腾列宽；②Catalog.vue 把「存放位置」tag 的悬浮说明换成具体完整路径（出厂=built_in_path / 外挂=external_path），页头问号 tooltip 补两行维护文件路径。
- **为什么**：用户 2026-11-17 反馈——poc name 与文件名不一致，很难搜索定位到 yaml 文件；标准产品条目存在哪个文件里页面不可见。数据契约核查：后端 `/api/pocs` 列表早已返回 `file` 字段（api_poc.go pocItem.File，内置=embed 真实文件名）、`catalog_list` 早已返回 `built_in_path` + `external_path`（api_catalog.go）——前端只差展示与搜索匹配，零后端改动。
- **怎么做**：Poc.vue 的 filtered 计算扩展匹配 `(p.file ?? '')` + 列表加列 + placeholder 改「搜索名称/文件名...」+ 编辑器标题 el-tooltip 显示 selFile；Catalog.vue load() 补读 external_path 存 ref，「存放位置」tooltip 与页头问号 tooltip 引用两个路径（复用 BL-POC-011 的 help-icon 悬浮范式与 help-tip 限宽）。
- **为什么这么做**：加列而非只进 tooltip（用户拍板）——文件名是要复制、肉眼扫的定位目标，藏悬浮里等于没解决"难搜索"；路径进 tooltip 而非常驻文本（用户拍板）——遵守 BL-POC-011 定下的文案纪律（解释进悬浮，页面不堆常驻说明）。否决后端加字段（数据已具备，加了就是过度设计）；否决 Finger 页同改（其维护文件名 goby.json/fingers_http.json 已在页头可见）。

### 范围

- web/apps/admin/src/views/Poc/Poc.vue（文件名列、搜索语义、编辑器标题悬浮、左列卡片宽度）
- web/apps/admin/src/views/Catalog/Catalog.vue（external_path 读取、存放位置 tooltip、页头 tooltip）

### 不做

- 不动后端 Go API / 数据契约
- 不动 Finger 页（维护文件名已可见）
- 不做"复制路径/文件名"按钮等附加功能

### 验收条件

- [x] POC 列表新增文件名列：内置行显示 embed 真实文件名、外挂行显示 pocs/ 下文件名，超长省略且悬浮可见全名
- [x] POC 搜索框输入文件名片段可命中对应行；placeholder 文案反映新语义（名称/文件名）
- [x] 编辑器选中某行后，标题悬浮可见该 poc 当前文件名
- [x] 标准产品页存放位置 tag 悬浮显示具体完整路径（出厂/外挂各对应正确文件）；页头问号 tooltip 含两个维护文件路径
- [ ] `bash web/build.sh` + `go build ./cmd/fscanx-web` 通过；产物版实测无回归（用户确认后闭合）
- [x] CHANGELOG / 索引 / 控制台 / 模块文档（如涉及行为描述）按同步矩阵更新（check-sync 0 HIGH，2 条 LOW 为历史 item 存量注记；模块文档无被推翻的行为描述，无需改写）

### 依赖

- 无前置（BL-POC-011 已验收，其 tooltip 范式与文案纪律直接复用）

### 状态记录

- 2026-11-17：立项。数据契约核查（/api/pocs 返回 file、catalog_list 返回 built_in_path+external_path）证实纯前端展示缺口；用户批准两项展示方案（POC 列表加文件名列 + 产品册路径 tooltip 化）。
- 2026-11-17：实现完成（2 文件 +22/-6）。四层验证：①自检 + ②reviewer 独立验收 7 条全 PASS（APPROVE；vue-tsc --noEmit 全量零错误；1 条 LOW 观察项——编辑器标题 tooltip 兜底文案「新建未保存」在现有数据流下不可达，openPoc 走后端 findPocFile 命中时 file 恒非空，保留作防御性冗余不改）；③deslop 扫 diff 无发现；④`bash web/build.sh` + `go build ./cmd/fscanx-web` 双过，产物含新文案（Poc-DUcp9eIr.js / Catalog-dHElFSA8.js 命中），产物时间戳晚于源文件确认为新鲜构建。用户手工修复的保存区 flex 布局（.save-actions）零触碰。产物版浏览器实测待用户确认，验收状态置待验收。
