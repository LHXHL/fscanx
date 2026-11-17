# BL-POC-008 内置 POC 写回编辑（保存目标：内置源 / 外置）

- 模块：POC 扫描
- 优先级：P1
- 状态：已完成
- 当前阶段：持续迭代
- 价值：内置 POC 在 web 端修改后写回内置源目录（PocScan/pocs/），下次 go build 自然带上——"想内置进二进制的 POC"编辑-固化闭环打通；外置保存路径保留不变。
- 关联模块文档：docs/modules/10-POC扫描模块.md
- 关联计划：plans/2026-11-16-BL-POC-008-内置POC写回编辑.md
- 关联代码：cmd/fscanx-web/api_poc.go、cmd/fscanx-web/api_poc_target_test.go、web/apps/admin/src/views/Poc/Poc.vue、web/apps/admin/src/api/poc/index.ts
- 验收状态：已验收

### 方案（立项时必填）

- **做什么**：POC 编辑器保存时可选写回目标——内置源（`<repoRoot>/PocScan/pocs/<embed 真实文件名>`，下次编译生效）或外置（现有 pocs/ 覆盖语义，零编译即生效）。配套：listPocs/getPoc 透出 embed 真实文件名与来源，getPoc 新增 source 参数按来源定位（同名内置/外置并存时打开哪条改哪条，列表行以 (source, name) 复合标识）；putPoc 新增 target 参数（取值沿用既有契约词 built-in|external），target=built-in 时校验目标文件已存在于 `<repoRoot>/PocScan/pocs/`（root 未指向仓库或文件不存在→明确拒绝，仅可外置）；落盘双重校验（yaml round-trip + CEL 编译）复用。前端保存区加目标选择（内置来源 POC 默认内置源，外置 POC 仅外置）。
- **为什么**：内置 POC 经 go:embed 编进二进制运行时只读，现状编辑内置 POC 只能另存外置副本，把修改固化进下次编译产物没有入口（2026-11-16 用户明确提出，并明示"下次 go build 自然带上，编译时机用户自己知道"）。
- **怎么做**：api_poc.go 改造（pocItem 加 File 字段、getPoc/putPoc 加 source/target 参数、内置写回路径与存在性校验）+ Poc.vue 保存目标选择 UI；扫描器零改动。
- **为什么这么做**：写回源文件直接符合用户语义；用 embed 真实文件名避免 name→文件名推导出错（内置文件名与 name 不保证一致）；目标文件必须已存在——防止 -root 指向版本不同的检出时凭空造文件；显式选目标而非自动双写，避免内置/外置静默分叉；保存目标枚举沿用 pocItem.Source 既有词 built-in|external 不新造 internal。

### 范围

- cmd/fscanx-web/api_poc.go（pocItem/getPoc/putPoc 改造）
- web/apps/admin/src/views/Poc/Poc.vue（保存目标选择 UI）

### 不做

- 新建 POC 直写内置源（新建仍走外置）
- 外置→内置迁移（外置 POC 固化进二进制暂无入口，需时另立）
- 外置文件名可配置（维持 sanitizeName(name).yml；覆盖按 POC name 字段匹配，文件名=name 是列表合并视图对齐的最简约定）
- go build 自动触发（编译时机用户自行掌握）
- 列表同名双行合并展示改造
- fscanx 扫描器本体改动（加载覆盖语义不变）

### 验收条件

- [x] 编辑内置来源 POC 选"写回内置源"：PocScan/pocs/ 对应源文件内容更新（沙箱实测 marker 落盘）；外置 pocs/ 零新文件且既有文件字节不变（md5 实测）；未重编译时 listPocs 保持 embed 真源；重编译后新内容生效（重建二进制 grep marker=1，恢复后归零）
- [x] -root 未指向仓库、或目标文件在 PocScan/pocs/ 不存在时：内置保存被拒绝且提示明确（实测缺文件拒绝信封；指错检出场景由内容同一性守卫覆盖——单测身份回落+守卫子测试）
- [x] 同名内置+外置并存：编辑内置行保存→内置源文件更新且外置同名文件字节不变（反向同理）；两条各自打开内容互不串（双向字节+内容实测）
- [x] 写回内置源落盘前校验生效（坏 yaml/坏 CEL 被拦截，源文件 md5 前后一致）
- [x] headless 产物版交互链零 console 错误（11/11）；go build ./... 过（build+vet+test 全绿）
- [x] 文档五件套同步 + check-sync 无 HIGH（收尾执行，见状态记录）

