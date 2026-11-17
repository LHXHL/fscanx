---
name: project-control-plane
description: Manage continuous project delivery through a project console, module backlog ledgers, scoped context packages, verification, and state synchronization. Use when reviewing project status, planning next steps, clarifying vague requirements, generating/executing backlog items, auditing recent changes, or running consensus planning for high-risk work.
triggers:
  - 项目控制台
  - backlog
  - Backlog
  - 阶段交付
  - 阶段范围
  - 当前阶段
  - 下一步做什么
  - 执行 backlog
  - 开发流程
  - 项目状态
  - 需求澄清
  - 共识规划
  - 需求模糊
  - 高风险项
  - 一直运行
  - 持续运行
  - 持续执行
  - 直到完成
  - 做完所有
  - 不用询问
  - 不要停顿
---

# Project Control Plane

持续演进项目的控制面：`docs/项目控制台.md` 是唯一人类入口，`docs/backlog/*.md` 是 item 状态唯一事实源，`docs/modules/*.md` 是模块需求与设计。文档四层金字塔、第0-2层叙事白话（语言规范见 `references/项目约定.md`）。每个任务只加载所选 item 的上下文包（封顶四样），实现后按同步矩阵（`references/执行协议.md`）同步五件套：backlog / 控制台 / CHANGELOG / 模块文档 / 设计总览；**item 验收闭合即从控制台删除**（归档不残留）。

语言契约：避免一次性交付词（MVP/上线/交付），用持续迭代词（阶段范围/阶段验收/演进路线/能力地图）。"MVP"仅当用户明示或既有标题含该词时使用。

## 硬禁止（违反即失控，无豁免）

1. 🚫 禁用 `EnterPlanMode`——本技能有自建规划体系（B0/B/C/G/H），内置 plan 流程会跳过 backlog/CHANGELOG 文档维护。
2. 🚫 禁用 `oh-my-claudecode:planner` agent 替代 Mode H 的 Planner→Architect→Critic 循环（缺 steelman antithesis 与 quality gate）。
3. 🚫 非琐碎任务无 backlog item 不得直接实现——先建 backlog 再走 Mode D。
4. 🚫 plan 批准后必须维护文档——按同步矩阵同步 backlog/控制台/CHANGELOG。
5. 🚫 入口下的功能行为变更不算 trivial——涉及行为/API/数据模型/搜索语义/任务流转/文档状态/验收口径，改 1 行也走 B/D/D-lite；**例外：合法路径零变化且 grep 无文档漂移的防御性修复走 D-mini（直改+CHANGELOG），见「When not to use」**。
6. 🚫 "先聊聊"草案 ≠ 已批准方案——用户说"批准/开始做"后重新判定 Mode，补齐最小文档。
7. 🚫 未获显式授权不改运行时外部状态（Meilisearch/DB schema/Redis/Celery/服务配置/远端 issue/PR）——先说明影响范围等确认。
8. 🚫 禁止伪"已验收"——`验收状态：已验收` 必须三条件同时满足（验收条件全勾 + 状态记录有实测通过闭合 + backlog 状态非"未开始"），任一不满足置 `待验收`/`未验收`（见 `项目约定.md`「伪完成判定」）。本次实践曾发现 5 处伪已验收（BL-04-018/BL-09-009/BL-09-010/BL-04-016/BL-09-014）。
9. 🚫 禁止关联字段写不存在路径——`关联计划`/`关联代码`/`关联模块文档` 路径必须存在或写 `—`，"待建"占位不得长期残留（见 `项目约定.md`「关联字段有效性」）。
10. 🚫 禁止控制台索引行退化为 mini-backlog——索引表/演进项列表只存汇总，item 详情回 backlog，单行 ≤ 200 字符（见 `项目约定.md`「控制台与索引行规则」）。

## 输出沟通约定：业务视角优先

向用户反馈问题、请示决策、汇报结果时，**回复以一段业务视角摘要开头**，再展开技术细节。这是默认语气，不是可选修饰。

- **业务摘要**（开头 1–3 句）：用业务语言抽象归纳"发现了什么问题 / 要请示什么决定 / 影响什么业务"，让不懂代码的用户秒懂 AI 想表达什么。不出现文件名、函数名、行号；技术术语首次出现配一句白话（如"CPE——业界给软件发的身份证号"）。一个具体例子贯穿比罗列清单有效。
- **正文**（摘要之后）：技术证据、定位、选项、影响——给能看懂的人，或留作后续排查依据。

为什么：控制面决策是按业务影响拍板的，不是按代码细节。技术细节是支撑不是开场——用户看不懂的开场等于没汇报。纯机械执行（跑脚本、改 typo、纯查数返回）可省摘要。

## 路由：进哪个 Mode

三个问题定路由；完整流程加载对应协议文件（见「加载协议」）。

