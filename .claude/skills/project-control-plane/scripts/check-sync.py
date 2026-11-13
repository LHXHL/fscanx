#!/usr/bin/env python3
"""项目文档一致性校验（project-control-plane 配套工具）。

扫描 docs/backlog/*.md 所有 item，报告：
  1. 孤儿引用：关联计划/关联代码/关联模块文档 路径不存在
  2. 体积超标：backlog 文件 > 500 行 / 单 item 块 > 150 行
  3. 模块文档覆盖：item 声明关联模块文档但该文档无该 ID
  4. 控制台覆盖：backlog ID 未在控制台出现 / 控制台 ID 在 backlog 不存在
  5. 状态值漂移：backlog item 验收状态 vs 控制台/backlog 索引行状态词不一致
  6. 控制台行规则：单行 >200 字符 / 划线闭合项残留 / 总行数 >150
  7. 目录索引完整性：backlog 目录的索引文件缺失 / item 未入索引 / 索引缺「状态汇总」节
  8. 内容漂移（防总览/模块文档失实）：设计总览能力地图断链与模块未收录 /
     控制台模块计数 vs backlog 实算 / 总览「上次全面核对」超 45 天 /
     索引表行与汇总计数 vs item 字段实算 / 活跃 item 缺席控制台 / 哨兵行缺失

用法：python .claude/skills/project-control-plane/scripts/check-sync.py [--json]
退出码：0 无发现 / 1 有发现

状态值漂移中"控制台自由文本状态词"用关键词匹配（已完成/进行中/未开始/已验收/待验收/未验收），
控制台文案为自由文本，机器解析是启发式，结果供人工复核。

格式契约见 .claude/skills/project-control-plane/references/项目约定.md
"""
from __future__ import annotations

import datetime
import json
import os
import re
import sys
from dataclasses import dataclass, field
from pathlib import Path

# Windows 控制台默认 GBK，强制 stdout UTF-8 避免中文乱码
try:
    sys.stdout.reconfigure(encoding="utf-8")
except (AttributeError, ValueError):
    pass

def find_repo_root() -> Path:
    """从脚本位置向上找含 docs/backlog 的目录作为仓库根。

    教训（2026-10-22）：旧实现固定 parent.parent.parent 解析到 .claude/skills/，
    docs/backlog 不存在 → rglob 空转 → 永远报"✅ 无发现"——五周文档漂移全部漏报的根因。
    找不到根必须报错退出，禁止静默空跑通过。
    """
    for anc in Path(__file__).resolve().parents:
        if (anc / "docs" / "backlog").is_dir():
            return anc
    raise SystemExit("🛑 找不到仓库根：任意上级目录均无 docs/backlog。检查脚本位置或目录结构。")


ROOT = find_repo_root()
BACKLOG_DIR = ROOT / "docs" / "backlog"
CONSOLE = ROOT / "docs" / "项目控制台.md"

FILE_LINE_WARN = 500
ITEM_LINE_WARN = 150
CONSOLE_LINE_LIMIT = 200   # 控制台单行字符上限（项目约定.md「控制台与索引行规则」）
CONSOLE_TOTAL_WARN = 150   # 控制台总行数报警（目标 ≤60，活跃项多时放宽）

# backlog item 编号支持三种形态：BL-001（模块单段数字）、BL-MEM-001（字母模块码）、BL-03-001（数字模块号）
ID_PATTERN = r"BL-(?:\d+|[A-Za-z]{2,})(?:-\d+)?"

# 状态关键词（用于控制台自由文本启发式解析）
STATUS_KEYWORDS = {
    "已完成": "已完成",
    "进行中": "进行中",
    "未开始": "未开始",
    "已验收": "已验收",
    "待验收": "待验收",
    "未验收": "未验收",
    "验收未通过": "验收未通过",
}


@dataclass
class Finding:
    severity: str  # HIGH / MED / LOW
    category: str
    location: str
    description: str


@dataclass
class Item:
    id: str
    file: str  # backlog 文件名
    start: int  # 起始行号（1-based）
    block_lines: int
    status: str = ""        # 状态：字段
    verify: str = ""        # 验收状态：字段
    plans: list[str] = field(default_factory=list)
    code: list[str] = field(default_factory=list)
    modules: list[str] = field(default_factory=list)


# backlog item 标题：模块单文件形态 `## BL-...`，per-item 文件形态 `# BL-...`（H1）
ITEM_RE = re.compile(r"^#{1,2} (" + ID_PATTERN + r")\b")
# 字段行：- 字段名：值
FIELD_RE = re.compile(r"^- (状态|验收状态|关联计划|关联代码|关联模块文档)：(.*)$")


