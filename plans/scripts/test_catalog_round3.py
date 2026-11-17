# -*- coding: utf-8 -*-
"""BL-POC-009 验收 API 实测第三轮（修正版）：
[3] 外置别名注入 resolve——先加含别名条目再判定（前轮脚本漏了加条目，测了个不存在的别名）
[4b] 自标准名放行例外——用指纹库真实名（前轮用假名字，拒绝是正确行为）
产出 plans/scripts/catalog_api_result3.txt
"""
import json
import urllib.request

BASE = "http://127.0.0.1:7758"
REPO = r"E:\code\go\fscanx"
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

    # ---- [3] 外置别名注入 resolve ----
    ra = api("/api/catalog_add", {"entry": {"name": "验收外置产品Q", "alias": {"goby": ["验收实测别名Y"]}, "pocs": []}})
    lines.append(f"[3a] 外置条目新建：code={ra['code']}")
    rv = api("/api/catalog_resolve", {"items": [{"source": "goby", "name": "验收实测别名Y"}]})["data"]["verdicts"][0]
    ok3 = rv["relation"] == "alias-of" and rv["owner"].lower() == "验收外置产品q"
    lines.append(f"[3b] 外置别名注入 resolve：relation={rv['relation']} owner={rv['owner']} {'PASS' if ok3 else 'FAIL'}")
    rd = api("/api/catalog_delete", {"name": "验收外置产品Q"})
    lines.append(f"[3c] 外置条目清理：code={rd['code']}")

    # ---- [4b] 自标准名放行例外（指纹库真实名 + 空别名 → 允许）----
    r4b = api("/api/catalog_add", {"entry": {"name": "Nginx", "alias": {}, "pocs": []}})
    ok4b = r4b["code"] == 0
    lines.append(f"[4b] 指纹全集名空别名放行：code={r4b['code']} {'PASS' if ok4b else 'FAIL msg=' + r4b.get('message', '')[:60]}")
    if ok4b:
        api("/api/catalog_delete", {"name": "Nginx"})
        lines.append("[4c] 放行条目已清理")

    with open(REPO + r"\plans\scripts\catalog_api_result3.txt", "w", encoding="utf-8") as f:
        f.write("\n".join(lines))
    print("written")


if __name__ == "__main__":
    main()
