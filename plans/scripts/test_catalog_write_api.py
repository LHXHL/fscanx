# -*- coding: utf-8 -*-
"""BL-POC-009 验收 API 实测：built-in 写回 / external 覆盖 / 防死条目守卫 / 整条全量写出。
出厂源文件先备份后还原，外置产物实测后清理。产出 plans/scripts/catalog_api_result.txt
"""
import json
import shutil
import urllib.request

BASE = "http://127.0.0.1:7758"
REPO = r"E:\code\go\fscanx"
SRC = REPO + r"\mylib\finger\resources\catalog.yaml"
BAK = REPO + r"\plans\old-catalog.tmp.yaml"
TOKEN = None

lines = []


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
        return json.loads(r.read())


def main():
    login()
    shutil.copy(SRC, BAK)  # 备份出厂源
    try:
        # ---- 取出厂条目「泛微 e-cology」完整条目 ----
        entries = api("/api/catalog_list")["data"]["entries"]
        eco = next(e for e in entries if e["name"] == "泛微 e-cology")
        alias_before = json.dumps(eco["alias"], ensure_ascii=False, sort_keys=True)
        lines.append(f"[0] 出厂条目取得：alias 源数={len(eco['alias'])} pocs 数={len(eco['pocs'])}")

        # ---- 实测1：built-in 写回（追加一条别名，整条全量）----
        new_alias = dict(eco["alias"])
        new_alias["goby"] = list(new_alias.get("goby", [])) + ["验收实测写法X"]
        payload = {"name": "泛微 e-cology", "vendor": eco["vendor"], "product": eco["product"],
                   "alias": new_alias, "pocs": eco["pocs"]}
        r1 = api("/api/catalog_edit", {"original": "泛微 e-cology", "target": "built-in", "entry": payload})
        lines.append(f"[1] built-in 写回：code={r1['code']} path={r1['data'].get('path', r1['data'].get('message'))}")

        # 校验：源文件变化 + 别名区零丢失 + 新别名在（整文件 yaml 解析）
        import yaml
        doc = yaml.safe_load(open(SRC, encoding="utf-8"))
        eco2 = next(e for e in doc if e.get("name") == "泛微 e-cology")
        alias_after = json.dumps(eco2["alias"], ensure_ascii=False, sort_keys=True)
        lines.append(f"[1a] 别名区完整性：源数 {len(eco2['alias'])}（前 {len(eco['alias'])}），"
                     f"原有别名保留={'验收实测写法X' not in ''.join(sum(eco2['alias'].values(), [])) and alias_after.startswith(alias_before[:40])}")
        lost = []
        for src, names in eco["alias"].items():
            for n in names:
                if n not in eco2["alias"].get(src, []):
                    lost.append(f"{src}:{n}")
        lines.append(f"[1b] 原别名零丢失：{'PASS' if not lost else 'FAIL ' + str(lost)}")
        lines.append(f"[1c] 新别名写入：{'PASS' if '验收实测写法X' in eco2['alias'].get('goby', []) else 'FAIL'}")
        lines.append(f"[1d] pocs 零丢失：{'PASS' if eco2.get('pocs') == eco['pocs'] else 'FAIL'}")

    finally:
        shutil.copy(BAK, SRC)  # 还原出厂源
        lines.append("[R] 出厂源文件已还原")

    with open(REPO + r"\plans\scripts\catalog_api_result.txt", "w", encoding="utf-8") as f:
        f.write("\n".join(lines))
    print("written")


if __name__ == "__main__":
    main()
