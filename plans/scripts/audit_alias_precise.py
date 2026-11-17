# -*- coding: utf-8 -*-
"""漏收精筛（复核口径）：unassigned 写法与出厂【绑定条目】(has_pocs) 别名区写法做 normalize
双向子串匹配——只报与已绑定产品实质相关的写法（如"致远OA-A8"⊃"致远OA"），
排除出厂册根本没收录的产品（gitlab/jenkins 等正常自标准名）。
产出：plans/scripts/alias_suspects_precise.txt
"""
import json
import re
import urllib.request

BASE = "http://127.0.0.1:7758"
REPO = r"E:\code\go\fscanx"
TOKEN = None

SOURCES = {
    "goby": REPO + r"\mylib\finger\resources\goby.json\goby.json",
    "fingers": REPO + r"\mylib\finger\resources\fingers_http.json\fingers_http.json",
}


def normalize(s: str) -> str:
    """近似 resources.NormalizeString 口径（lower+去空白/分隔符）——包含关系判定对
    精确实现细节不敏感（子串关系在两种口径下方向一致）。"""
    return re.sub(r"[\s\-_/·・()（）]+", "", s.lower())


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


def api(path, payload=None):
    data = json.dumps(payload).encode("utf-8") if payload is not None else None
    req = urllib.request.Request(
        BASE + path, data=data,
        headers={"Content-Type": "application/json", "Authorization": TOKEN},
        method="POST" if data else "GET",
    )
    with urllib.request.urlopen(req) as r:
        return json.loads(r.read())["data"]


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
    cat = api("/api/catalog_list")
    bound = [e for e in cat["entries"] if e["has_pocs"]]
    # 绑定条目的别名写法全集（含标准名自身），normalize 键
    alias_keys = []  # (entry_name, normalized_alias, raw_alias)
    for e in bound:
        keys = {e["name"]}
        for names in (e["alias"] or {}).values():
            keys.update(names)
        for k in keys:
            alias_keys.append((e["name"], normalize(k), k))

    unassigned_all = []
    for src, path in SOURCES.items():
        names = load_names(path)
        for i in range(0, len(names), 500):
            batch = [{"source": src, "name": n} for n in names[i : i + 500]]
            for v in api("/api/catalog_resolve", {"items": batch})["verdicts"]:
                if v["relation"] == "unassigned":
                    unassigned_all.append((src, v["name"]))

    lines = [f"绑定条目 {len(bound)} 个；unassigned 全量 {len(unassigned_all)}；精筛：", ""]
    hits = []
    for src, n in unassigned_all:
        nn = normalize(n)
        for entry, ak, raw in alias_keys:
            if len(ak) < 2:
                continue
            if ak in nn or (len(nn) >= 2 and nn in ak):
                hits.append((src, n, entry, raw))
                break
    lines.append(f"=== 疑似漏收（与绑定条目别名存在包含关系，{len(hits)} 条）===")
    for src, n, entry, raw in sorted(hits):
        lines.append(f"  [{src}] {n!r}  ⊂ {entry}（别名 {raw!r}）")

    report = "\n".join(lines)
    print(report[:1500])
    with open(REPO + r"\plans\scripts\alias_suspects_precise.txt", "w", encoding="utf-8") as f:
        f.write(report)


if __name__ == "__main__":
    main()
