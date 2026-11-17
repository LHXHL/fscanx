# -*- coding: utf-8 -*-
"""别名完整性全量实测（BL-POC-009 验收条件③）：两指纹明文全集逐写法过 catalog_resolve，
统计三态分布；unassigned 清单按出厂绑定条目关键词筛"疑似漏收"（人工裁决候选）。
产出：plans/scripts/alias_coverage_report.txt
"""
import json
import urllib.request

TOKEN = None
BASE = "http://127.0.0.1:7758"
REPO = r"E:\code\go\fscanx"

SOURCES = {
    "goby": REPO + r"\mylib\finger\resources\goby.json\goby.json",
    "fingers": REPO + r"\mylib\finger\resources\fingers_http.json\fingers_http.json",
}

# 出厂绑定条目（catalogSeed 投影=有 pocs 的条目）与其别名核心词（漏收筛查用）
BIND_KEYWORDS = [
    "泛微", "weaver", "ecology", "e-office", "eoffice", "e-bridge", "ebridge",
    "致远", "seeyon", "zhiyuan",
    "通达", "tongda", "oa",
    "蓝凌", "landray",
    "用友", "yongyou", "u8", "nc", "grp",
    "齐治", "qizhi",
    "coremail", "discuz", "phpoa", "ibos", "ctop", "topoffice",
    "weblogic", "thinkphp", "jboss", "zabbix", "nexus", "gitlab", "jenkins",
    "kingsoft", "vmware", "vcenter", "esc", "深信服", "sangfor",
    "nacos", "druid", "shiro", "若依", "ruoyi", "宝塔", "bt",
]


def login():
    global TOKEN
    req = urllib.request.Request(
        BASE + "/api/user/login",
        data=json.dumps({"username": "admin", "password": "admin123"}).encode(),
        headers={"Content-Type": "application/json"},
        method="POST",
    )
    with urllib.request.urlopen(req) as r:
        TOKEN = json.loads(r.read())["data"]["accessToken"]


def resolve(items):
    req = urllib.request.Request(
        BASE + "/api/catalog_resolve",
        data=json.dumps({"items": items}).encode("utf-8"),
        headers={"Content-Type": "application/json", "Authorization": TOKEN},
        method="POST",
    )
    with urllib.request.urlopen(req) as r:
        return json.loads(r.read())["data"]["verdicts"]


def load_names(path):
    with open(path, encoding="utf-8") as f:
        arr = json.load(f)
    seen, out = set(), []
    for m in arr:
        n = m.get("name")
        if n and n not in seen:
            seen.add(n)
            out.append(n)
    return out


def main():
    login()
    stats = {}
    unassigned_all = []
    for src, path in SOURCES.items():
        names = load_names(path)
        verdicts = []
        for i in range(0, len(names), 500):
            batch = [{"source": src, "name": n} for n in names[i : i + 500]]
            verdicts.extend(resolve(batch))
        c = {"alias-of": 0, "self": 0, "unassigned": 0}
        for v in verdicts:
            c[v["relation"]] += 1
            if v["relation"] == "unassigned":
                unassigned_all.append((src, v["name"]))
        stats[src] = {"total": len(names), **c}

    # 漏收候选：unassigned 写法含出厂绑定条目关键词
    suspects = [(s, n) for (s, n) in unassigned_all if any(k in n.lower() for k in BIND_KEYWORDS)]

    lines = []
    lines.append("=== 两指纹明文全集归属分布（验收条件③实测） ===")
    for src, c in stats.items():
        lines.append(f"[{src}] total={c['total']} alias-of={c['alias-of']} self={c['self']} unassigned={c['unassigned']}")
    lines.append("")
    lines.append(f"=== 疑似漏收候选（unassigned 且含出厂绑定条目关键词，{len(suspects)} 条）===")
    for s, n in sorted(suspects):
        lines.append(f"  [{s}] {n}")

    report = "\n".join(lines)
    with open(REPO + r"\plans\scripts\alias_coverage_report.txt", "w", encoding="utf-8") as f:
        f.write(report)
    print("report written, suspects:", len(suspects))


if __name__ == "__main__":
    main()
