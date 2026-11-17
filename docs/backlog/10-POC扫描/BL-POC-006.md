# BL-POC-006 指纹库 SQLite 化 + 结构化条目编辑（CRUD）

- 模块：POC 扫描
- 优先级：P1
- 状态：已完成
- 当前阶段：持续迭代
- 价值：指纹维护从"textarea 整文手改数千条 JSON"升级为条目列表+结构化表单+完整增删改复制——修改面收敛到单条、格式由重生成器保证，改完 pack 供下次编译；生效模型不变（保持编译嵌入，用户 2026-11-16 重申"指纹不需要外置，只修改内置指纹 json"）。
- 关联模块文档：docs/modules/10-POC扫描模块.md
- 关联计划：plans/2026-11-16-BL-POC-006-指纹结构化CRUD.md
- 关联代码：cmd/fscanx-web/finger_db.go、cmd/fscanx-web/api_finger.go、cmd/fscanx-web/finger_db_test.go、web/apps/admin/src/views/Finger/Finger.vue、web/apps/admin/src/api/finger/index.ts
- 验收状态：已验收

### 方案（立项时必填）

- **做什么**：参照 binds_db.go 模式把两份指纹明文的条目级维护迁入 SQLite（finger.db 放二进制旁，同 binds.db 位；条目按 **(file, rowid) 复合定位**——实测 fingers_http 有 30 组重名条目，name 不构成身份；条目以原始 JSON 字节落库保结构零建模成本）：单条增删改复制 API（/api/finger_list、/api/finger_add、/api/finger_edit、/api/finger_delete，按项目命名规范；旧 GET/PUT /api/finger 整文端点退役，POST /api/pack 保留）；每次变更后重生成明文——**未动条目原文字节原样保留**，改动/新增条目按所在源格式契约序列化（goby：2 空格缩进+首行 `[{`+无结尾换行；fingers_http：4 空格缩进+无结尾换行），temp+rename 原子写 + sha256 记录；读写前哈希对齐吸收手改（文件被手改→以文件为准全量吸收进 DB，同 binds 机制）；web 端指纹页从 textarea 整文升级为条目列表（file 切换+搜索+分页）+ 单条结构化表单（goby：name/focus/rule[label,feature,is_equal]；fingers_http：name/link/focus/tag/favicon md5/mmh3）+ **条目级降级原文编辑**（降级判据：logic 不是本条 rule label 的 &&/|| 布尔式、或条目含表单 schema 外字段/rule 异形嵌套 → 整条走原文编辑，同 POC 编辑器 form/source 双模式先例）。
- **为什么**：现状整文 textarea 手改单条要人肉定位数千条 JSON 段，格式敏感（BL-POC-004 曾踩 \u 转义/整体重序列化坑）；条目级 CRUD 后修改面收敛单条、格式由重生成器保证。用户 2026-11-16 主动诉求"和 POC 编辑器一样易用"+确认完整 CRUD 范围（含新增/删除/复制）。
- **怎么做**：finger_db.go（条目表+meta 哈希表；DB 空→首启从 repoRoot 明文全量导入原始字节+建哈希基线）→ 明文重生成器（按源格式契约）→ API 四端点（go1.20 无方法路由，按 binds_* 先例逐个注册 handler）→ 前端 Finger.vue 条目化重构（列表+表单+降级原文编辑）。
- **为什么这么做**：与 binds 存储化同一分层决策（DB 主存+文件快照+哈希吸收）。**不做墓碑**——binds 墓碑为防"DB 删行后 embed 种子复活"（引擎消费 {product,[]} 空覆盖），指纹权威只有 DB 一层、快照由 DB 重生成，不存在复活路径；删除语义=删除即消失，手改明文把条目加回→哈希失配吸收→按手改内容复活（尊重手改）。否决"未动条目也整体重序列化"——两源格式契约不同（2/4 空格），统一序列化=全库格式漂移，恰是本 item 要消灭的事。否决"运行时外挂加载"（用户既定拍板：保持编译嵌入，编译时机用户自行掌握）。

### 范围

- cmd/fscanx-web/finger_db.go（新：存储层）+ api_finger.go 改造（旧整文端点退役、新四端点、pack 保留）
- web/apps/admin/src/views/Finger/Finger.vue 条目化重构（列表+表单+降级原文编辑）
- 明文 JSON 格式契约固化（按源序列化规则，pack 输入不变）

### 不做

- 指纹运行时外挂加载（保持编译嵌入）
- 引擎 Match 逻辑变更
- 指纹内容治理（误报清理归 05 模块）
- 批量导入（整文粘贴多条新增）——另立
- ehole/fingerprinthub 等其他数据源条目化（仅 goby/fingers_http 两明文）

### 验收条件

