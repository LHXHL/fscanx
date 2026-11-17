# plans/2026-11-17-BL-POC-011-web-ux-polish.md

## 目标

fscanx-web 三业务页（标准产品 Catalog / 指纹 Finger / POC）纯展示层整理：冗长说明文字按三分法收敛（警示内联一行 / 解释进问号 tooltip / 废话删除），框体尺寸规格统一（弹窗 680px、label 列宽同值、按钮/分页 small、表格 border、gap 8px），并清理 Finger.vue 重复 .dlg-field 样式块。

## 当前状态

- 2026-11-17：三页改造完成（文案 tooltip 化 + 尺寸统一 + Finger 重复样式块删除），构建验证中。
- 2026-11-17：立项完成（用户批准 P2 + 单一 680px）。item 已落档，开始实现。

## 文案映射表（删除文案 → tooltip 落点，验收证据）

| 原文案（位置） | 处理 | 新落点 |
|---|---|---|
| Catalog 页顶 alert「这里维护…编辑时可选择」 | tooltip 化 | 页标题旁问号图标 |
| Catalog 弹窗·别名区下「绑定命中依赖识别输出=标准名…」 | tooltip 化（防错知识） | 「别名」区标题旁问号 |
| Catalog 弹窗·测试名单下「绑定命中宽松匹配（含子串）…」 | tooltip 化 | 「测试名单」区标题旁问号 |
| Catalog 弹窗·保存位置三态提示（新建/内置/转正） | **保留内联**（后果警示，各 ≤1 行） | 原位 |
| Finger 页顶 alert「条目化编辑…手改会被吸收」 | tooltip 化 | 页标题旁问号图标 |
| Finger 编辑弹窗·schema 锁定警示 alert | **保留内联**（条件警示）+ 36 字缩至 34 字 | 原位 |
| Finger「新增走表单结构化；原文形态条目请复制相近条目…」(.sub) | tooltip 化 | 「编辑模式」行问号图标（仅新增态显示） |
| Finger「focus：命中时输出 [*产品名] 高亮」（goby .sub） | tooltip 化 | 「特别关注」开关旁问号（fingers 表单同步补齐同款） |
| Finger「is_equal=false 为排除条件…」(.hint) | tooltip 化 | 「全等」开关本体悬浮 |
| Finger「整条 JSON 原文编辑（内容即所写；保存校验…）」(.hint) | tooltip 化 | 「原文模式」radio 悬浮 |
| Finger 归属弹窗「绑定保存到外挂产品册文件…也可在标准产品页编辑」 | tooltip 化 | 「绑定」按钮旁问号 |
| Finger 归属弹窗「挂为别名后…归一显示为目标标准名」 | **删除** | 冗余——挂为别名确认框已含同义文案 |
| Finger 归属弹窗「以该写法为别名新建标准产品…」 | tooltip 化 | 「标准名」输入框旁问号 |
| Poc「always：不依赖指纹命中…」(.hint) | tooltip 化 | 「无条件扫描」开关旁问号 |
| Poc 列头「set：单值变量（值是表达式，如 randomInt…）」 | 拆分 | 可见「set：单值变量」+ 问号 tooltip 含示例 |
| Poc 列头「sets：列表变量（值一行一个）」 | 拆分 | 可见「sets：列表变量」+ 问号 tooltip |
| Poc 步骤标题内「｜按序执行：任一步不满足即停止…」(.sub) | tooltip 化 | 「请求步骤」标题旁问号 |
| Poc「continue（爆破模式专用）…」(.hint) | tooltip 化 | 「遍历输出」开关旁问号 |
| Poc search 长 placeholder（37 字） | 缩短 + tooltip 补语义 | placeholder「正则提取变量，供后续步骤引用」+「变量提取」行问号（留空跳过/提取失败语义） |
| Poc「写入代码源文件 PocScan/pocs/，重新编译后…」(.hint) | **保留内联**（后果警示，1 行） | 原位 |
| Poc raw 模式 alert（rawNote/默认提示） | **保留**（条件显示，非常驻） | 原位 |
| Poc 绑定弹窗「绑定后指纹命中…缺别名条目绑了不命中」 | tooltip 化（候选列表已标注缺别名） | 「新增绑定」行问号 |
| Finger label「favicon md5 / favicon mmh3」 | 改名「图标 md5 / 图标 mmh3」 | 适配三页统一 60px label 列 |

## 尺寸规格统一记录

- 弹窗宽度：680/720/600/560 → **全部 680px**（用户批准单一档）
- .dlg-field label 列宽：Finger 100px（重复样式块级联覆盖，832-846 行）→ 删除重复块 → **三页统一 60px**
- 按钮：Poc 保存/删除外置（默认尺寸混排）→ **small**；三页 + CondTree 全量复核均有显式 size
- 分页：Catalog 无 size → **small**（Finger 原有）
- 表格 border：Finger/Poc 无 → **加 border**（Catalog 先例）
- head-actions gap：Poc 12px → **8px**（Catalog/Finger 基准）；Poc 左右卡片布局 gap 12px 保留（布局间距，非操作区间距）
- 死 CSS 清理：Finger `.hint`/`.sub`/`.dlg-hint`/重复 `.dlg-field` 块删除；Poc `.section-title .sub` 删除

## 执行计划

1. 通读三页 + CondTree.vue 源码，锁定全部文案点位与样式点位。
2. 文案三分法改造（页顶 alert → 卡片标题问号 tooltip；弹窗 hint → label 问号 tooltip / 删除；警示保内联 ≤1 行）。
3. 尺寸统一：弹窗 width=680、.dlg-field 对齐（含删 Finger 重复块）、按钮/分页 small、表格 border、shadow=never、head gap=8px。
4. 构建验证：`bash web/build.sh` → `go build ./cmd/fscanx-web`。
5. Reviewer 独立验证（按验收标准）+ deslop 扫改动文件。
6. 同步矩阵收尾：CHANGELOG / 模块文档「关键功能怎么做」web 节核对 / 索引 / 控制台 / check-sync。

## 风险

- tooltip 藏住防错知识 → 用映射表核对，警示类一律内联。
- vite 产物 go:embed 白屏前科（下划线 js）→ 构建链用项目现成 build.sh，终验跑产物版。
- 交互回归（弹窗 CRUD/双目标 radio/两步式绑定）→ 回归最小清单逐项走。

## 验证方式

- 验收条件逐条收集证据（grep 对照表 + 构建输出 + 浏览器产物版实测）。
- check-sync.py 无 HIGH。

## 决策记录

- 弹窗宽度：单一 680px（用户批准；否决两档制——观感整齐优先）。
- 表格：统一带 border（Catalog 先例为准，三页一致）。
- 文案三分法：警示内联 / 解释 tooltip / 废话删除（防错知识不进悬浮，后果性提示不进悬浮）。
