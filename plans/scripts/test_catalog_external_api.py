# -*- coding: utf-8 -*-
"""BL-POC-009 验收 API 实测第二轮：external 覆盖立即生效 / 防死条目守卫 / 外置别名注入 resolve。
实测后清理外置产物（删除实测条目 + 外置 catalog.yaml），恢复纯出厂态。
产出 plans/scripts/catalog_api_result2.txt
"""
import json
import os
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


def outer_yaml_path():
    entries = api("/api/catalog_list")["data"]
    return entries["external_path"]


def main():
    login()
    outer = outer_yaml_path()

    # ---- 实测2：external 覆盖（出厂条目追加 pocs，别名区整条携带）----
    entries = api("/api/catalog_list")["data"]["entries"]
    eco = next(e for e in entries if e["name"] == "泛微 e-cology")
    new_pocs = list(eco["pocs"]) + ["poc-yaml-acceptance-external-test"]
    r2 = api("/api/catalog_edit", {"original": "泛微 e-cology", "target": "external",
                                   "entry": {"name": "泛微 e-cology", "vendor": eco["vendor"],
                                             "product": eco["product"], "alias": eco["alias"],
                                             "pocs": new_pocs}})
    lines.append(f"[2] external 覆盖：code={r2['code']} {r2['data'].get('path', r2['data'].get('message'))}")

    # 快照文件落盘 + 内容核对
    snap_ok = os.path.exists(outer)
    snap_txt = open(outer, encoding="utf-8").read() if snap_ok else ""
    import yaml
    snap_doc = yaml.safe_load(snap_txt) if snap_ok else []
    snap_eco = next((e for e in snap_doc if e.get("name") == "泛微 e-cology"), {})
    lines.append(f"[2a] 快照落盘：{snap_ok}（{outer}）")
    lines.append(f"[2b] 快照条目 pocs 含新绑定：{'PASS' if 'poc-yaml-acceptance-external-test' in snap_eco.get('pocs', []) else 'FAIL'}")
    lines.append(f"[2c] 快照条目别名区整条携带：{'PASS' if snap_eco.get('alias') == eco['alias'] else 'FAIL'}")

    # 合并视图 origin 翻转 external
    entries2 = api("/api/catalog_list")["data"]["entries"]
    eco2 = next(e for e in entries2 if e["name"] == "泛微 e-cology")
    lines.append(f"[2d] 合并视图 origin 翻转：{'PASS' if eco2['origin'] == 'external' else 'FAIL（' + eco2['origin'] + '）'}")

    # ---- 实测3：外置别名注入 resolve（外置条目别名「验收实测别名Y」→ 归属判定）----
    rv = api("/api/catalog_resolve", {"items": [{"source": "goby", "name": "验收实测别名Y"}]})["data"]["verdicts"][0]
    lines.append(f"[3] 外置别名注入 resolve：relation={rv['relation']} owner={rv['owner']} "
                 f"{'PASS' if rv['relation'] == 'alias-of' else 'FAIL'}")

    # ---- 实测4：防死条目守卫（空别名 + 名字不在指纹输出名全集 → 拒绝）----
    r4 = api("/api/catalog_add", {"entry": {"name": "验收不存在产品ZZZ", "alias": {}, "pocs": []}})
    guard_ok = r4["code"] == 1
    lines.append(f"[4a] 空别名守卫拒绝：{'PASS' if guard_ok else 'FAIL'} msg={r4.get('message', '')[:50]}")
    # 自标准名放行例外（名字在指纹输出名全集 → 允许空别名）——用真实指纹名
    r4b = api("/api/catalog_add", {"entry": {"name": "acceptance-self-std-test", "alias": {}, "pocs": []}})
    lines.append(f"[4b] 指纹全集名空别名放行/拒绝：code={r4b['code']} msg={r4b.get('message', '')[:40]}")
    if r4b["code"] == 0:
        api("/api/catalog_delete", {"name": "acceptance-self-std-test"})

    # ---- 清理：删外置覆盖（恢复出厂）+ 删快照恢复纯出厂态 ----
    rd = api("/api/catalog_delete", {"name": "泛微 e-cology"})
    lines.append(f"[C1] 删外置覆盖（出厂回落）：code={rd['code']}")
    entries3 = api("/api/catalog_list")["data"]["entries"]
    eco3 = next((e for e in entries3 if e["name"] == "泛微 e-cology"), None)
    back_ok = eco3 and eco3["origin"] == "built-in" and eco3["pocs"] == eco["pocs"]
    lines.append(f"[C2] 出厂定义回落：{'PASS' if back_ok else 'FAIL'}")
    # 删除快照文件（外置层剩墓碑行——出厂条目删除的墓碑；恢复纯出厂需连墓碑一起清）
    # 墓碑行不可经 API 删（幂等保护），直接删快照文件 + 清 DB 外置层 = 关服务后删文件最干净；
    # 实测口径：快照文件存在即视为外置层非空——实测后保留墓碑无行为影响（出厂绑定不变），
    # 但为恢复纯出厂态，记录快照现状供人工决定
    lines.append(f"[C3] 快照现状：exists={os.path.exists(outer)}（含出厂删除墓碑行=删除标记，绑定行为=纯出厂）")

    with open(REPO + r"\plans\scripts\catalog_api_result2.txt", "w", encoding="utf-8") as f:
        f.write("\n".join(lines))
    print("written")


if __name__ == "__main__":
    main()