- [x] 条目列表（file 切换+搜索+分页）可用；单条结构化编辑 round-trip 一致——实测覆盖四类条目：goby 纯 rule（Comtrend）语义深度相等 0 差/7200、含 focus（Kedacom-NVR 命中）、fingers_http 纯 favicon（jira 字段级 round-trip 一致/全库规范化语义 0 差）、含 default_port（portainer 原文同文编辑 md5 零漂移）
- [x] 新增/删除/复制实测闭环：删除后明文该条消失（553 条）；手改明文把该条加回（回写备份字节）→哈希失配吸收→复活且明文逐字节=手改；新增/复制真实库实测（修复后复测，见状态记录）
- [x] 重生成格式逐字节契约：真实库种子后两明文 md5 与备份一致（8a10fc7a/0f835d6e）；pack 产物 md5 三方全等（web pack==CLI pack==原始基线 9f449a93/fa5cd883）
- [x] 降级判据生效：portainer（default_port）UI 锁定原文模式实测；单测缩进契约+审查 agent 扫描全库确认无误进表单条目
- [x] 手改坏 JSON 显式报错（信封提示"解析失败…修正后重试"），报错后明文 md5 原状、listProducts 正常返回且不产出坏数据
- [x] 旧 GET/PUT /api/finger 退役（GET 实测 404），新端点行为正确，前端零残留调用（getFingerApi/putFingerApi 已删）
- [x] headless 产物版交互链零 console 错误（14/14）；go build ./... 过（build+vet+单测全绿）
- [x] 文档五件套同步 + check-sync 无 HIGH（收尾执行）

### 依赖

- BL-POC-005（binds_db.go 存储层模式与哈希吸收先例，已闭合）

### 状态记录

- 2026-11-16：方向立项草案（用户在 BL-POC-005 范围确认时批注"并且指纹也可以这样做"）。优先级 P2 为草拟值，**待用户确认优先级后 committed 转 Mode D**。
- 2026-11-16：**升级立项（Mode B0 澄清 + 独立审查 REQUEST_CHANGES 修订后用户批准 P1）**。范围从"SQLite 存储化"升级为"存储化+结构化条目 CRUD"：完整增删改复制、列表搜索分页、按源 schema 表单+条目级降级原文；拍死两项契约——旧整文端点退役（前端整文读写随本 item 改造）、生效模型保持 pack+重编译（用户明示"指纹不需要外置，只修改内置指纹 json"，编译时机用户自行掌握）。审查必改项修订：①条目身份改 (file, rowid) 复合主键（fingers_http 实测 30 组重名，name 不唯一）②重生成契约改"未动条目字节保留"（goby 实测 2 空格缩进+首行 `[{`+无尾换行，原"4 空格统一"会全库漂移）③删墓碑机制（权威单层无复活路径，删除+手改加回=尊重手改吸收复活）④表单 schema 补 focus/default_port 长尾字段覆盖 ⑤API 退役决策与验收补齐。数据勘察入档（审查 agent 实测）：goby 7200 条（564 含 focus、logic 422 个不同值、2 处嵌套异形 rule）；fingers_http 554 条（hash-only 547、mmh3 列表型、default_port 5 条/level 1 条/非 favicon rule 形态若干、favicon.path 2 处、无结尾换行）。
- 2026-11-16：**执行完成（Mode D 四层验证）**。交付三层（详见 plans/2026-11-16-BL-POC-006-指纹结构化CRUD.md）：①存储层 finger_db.go——自研 JSON 数组扫描器逐字节切分（json.Decoder InputOffset 会前视吞间隙，弃用）{前缀/逐间隙分隔符/后缀/元素原文字节} 模板；seed/absorb/regenerate（temp+rename+sha256）；(file,id) 主键、无墓碑。②API 四端点+copy，两源 canonical 序列化（缩进契约 goby ("","  ")/fingers ("    ","    ")，键序=name/focus/link/rule/tag 实测约定），form 模式拒绝 schema 外键；旧整文端点退役。③前端 Finger.vue 条目化（列表+表单/原文双模式弹窗+判定降级锁定）。**Reviewer 轮（sonnet，10 findings：1C/1H/2M/6L）全处置**：C serializeFingerArray 间隙越界（add/copy 后元素数>间隙数→提交后 panic→DB/明文分叉，agent 独立复刻实证 index out of range）→ serialize 复用末间隙修复+新增字节契约回归锁；H fingerEdit 缺 absorb（编辑重生成覆灭未吸收手改）→补对齐+live 验证手改存活；M 空数组种子丢 `[`（含 prefix/suffix 区间重叠二次修正）；M 新增弹窗原文模式死控件→新增态禁用；L md5>1 元素降级/schema 外键拒绝/RowsAffected 错误直传/add 回归测试补齐。**如实记录一处异常**：首版一轮 live add 曾返回 saved 而该版代码该路径经实证必 panic——成因未复现，该轮证据作废，以修复后全链复测为准。**验证（修复后）**：真实库 7200/554 种子后两明文 md5 零漂移；四类条目 round-trip（语义深度相等 0 差/7200、jira 字段级一致、portainer 原文同文 md5 零漂移）；add/copy live 复测通过（7201/命中 2）+编辑不覆灭手改（手改标记存活+编辑生效）；删除→手改回写→吸收复活且明文逐字节=手改；pack 三方 md5 全等；坏 JSON 显式报错+明文原状+listProducts 不受污染；旧端点 404；单测全绿；UI headless 产物版 14/14 零 console 错误；终态真实文件四 md5 恢复基线。置已完成+待验收（UI 观感待用户浏览器确认）。
- 2026-11-16：验收闭合（用户真机实测通过，含绑定产品弹窗性能修复后的完整链路）。置已验收。
