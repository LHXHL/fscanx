# -*- coding: utf-8 -*-
"""catalog_resolve 编码排除验证（Phase 6）：Python requests 以 UTF-8 发请求，排除 Git Bash curl 中文编码损坏变量。"""
import json
import urllib.request

TOKEN = "597bdb7ee90fa0a4ca6850557a013ca3d35781a8d84751667f43a95c32e19842"

items = [
    {"source": "fingers", "name": "泛微云桥 e-Bridge"},
    {"source": "goby", "name": "Weaver OA"},
    {"source": "fingers", "name": "致远OA"},
    {"source": "goby", "name": "Nginx"},
    {"source": "fingers", "name": "seeyon"},
    {"source": "fingers", "name": "用友 Seeyon OA"},
    {"source": "goby", "name": "泛微-移动办公OA"},  # 已清理的失效别名——预期 unassigned
]

req = urllib.request.Request(
    "http://127.0.0.1:7758/api/catalog_resolve",
    data=json.dumps({"items": items}).encode("utf-8"),
    headers={"Content-Type": "application/json", "Authorization": TOKEN},
    method="POST",
)
with urllib.request.urlopen(req) as resp:
    data = json.loads(resp.read().decode("utf-8"))["data"]

out = []
for v in data["verdicts"]:
    out.append(f"{v['source']:10s} {v['name']:20s} -> {v['relation']:10s} {v['owner']}")
report = "\n".join(out)
with open(r"E:\code\go\fscanx\plans\scripts\resolve_result.txt", "w", encoding="utf-8") as f:
    f.write(report)
print("written")
