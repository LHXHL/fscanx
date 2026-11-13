# project-control-plane scripts

本 skill 配套工具。skill 协议（SKILL.md / 执行协议 / 查询验收协议）多处引用这些工具。

## check-sync.py — 文档一致性校验

扫描 `docs/backlog/*.md` 所有 item，报告五类问题：

| 类别 | 严重度 | 说明 |
|------|--------|------|
| 孤儿引用（关联计划/模块文档不存在） | HIGH | 路径不存在，需建文件或修字段 |
| 状态漂移 | HIGH | backlog 验收状态 vs 控制台状态词不一致（真实漂移，人工复核） |
| 体积超标 | MED | backlog 文件 > 500 行 / 单 item 块 > 150 行，建议按 item 拆文件 |
| 控制台孤儿 | MED | 控制台出现的 ID 在 backlog 不存在（笔误/已删） |
| 孤儿引用（关联代码） | LOW | backlog 常写相对路径/简称，脚本无法猜全路径，提示人工核对 |
| 模块文档覆盖 | LOW | item 关联的模块文档有「变更记录」节但未提该 ID |

### 用法

```bash
python .claude/skills/project-control-plane/scripts/check-sync.py           # 文本报告（退出码 0=无发现 / 1=有发现）
python .claude/skills/project-control-plane/scripts/check-sync.py --json    # JSON 输出（供 agent 解析）
```

### 何时跑

- Mode A（状态评审）/ Mode F（阶段验收）前跑一次，快速定位漂移
- Mode D 收尾同步后跑一次，确认文档对齐（硬禁止 8/9/10 的机械校验）
- 任何"同步文档"任务开始前跑一次，建立修复基线

### 局限

- 控制台状态词为自由文本，启发式解析（按行内关键词共现），多 ID 共行跳过避免误报
- 关联代码字段 backlog 惯用相对路径/简称（`api/vshell.ts` 实为 `impl/frontend/.../api/vshell.ts`），存在性检查会误报 LOW，人工核对
- 状态漂移只报"验收类"差异（已验收/待验收/未验收），"已完成 vs 已验收"语义等价不报

格式契约见 `.claude/skills/project-control-plane/references/项目约定.md`。