**① 持续运行？** 用户明示"一直运行 / 持续运行 / 持续执行 / 做完所有 / 不用询问 / 不要停顿 / 直到完成 / 批准你一直运行"或点名多 item（"执行 BL-001 到 BL-003"）→ **D-Chain**。

**② 只读 or 写？**
- "只检查 / 只审查 / 只出报告 / 不要改代码" → **R** 审计
- "项目到哪了 / 下一步做什么 / 阶段状态" → **A** 状态评审
- "阶段完成了吗 / 验收检查" → **F** 阶段验收
- "同步一下 backlog 状态 / 同步文档" → 先跑 `python .claude/skills/project-control-plane/scripts/check-sync.py` 建立漂移基线，再按 `执行协议.md` 同步矩阵执行
- "文档太大 / 重构文档 / 瘦身 / 拆分 backlog" → **S** 文档维护（跑 check-sync 定位 + 按体积/漂移结果拆分归档，见 `执行协议.md` 同步矩阵 + 体积阈值）
- 其余 → 写类，走 ③

**③ 有无已确认的 backlog item？**
- 无：
  - 需求模糊 / 新功能 / "帮我理清需求" → **B0** 需求澄清
  - 把需求/模块文档转成 backlog → **B** backlog 生成
  - "当前阶段做什么" / 阶段规划 → **C** 阶段规划
  - "进入下一阶段" → **G** 阶段过渡
- 有：
  - 高风险（P0 + 跨模块 | DB 迁移 | 安全认证 | >20 文件 | API 破坏性变更）→ **H** 共识规划
  - 标准风险 → **D** 执行；入口下小行为修复（≤3 文件、无 schema/API/安全/跨模块/外部状态变化）→ **D-lite**
  - 合法路径零变化且无文档漂移的防御性修复（参数校验改报错/文案/告警，见「When not to use」）→ **D-mini** 直改

路由时若范围模糊，先看 `references/规划协议.md` 的「模糊检测」。

## When not to use

typo 修复、纯视觉微调（列宽/颜色）、不涉及项目流的纯问答、用户明示的紧急直改 → 直接做，跳过本技能。

trivial 与 D-mini 的判定看**文档漂移风险**（有无已记录的行为契约被推翻），不看"是否碰行为"：

- **trivial（直接做）**：仅 typo / 措辞 / 纯视觉，无任何行为影响。
- **D-mini（直改 + CHANGELOG 一行，不建 item 不建 plan）**：合法路径零变化的防御性修复——参数校验改报错、错误/日志文案、纯新增告警。准入四条件：①合法路径行为不变 ②grep docs/ 无已记录行为被推翻 ③≤5 文件 ④非安全认证/DB 迁移/API 破坏/跨模块。事后用户报告相关问题时凭 git 历史补 item 升档。
- 行为契约变化且会被文档描述（API 形态/搜索语义/数据形态/任务流转）→ 不是 D-mini，按路由走 D/D-lite。

## 加载协议（渐进式披露，强制）

按 Mode 加载对应协议文件，**只读所需节**（各文件内 Mode 互斥，读整文件后按节标题聚焦，不全量展开）：

| 场景 | 必读文件 | 聚焦节 |
|------|---------|--------|
| 路由判断 | 本文件 | 路由 + 加载协议 |
| 进 D | `references/执行协议.md` | 必答清单 + 同步矩阵 + Mode D + 失败处理 |
| 进 D-mini | `references/执行协议.md` | Mode D-mini + 同步矩阵 |
| 进 D-lite | `references/执行协议.md` | 必答清单 + 同步矩阵 + Mode D-lite + 失败处理 |
| 进 D-Chain | `references/执行协议.md` | 必答清单 + 同步矩阵 + Mode D-Chain + 失败处理 |
| Mode S 文档维护 | `references/执行协议.md` | 同步矩阵 + 跑 `python .claude/skills/project-control-plane/scripts/check-sync.py` 按体积/漂移结果拆分归档 |
| 同步文档状态 | `references/执行协议.md` | 同步矩阵（收尾核对） |
| 进 B0 | `references/规划协议.md` | 模糊检测 + Mode B0 + 规划类失败处理 |
| 进 B | `references/规划协议.md` + `references/项目约定.md` | Mode B + 约定的「编号」「状态与优先级」「Backlog item 格式」 |
| 进 C / G | `references/规划协议.md` + `references/项目约定.md` | Mode C/G + 约定的「编号」 |
| 进 H | `references/规划协议.md` | 全文（高风险共识需完整流程） |
| 进 A | `references/查询验收协议.md` | Mode A |
| 进 R | `references/查询验收协议.md` | Mode R |
| 进 F | `references/查询验收协议.md` | Mode F |
| backlog 格式核对 | `references/项目约定.md` | 「状态与优先级」「Backlog item 格式」（唯一权威，勿在他处重复定义） |

汇报统一三件套：`现状 / 下一步 / 需确认`；Mode 特有字段见各协议步骤。
