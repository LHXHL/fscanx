# POC 扫描模块

覆盖 Web POC 检测能力的触发语义、poc 加载机制与维护体验。引擎复用 fscan 原有的 xray CEL 兼容实现（CEL 是一种表达式语言，poc 里用它声明"响应满足什么条件算打中"，如 `response.status == 200 && response.body.bcontains(b"xxx")`）不改写；本模块解决四件事——触发链断裂（`-poc` 开关静默零请求）、内置 embed 加载导致新增 poc 必须重新编译、指纹→POC 定向管道缺失（全项目独立缺口，此前仅目录扫描 backlog 有记录）、yaml/CEL 手写维护门槛高。

## 关键决策（为什么）

| 决策 | 内容 | 否决过什么 |
|------|------|-----------|
| 放弃 nuclei 兼容 | 保留现有 xray CEL 引擎复用；不引入 projectdiscovery nuclei 重依赖，定位小而精 | nuclei 引擎整库嵌入（依赖膨胀、二进制变大、与 02 资源与内存控制的轻量定位冲突，且内网场景不少模板依赖外网 interactsh）；导入期转换器（放弃 nuclei 后无必要） |
| poc 与指纹资产分层 | poc=用户级资产：`./pocs/` 外置目录运行时生效零编译；指纹=项目级资产：保持内嵌 `.gz`（用户拍板），web 端编辑明文源文件加重打包按钮（产物供下次编译嵌入） | 指纹外挂目录（用户拍板不做，指纹随版本发布） |
| 触发三态语义（终态，BL-POC-003 落地） | 空 `-pocname`=智能指纹定向（无命中不打且不提示）；`-pocname all`=全量 386；`-pocname <名字>`=定向过滤（大小写不敏感）。BL-POC-001 过渡期空=全量 | 空=全量作为终态（大网段流量不可控）；无命中回退全量（all 失去显式入口意义） |
| binds 键=引擎输出产品名 | 绑定关系键用 fingers 引擎统一输出的产品名（goby.json / fingers_http.json / favicon 哈希三源合并后的输出），与数据源文件解耦 | 按数据源文件分多套绑定表（同产品在多源间名称易漂移） |
| web 维护端独立二进制 | `fscanx-web` 独立发布（用户拍板），扫描器本体不膨胀 | 内嵌 `-web` 子命令（单二进制但扫描器本体增大） |

## 关键功能怎么做

> 终态设计随 item 渐进落地（BL-POC-001 进行中，002~004 未开始）；各 item 闭合时本节同步改写为实际行为。

### 触发链

- BL-POC-003 终态已落地（2026-11-15，含 001 触发修复）：`PocScan()` 三态分发——`-pocname all`=全量（`filterPoc("")` 返回全部）；显式名=过滤（大小写不敏感）；**空 `-pocname`=指纹智能定向**：`HostInfo.Frames`（指纹原始产品名，DoWebScan 文本指纹 + favicon 链尾命中 + webpoc 分支 detectProductsSilent 静默首页探测三路填充）查 `PocBinds` 定向执行，无命中不打且不提示（`all` 是唯一全量入口）。webpoc 分支经 `ensureURL`（Url 为空才构造）补 Url；`ParseScantype` 有 `case "webpoc"` 用 Webport。遗留：`-m <模式> + -pa` 追加被模式端口覆盖的顺序问题未动（001 状态记录有记）。
- 实测流量对比（vulhub thinkphp 靶场）：全量 612+ 条 → 智能定向 8 条（约 98.7% 降幅）。

### poc 加载

- BL-POC-002 已落地（2026-11-15）：`initpoc()` 三层合并——内置 `go:embed` 386 个兜底 → **二进制所在目录** `pocs/` 自动加载（存在才加载并打印一行；用二进制目录而非工作目录：内网工具目录习惯，cd 不影响）→ `-pocpath` 显式追加目录。同名 poc 后加载覆盖先加载（外置修正内置），**替换保位**（slice + index map）保证 `filterPoc` 遍历顺序稳定。坏 yml 沿用静默跳过；`-pocpath` 目录空加载时打印提示。新增 poc 流程：yml 丢进二进制旁 `pocs/` 即生效，零编译。

### 绑定关系（binds，两层）