def split_paths(raw: str) -> list[str]:
    """从字段值提取路径 token：剥括号注释、按分隔符分割、只留像路径的。

    backlog 关联代码常含省略写法，需跳过避免误报孤儿引用：
    - `impl/frontend/.../views/x.vue`  用 `...` 省略中间段
    - `vshell/{files,terminal}.vue`    用 `{a,b}` glob 并列
    - `views/vshell/*.vue`             用 `*` glob
    - `api/vshell.ts；impl/...go`       用 `；` 分隔（已在 rstrip 处理）
    - `proc_windows.go/proc_unix.go`   斜杠并列两文件名（无目录前缀，不像路径）
    """
    raw = re.sub(r"（[^）]*）", "", raw)
    raw = re.sub(r"\([^)]*\)", "", raw)
    # 按 、/,/，/；/; 分割
    parts = re.split(r"[、,，；;]", raw)
    out = []
    for p in parts:
        p = p.strip().rstrip(".").rstrip("；").rstrip(";")
        if "/" not in p:
            continue
        if re.search(r"\s", p):
            continue
        # 省略/glob 写法跳过
        if "..." in p or "{" in p or "}" in p or "*" in p:
            continue
        out.append(p)
    return out


def parse_backlog(path: Path) -> list[Item]:
    """解析单个 backlog 文件，返回 item 列表（含块行数）。"""
    lines = path.read_text(encoding="utf-8").splitlines()
    items: list[Item] = []
    cur: Item | None = None
    in_fields = False  # 字段区 = item 标题后、首个 ### 节前（防止状态记录里形如"- 状态：x"的行覆盖字段）
    for i, line in enumerate(lines, start=1):
        m = ITEM_RE.match(line)
        if m:
            if cur is not None:
                cur.block_lines = i - cur.start
            cur = Item(id=m.group(1), file=path.name, start=i, block_lines=0)
            items.append(cur)
            in_fields = True
            continue
        if cur is None:
            continue
        if line.startswith("#"):
            in_fields = False
            continue
        fm = FIELD_RE.match(line) if in_fields else None
        if fm:
            key, val = fm.group(1), fm.group(2).strip()
            if key == "状态":
                cur.status = val
            elif key == "验收状态":
                cur.verify = val
            elif key == "关联计划":
                cur.plans = split_paths(val)
            elif key == "关联代码":
                cur.code = split_paths(val)
            elif key == "关联模块文档":
                cur.modules = split_paths(val)
    if cur is not None:
        cur.block_lines = len(lines) - cur.start + 1
    return items


def parse_console_status(console_text: str) -> dict[str, str]:
    """启发式：从控制台文本提取每个 ID 附近的状态词。

    控制台 backlog 索引表与演进项列表为自由文本。按行扫描，
    仅当该行**只出现一个 ID** 时才归属状态词（多 ID 共行无法精确归属，
    跳过避免误报——这也是控制台多 item 挤一行的结构问题提示）。
    返回 {id: 状态词}，状态词优先取「验收类」>「进行类」。
    """
    result: dict[str, str] = {}
    id_re = re.compile(ID_PATTERN)
    for line in console_text.splitlines():
        ids = id_re.findall(line)
        if not ids:
            continue
        # 多 ID 共行：状态词无法精确归属，跳过（不猜）
        unique_ids = set(ids)
        if len(unique_ids) > 1:
            continue
        id_ = next(iter(unique_ids))
        # 该行出现的状态关键词
        found = [kw for kw in STATUS_KEYWORDS if kw in line]
        # 优先级：验收类 > 状态类
        verify_kws = ["已验收", "验收未通过", "待验收", "未验收"]
        status_kws = ["已完成", "进行中", "未开始"]
        chosen = next((k for k in verify_kws if k in found), None)
        if chosen is None:
            chosen = next((k for k in status_kws if k in found), None)
        if chosen is None:
            continue
        rank = {"已验收": 5, "验收未通过": 4, "待验收": 3, "未验收": 2,
                "已完成": 1, "进行中": 0, "未开始": -1}
        if id_ not in result or rank[chosen] > rank.get(result[id_], -99):
            result[id_] = chosen
    return result