### 依赖

- 无（BL-POC-004 编辑器与校验链已闭合）

### 状态记录

- 2026-11-16：立项（Mode B0 澄清 + 独立审查修订后用户批准 P1，批准连做 BL-POC-008→006）。用户拍板模型：编辑内置 POC 写回内置源目录供下次编译，"保存到外置"保留；编译时机用户自行掌握。审查修订：保存目标枚举统一 built-in|external；验收"旧二进制行为不变"（恒真不可测）改为外置目录字节不变+未重编译 API 内容不变的可操作断言；补 getPoc source 参数契约与"目标文件必须已存在"防错仓写回；不做清单补"外置→内置迁移不做"。
- 2026-11-16：**执行完成（Mode D 四层验证）**。交付：①后端 api_poc.go——pocItem/pocDetail 透出 file/source；getPoc 收 source 参数（built-in 编辑视图**仓库源文件优先**防写回后重载 embed 陈旧基底二次覆盖，身份不符回落 embed；listPocs 保持 embed 真源）；putPoc 加 target=built-in|external（内置写回：embedPocByName 按 yaml 校验后的 name 服务端解析目标文件名 + 存在性/内容同一性/改名三守卫 + temp+rename 原子写）；groups 形态放行且 groups 内 rule 过 CEL 编译；poc 域错误面统一 httpErrorf 信封。②前端 Poc.vue/api——行点击传整行按 (source,name) 定位，保存目标 radio（仅内置来源行出现，默认内置源）+ 提示行，删除外置对内置行禁用，reopenCurrent 按保存目标重开，API 错误带 code 后 catch 去重 toast。③新增 api_poc_target_test.go（source 三语义/身份回落/写回成功/四守卫/groups 放行/原子写无残留）。**验收条件修订**：原文"未重编译时 getPoc 返回内置内容不变"修订为 getPoc(built-in) 读仓库源文件最新内容——写回后编辑器重载 embed 旧版会以陈旧基底二次保存覆盖写回成果（跨刷新丢数据路径）；listPocs 保持 embed 真源语义不变。**意外发现并修复存量 bug**：来源列 el-table formatter 签名错位（`(_,__,row)` 第三参实为 cellValue，`"external".source` 恒 undefined）致来源列**恒显"内置"**、外置行不可辨识——修为 (row) 一参签名。**Reviewer 轮（sonnet，10 findings：1H/3M/6L）全处置**：H groups 放行（原"rules 为空"校验使 31/386 groups 形态任何目标均不可保存且报错误导）、M reopen 锚定保存目标、M 版本偏斜身份守卫（getPoc 回落+putPoc 拒绝）、M 改名守卫、L 错误面信封化/getPoc·deletePoc/gofmt/原子写/新增单测/双 toast 去重已修；L CRLF 归一化（写回统一 LF，autocrlf=true 下 git 无感）与 -addr 非回环并发写为已知取舍记录在案。验证：go build/vet/test 全绿；沙箱 API 实测（写回/三类拒绝/双行互不串写双向字节/坏 yaml+CEL 拦截且源文件不破坏/改名守卫/groups 保存/原子写无残留）；重编译 embed 携带新内容 grep 证实（真实源树临时写入→编译→字节级恢复→归零，git 干净）；UI headless 产物版 11/11 零 console 错误。置已完成+待验收（UI 观感待用户浏览器确认后验收闭合）。
- 2026-11-16：验收闭合（用户真机实测通过）。置已验收。