- schema：顶层 `binds` 数组，元素 `product`（fingers 引擎输出产品名，goby/fingers_http/favicon 三源合并后的输出）/ `pocs`（poc 的 yaml `name` 字段列表）。无条件全打型不在此表达——已迁至 poc yaml 自身 `always: true` 顶级标记（BL-POC-004 迭代轮，用户 UX 反馈：always 是 poc 属性而非绑定关系属性）。
- BL-POC-003 已落地两层加载：`PocScan/binds-seed.yaml` **内置种子**（go:embed，原 fscan PocDatas 14 条产品经 `plans/scripts/gen_binds_seed.go` 转出的内置库快照，15 产品 53 绑定，可重跑刷新）打底 + 二进制旁 `binds.yaml` 外置**同 product 整条覆盖、异名追加**（`mergeBinds` 纯函数）。种子兜底保证智能定向不因缺外置文件而失效。
- 消费端 `boundPocNames(frames)`：命中产品条目 pocs 按表序去重；PocScan 空态再并上 `alwaysPocNames()`（遍历 AllPocs 取 yaml `always: true` 者，无条件全打型）去重后执行。单测覆盖解析/种子/合并/查询/always 名单。
- web 端存储层（BL-POC-005）：fscanx-web 以 SQLite（`binds.db`，二进制旁，modernc.org/sqlite 纯 Go 驱动）主存**外置层**，`binds.yaml` 降级为"DB 变更后自动重生成的快照 + 手改入口"双轨——扫描器零改动（启动时照旧读 种子 ⊕ binds.yaml）。同步机制：每次 API 读写前比对文件 sha256 与 meta 表记录，不一致=被手改→全量吸收进 DB→重生成；文件缺失按 DB 重生成（防误删丢数据）；坏 yaml 显式报错。API：`/api/binds_list`（种子⊕外置合并视图）、`/api/binds_add`（upsert-append）、`/api/binds_edit`、`/api/binds_delete`，PUT /api/binds 退役。**墓碑语义**：种子起源行（仅存在于 embed 种子）的编辑/删除落 `{product, pocs:[]}` 墓碑条目——引擎 MergeBinds 同名覆盖后空 pocs=绑定失效（boundPocNames 对空 pocs 为空操作），合并视图过滤墓碑；墓碑不可被二次删除移除（幂等），重新添加同名产品即自然复活。种子起源产品的 binds_add 以种子 pocs 为基线追加（避免快捷绑定单 poc 覆盖种子全部绑定）。

### web 维护端（fscanx-web，独立二进制，BL-POC-004 已落地 + 前端重做）

- 前端为 **vue-element-plus-admin 3.1.0**（`web/` monorepo：apps/admin + packages 共享包，pnpm workspace，裁掉 docs 子应用）。构建 `bash web/build.sh`（pnpm build → dist-pro → `cmd/fscanx-web/static` go:embed 真源）后 `go build ./cmd/fscanx-web`；开发模式 `cd web && pnpm dev:admin`（vite :4000，/api 代理到 7758，不 strip 前缀）。未来非 POC 功能加页面：Go `api_routes.go` 加一条菜单 + `src/views/` 加组件即可（动态路由 schema：component 'views/Xxx/Xxx' 映射）。
- Go 后端：标准库 net/http，响应统一 `{code:0,data}` 包装（@vea/request 拦截器契约，错误走 200+`{code:1,message}` 信封使指导性错误文本可达前端）；登录认证（BL-POC-007）：默认账号 admin/admin123（`cmd/fscanx-web/auth.go` 常量），登录签发内存会话 token（重启失效），authMiddleware 对 `/api/*`（除登录）校验 Authorization，无效返 `{code:401}` 信封→前端拦截器自动登出回登录页；hash 路由无需 SPA fallback。启动 `fscanx-web -addr 127.0.0.1:7758 -root <仓库根>`；poc/binds 读写扫描器部署目录（二进制旁 `pocs/`、`binds.yaml`，运行时生效），指纹/pack 读写仓库根。
- poc 表单编辑：谓词树（匹配项下拉 status/body/header + 操作等于/包含/正则 + header 名，全局 AND/OR）序列化为 CEL；编辑已有 poc 时 CEL 在表单子集内反解析回填（`predicate.go` cel-go AST 白名单），超出子集（嵌套树、多步利用链）自动切源码模式。落盘双重校验：yaml 解析为 Poc 结构 + 每条 expression 在求值环境编译。bmatches 生态形态是参数反转 `"pattern".bmatches(response.body)`。
- 绑定编辑器（BL-POC-005 交互重构）：表格只读展示（产品名文本 + POC tag），浏览态零下拉；每行编辑/删除按钮，添加/编辑共用弹窗表单（产品名 el-autocomplete 只在编辑态出现，候选=指纹库命名空间过滤按 精确>前缀>子串 排序 + 命名空间语义提示），客户端分页（10/20/50）；增删改确认即写盘（弹窗化后无未保存态）；同名产品拦截提示走行编辑。POC 页编辑器头部"绑定产品"弹窗（当前 POC 已绑产品只读 tag + 多选新增，复用 binds_add upsert-append 免跳页）。产品名数据源仍为两指纹明文 name 全集（`/api/products`）。
- 指纹编辑：textarea 整文编辑——"内容即所写"= 零格式漂移（实测三方 md5 一致）+ JSON 合法性校验；重打包按钮走 `mylib/finger/packlib` 共享包（cmd/pack 同一实现，产物字节等价）。
- 二期候选：谓词表单嵌套树 UI、指纹结构化单条目编辑。

## 机制与引擎复用边界

- `PocScan/lib` 全套（CEL 求值、请求执行、dnslog、响应体 4MB 限长）不改；本期只动触发层（`WebScan.go` 的 PocScan/Execute/filterPoc/initpoc）与管道层（`webScanTask.go`）。
- 输出契约不变：命中行格式 `[+] PocScan <url> <poc名> <详情>`（`PocScan/lib/check.go:41`）。