def main() -> int:
    out_json = "--json" in sys.argv
    findings: list[Finding] = []

    all_items: list[Item] = []
    # ---- 检查 1+2：孤儿引用 + 体积 ----
    # rglob 递归扫子目录：支持 backlog 按模块拆成目录 + per-item 文件（如 09-插件运行器/BL-09-019.md）
    for f in sorted(BACKLOG_DIR.rglob("*.md")):
        flines = f.read_text(encoding="utf-8").count("\n") + 1
        # 体积阈值针对「单文件模块形态」（item 正文内联）；目录形态的索引文件（根下 <NN>-x.md
        # 且存在同名目录 <NN>-x/）不算 item 正文，跳过体积检查（拆分处方对索引文件是误诊）
        is_dir_index = f.parent == BACKLOG_DIR and (BACKLOG_DIR / f.stem).is_dir()
        if flines > FILE_LINE_WARN and not is_dir_index:
            findings.append(Finding("MED", "体积", f.name,
                                    f"backlog 文件 {flines} 行 > {FILE_LINE_WARN}，建议按 item 拆文件"))
        items = parse_backlog(f)
        all_items.extend(items)
        for it in items:
            if it.block_lines > ITEM_LINE_WARN:
                findings.append(Finding("LOW", "体积", f"{f.name}:{it.id}",
                                        f"item 块 {it.block_lines} 行 > {ITEM_LINE_WARN}，状态记录可能过长，建议归档"))
            for p in it.plans:
                if not (ROOT / p).is_file():
                    findings.append(Finding("HIGH", "孤儿引用", f"{f.name}:{it.id}",
                                            f"关联计划不存在：{p}"))
            for c in it.code:
                # backlog 关联代码常写相对路径/简称（api/vshell.ts 实为 impl/frontend/.../api/vshell.ts），
                # 脚本无法猜全路径，存在性失败降为 LOW（提示人工核对）而非 MED。
                if not (ROOT / c).exists():
                    findings.append(Finding("LOW", "孤儿引用", f"{f.name}:{it.id}",
                                            f"关联代码不存在（可能为相对路径/简称，人工核对）：{c}"))
            for m in it.modules:
                if not (ROOT / m).is_file():
                    findings.append(Finding("HIGH", "孤儿引用", f"{f.name}:{it.id}",
                                            f"关联模块文档不存在：{m}"))
                else:
                    # 仅对含「变更记录」节的模块文档检查 ID 覆盖；
                    # 早期模块文档只写架构不记 item ID 属正常，跳过避免噪声。
                    mtext = (ROOT / m).read_text(encoding="utf-8")
                    if "变更记录" in mtext and it.id not in mtext:
                        findings.append(Finding("LOW", "模块文档覆盖", m,
                                                f"未提及 {it.id}"))

    # ---- 检查 4：控制台覆盖 ----
    # 控制台只列活跃/近期 item，早期已验收 item 不出现是正常的，逐条报噪声大。
    # 因此"backlog 未在控制台出现"只统计计数，不逐条列；"控制台出现但 backlog 无"逐条列（笔误/已删值得查）。
    backlog_ids = {it.id for it in all_items}
    console_ids = set()
    console_status: dict[str, str] = {}
    backlog_not_in_console_count = 0
    if CONSOLE.is_file():
        ctext = CONSOLE.read_text(encoding="utf-8")
        console_ids = set(re.findall(ID_PATTERN, ctext))
        console_status = parse_console_status(ctext)
        backlog_not_in_console_count = len(backlog_ids - console_ids)
        for id_ in console_ids - backlog_ids:
            findings.append(Finding("MED", "控制台覆盖", id_,
                                    "控制台出现但 backlog 无此 item（笔误/已删？）"))

    # ---- 检查 5：状态值漂移（backlog verify vs 控制台启发式） ----
    for it in all_items:
        if not it.verify:
            continue
        cs = console_status.get(it.id)
        if cs is None:
            continue  # 控制台该行无明确状态词，跳过（不误报）
        # 比较 backlog 验收状态 与 控制台状态词
        if it.verify != cs:
            # 仅当两者都属"验收类"才报漂移（避免 已完成 vs 已验收 这种语义等价）
            verify_kws = {"已验收", "验收未通过", "待验收", "未验收"}
            if it.verify in verify_kws and cs in verify_kws:
                findings.append(Finding("HIGH", "状态漂移", f"{it.file}:{it.id}",
                                        f"backlog 验收状态={it.verify} vs 控制台={cs}"))

    # ---- 检查 6：控制台行规则（项目约定.md「控制台与索引行规则」）----
    if CONSOLE.is_file():
        clines = CONSOLE.read_text(encoding="utf-8").splitlines()
        for i, line in enumerate(clines, start=1):
            stripped = line.lstrip()
            if len(line) > CONSOLE_LINE_LIMIT:
                findings.append(Finding("HIGH", "行超长", f"项目控制台.md:{i}",
                                        f"单行 {len(line)} 字符 > {CONSOLE_LINE_LIMIT}——只存摘要，详情回 item 文件"))
            if "~~" in line:
                findings.append(Finding("MED", "闭合残留", f"项目控制台.md:{i}",
                                        "删除线残留（含表格行内）——闭合项应整行删除，不是划线（模块索引+CHANGELOG 已有记录）"))
        if len(clines) > CONSOLE_TOTAL_WARN:
            findings.append(Finding("MED", "体积", "项目控制台.md",
                                    f"控制台 {len(clines)} 行 > {CONSOLE_TOTAL_WARN}——只留活跃项，闭合项删除归档"))

    # ---- 检查 7：backlog 目录索引完整性 ----
    # 目录形态（<NN>-<模块>/BL-*.md）必须有兄弟索引文件，且索引表覆盖目录内全部 item
    # （2026-10-22 实事故事：06 索引被单个 item 正文整体覆盖，42 item 汇总表丢失，旧脚本无此检查）
    if BACKLOG_DIR.is_dir():
        for d in sorted(p for p in BACKLOG_DIR.iterdir() if p.is_dir()):
            dir_ids = set()
            for f in d.glob("*.md"):
                m = re.match(r"(" + ID_PATTERN + r")(?:[-.\s]|$)", f.stem)  # 词边界防 BL-06-0942-x 产出幻影 ID
                if m:
                    dir_ids.add(m.group(1))
            if not dir_ids:
                continue
            idx = BACKLOG_DIR / (d.name + ".md")
            if not idx.is_file():
                findings.append(Finding("HIGH", "索引缺失", d.name,
                                        f"目录 {d.name}/ 有 {len(dir_ids)} 个 item 但索引文件 {d.name}.md 不存在"))
                continue
            itext = idx.read_text(encoding="utf-8")
            missing = sorted(i for i in dir_ids if i not in itext)
            if missing:
                shown = ", ".join(missing[:8]) + ("…" if len(missing) > 8 else "")
                findings.append(Finding("HIGH", "索引覆盖", idx.name,
                                        f"目录 {len(dir_ids)} item，索引缺 {len(missing)} 个：{shown}——索引被覆盖或未更新"))
            if len(dir_ids) >= 5 and "状态汇总" not in itext:
                findings.append(Finding("MED", "索引结构", idx.name,
                                        "索引缺「状态汇总」节（结构被破坏或未按模板建）"))

    # ---- 检查 8：内容漂移（防总览/模块文档/索引与系统实际偏移）----
    # 行为描述是否失实无法机械判定（靠 Mode D 收尾三问 + Mode F 契约核查），
    # 这里查可机械验证的结构性漂移。哨兵行（「上次全面核对」/「合计」/模块行「已验收」）
    # 缺失本身即发现——检查存在的前提条件必须被检查。
    OVERVIEW = ROOT / "docs" / "设计总览.md"
    if OVERVIEW.is_file():
        otext = OVERVIEW.read_text(encoding="utf-8")
        for m in sorted(set(re.findall(r"modules/\S+?\.md", otext))):
            if not (ROOT / "docs" / m).is_file():
                findings.append(Finding("HIGH", "能力地图断链", "设计总览.md",
                                        f"引用的模块文档不存在：{m}"))
        mdir = ROOT / "docs" / "modules"
        if mdir.is_dir():
            for f in sorted(mdir.glob("*.md")):
                fm = re.match(r"(\d+)-", f.name)
                if fm and fm.group(1) != "00" and f.name not in otext:
                    findings.append(Finding("MED", "能力地图缺模块", f.name,
                                            "模块文档未收录于设计总览能力地图——新模块必须同步能力地图段与文档导航"))

    by_mod: dict[str, list[int]] = {}   # 模块号 → [item 总数, 已验收数]
    for it in all_items:
        mid = it.id.split("-")[1]
        by_mod.setdefault(mid, [0, 0])
        by_mod[mid][0] += 1
        if it.verify == "已验收":
            by_mod[mid][1] += 1

    if not CONSOLE.is_file():
        findings.append(Finding("HIGH", "控制台缺失", "docs/项目控制台.md",
                                "控制台文件不存在——唯一人类入口缺失；且控制台侧全部检查因此静默，属最剧烈的哨兵缺失"))
    if CONSOLE.is_file():
        saw_module_row = False
        saw_total_row = False
        for line in CONSOLE.read_text(encoding="utf-8").splitlines():
            lm = re.search(r"\(backlog/(\d+)-[^)]+\)", line)
            if lm:
                mid = lm.group(1)
                tm = re.search(r"(\d+)\s*item", line)
                am = re.search(r"(\d+)\s*已验收", line)
                if not tm:
                    findings.append(Finding("MED", "哨兵缺失", f"项目控制台.md（模块 {mid}）",
                                            "模块行缺「N item」计数格式（写成「N 项」等变体会静默关闭三项检查）——格式即契约"))
                    continue
                saw_module_row = True
                if not am:
                    findings.append(Finding("MED", "哨兵缺失", f"项目控制台.md（模块 {mid}）",
                                            "模块行缺「M 已验收」计数（格式契约：N item：…；M 已验收）——缺失即跳过计数核对"))
                    continue
                if mid in by_mod:
                    real_t, real_a = by_mod[mid]
                    if int(tm.group(1)) != real_t or int(am.group(1)) != real_a:
                        findings.append(Finding("HIGH", "计数漂移", f"项目控制台.md（模块 {mid}）",
                                                f"控制台写 {tm.group(1)} item/{am.group(1)} 已验收，backlog 实算 {real_t}/{real_a}"))
            hm = re.search(r"合计\s*(\d+)\s*item[：:]\s*(\d+)\s*已验收", line)
            if hm:
                saw_total_row = True
                gt = sum(v[0] for v in by_mod.values())
                ga = sum(v[1] for v in by_mod.values())
                if int(hm.group(1)) != gt or int(hm.group(2)) != ga:
                    findings.append(Finding("HIGH", "计数漂移", "项目控制台.md（合计）",
                                            f"控制台写 {hm.group(1)}/{hm.group(2)}，backlog 实算 {gt}/{ga}"))
        if saw_module_row and not saw_total_row:
            findings.append(Finding("MED", "哨兵缺失", "项目控制台.md",
                                    "模块总览表缺「合计 N item：M 已验收」行——合计核对的前置哨兵"))

    # ---- 检查 9：索引侧状态比对（闭合即删除控制台后，反向伪完成的机械哨位）----
    # 索引表行（| ID | 标题 | 优先级 | 状态 | 验收状态 |）与「状态汇总」计数
    # 都必须与 item 字段实算一致——8 处反向伪完成（证据闭合、字段滞后）的事后照妖镜
    if BACKLOG_DIR.is_dir():
        for f in sorted(BACKLOG_DIR.glob("*.md")):
            ilines = f.read_text(encoding="utf-8").splitlines()
            try:
                sum_idx = next(i for i, ln in enumerate(ilines) if ln.startswith("## 状态汇总"))
            except StopIteration:
                continue  # 无索引汇总节（单文件形态 item 正文在旁）不比对
            summary_lines = ilines[sum_idx + 1:]  # 汇总计数行只在「状态汇总」节之后找——防演进叙事行（如"P0 主链 12 item …已验收"）被误当汇总
            for ln in ilines:
                rm = re.match(r"\|\s*(" + ID_PATTERN + r")\s*\|[^|]*\|[^|]*\|\s*([^|]*)\|\s*([^|]*)", ln)
                if not rm:
                    continue
                it = next((x for x in all_items if x.id == rm.group(1)), None)
                if not it:
                    continue
                for cell, actual, label in ((rm.group(2), it.status, "状态"), (rm.group(3), it.verify, "验收状态")):
                    if not cell.strip():
                        findings.append(Finding("MED", "索引行未同步", f"{f.name}:{rm.group(1)}",
                                                f"表行{label}单元格为空——空即未同步，正是本检查要抓的形态"))
                        continue
                    kws = ("验收未通过", "已验收", "待验收", "未验收") if label == "验收状态" else ("已完成", "进行中", "未开始")
                    word = next((k for k in kws if k in cell), None)
                    if word and actual and word != actual:
                        findings.append(Finding("HIGH", "索引状态漂移", f"{f.name}:{rm.group(1)}",
                                                f"索引表{label}={word} vs item 字段={actual}——状态变化须同步索引行"))
            fm = re.match(r"(\d+)-", f.name)
            if fm and fm.group(1) in by_mod:
                for ln in summary_lines:
                    tm = re.search(r"(\d+)\s*item", ln)
                    am = re.search(r"(\d+)\s*已验收", ln)
                    if tm and am:
                        rt, ra = by_mod[fm.group(1)]
                        if int(tm.group(1)) != rt or int(am.group(1)) != ra:
                            findings.append(Finding("HIGH", "索引计数漂移", f.name,
                                                    f"索引汇总写 {tm.group(1)}/{am.group(1)}，item 实算 {rt}/{ra}"))
                        break

    # ---- 检查 10：活跃 item 必须在控制台在册 ----
    if CONSOLE.is_file():
        for it in all_items:
            if (it.status == "进行中" or it.verify in ("待验收", "验收未通过")) and it.id not in console_ids:
                findings.append(Finding("HIGH", "活跃缺席", it.id,
                                        f"{it.id}（{it.status}/{it.verify}）未出现在控制台——活跃 item 必须在册"))

    # ---- 检查 11：总览核对时效（含哨兵存在性）----
    for doc in [OVERVIEW, ROOT / "docs" / "当前实现总览.md"]:
        if doc.is_file():
            dm = re.search(r"上次全面核对[：:]\s*(\d{4}-\d{2}-\d{2})", doc.read_text(encoding="utf-8"))
            if not dm:
                findings.append(Finding("MED", "哨兵缺失", doc.name,
                                        "缺「上次全面核对：YYYY-MM-DD」行——时效检查的前置哨兵，缺失即失效"))
                continue
            try:
                age = (datetime.date.today() - datetime.date.fromisoformat(dm.group(1))).days
            except ValueError:
                findings.append(Finding("MED", "核对过期", doc.name,
                                        f"「上次全面核对」日期无法解析：{dm.group(1)}——修正为合法日期"))
                continue
            if age > 45:
                findings.append(Finding("LOW", "核对过期", doc.name,
                                        f"上次全面核对 {dm.group(1)}（{age} 天前）>45 天——下次接触时安排内容核对并刷新日期"))
            elif age < -1:
                findings.append(Finding("LOW", "核对过期", doc.name,
                                        f"「上次全面核对」日期在未来（{dm.group(1)}）——笔误"))

    # ---- 空跑防线：0 item = 检查失效，拒绝出绿灯 ----
    if not all_items:
        print("🛑 扫描到 0 个 backlog item——检查输入失效（docs/backlog 存在但无可解析 item），拒绝出绿灯")
        return 1

    # ---- 输出 ----
    if out_json:
        print(json.dumps({"findings": [f.__dict__ for f in findings], "scanned_items": len(all_items)},
                         ensure_ascii=False, indent=2))
        return 1 if findings else 0

    if not findings:
        print("✅ 无发现：孤儿引用 / 体积 / 模块文档覆盖 / 控制台覆盖 / 状态漂移（控制台+索引双侧）/ 控制台行规则 / 目录索引完整性 / 内容漂移（能力地图·计数·时效）/ 活跃在册 全部通过")
        print(f"   已扫描 backlog item {len(all_items)} 个（{len(list(BACKLOG_DIR.rglob('*.md')))} 个文件）——空跑=检查失效，此计数须 >0")
        return 0

    print("=== 文档一致性校验报告 ===\n")
    order = {"HIGH": 0, "MED": 1, "LOW": 2}
    groups: dict[str, list[Finding]] = {"HIGH": [], "MED": [], "LOW": []}
    for f in findings:
        groups[f.severity].append(f)
    titles = {"HIGH": "■ HIGH（须修）", "MED": "■ MEDIUM（建议修）", "LOW": "■ LOW（提示）"}
    for sev in ("HIGH", "MED", "LOW"):
        rows = groups[sev]
        if not rows:
            continue
        print(titles[sev])
        rows.sort(key=lambda x: (x.category, x.location))
        for f in rows:
            print(f"  [{sev}] {f.category} @ {f.location}")
            print(f"     {f.description}")
        print()
    print(f"共 {len(findings)} 项发现。控制台状态词为启发式解析，状态漂移请人工复核。")
    if CONSOLE.is_file():
        closed_not_in_console = sum(1 for it in all_items if it.verify == "已验收" and it.id not in console_ids)
        if closed_not_in_console:
            print(f"另：{closed_not_in_console} 个已验收闭合 item 不在控制台（闭合即归档，属正常）。")
    return 1


if __name__ == "__main__":
    sys.exit(main())