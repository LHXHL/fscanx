# -*- coding: utf-8 -*-
"""调试：自立成功后弹窗现场 dump（分支切换是否发生/el-select-v2 存在性/verdict 状态）"""
import json
import urllib.request

from playwright.sync_api import sync_playwright

BASE = "http://127.0.0.1:7758"
TEST_STD = "调试产品D"


def cleanup():
    tok = json.loads(urllib.request.urlopen(urllib.request.Request(
        BASE + "/api/user/login",
        data=json.dumps({"username": "admin", "password": "admin123"}).encode(),
        headers={"Content-Type": "application/json"})).read())["data"]["accessToken"]
    req = urllib.request.Request(BASE + "/api/catalog_delete",
                                 data=json.dumps({"name": TEST_STD}).encode("utf-8"),
                                 headers={"Content-Type": "application/json", "Authorization": tok}, method="POST")
    try:
        print("cleanup code:", json.loads(urllib.request.urlopen(req).read())["code"])
    except Exception as e:
        print("cleanup:", e)


def main():
    with sync_playwright() as p:
        browser = p.chromium.launch(headless=True)
        page = browser.new_page()
        page.goto(BASE, wait_until="networkidle")
        page.wait_for_selector('input[type="password"]', timeout=15000)
        page.fill('input[type="text"]', "admin")
        page.fill('input[type="password"]', "admin123")
        page.keyboard.press("Enter")
        page.wait_for_selector('input[placeholder*="搜索"]', timeout=15000)
        page.goto(BASE + "/#/finger/edit", wait_until="networkidle")
        page.wait_for_timeout(2500)

        row = page.locator('tbody tr:has-text("未归属")').first
        name_a = row.locator("td").nth(1).inner_text().strip()
        print("row A:", name_a)
        row.locator('button:has-text("归属")').click()
        page.wait_for_timeout(1200)
        dlg = page.locator(".el-dialog:visible")
        print("=== 弹窗打开 ===")
        print(dlg.inner_text()[:400])
        print("el-select-v2 count:", page.locator(".el-dialog .el-select-v2").count())

        page.fill('.el-dialog:visible input[placeholder*="新标准产品"]', TEST_STD)
        page.click('.el-dialog button:has-text("自立")')
        page.wait_for_timeout(2500)
        print("=== 自立后 ===")
        d2 = page.locator(".el-dialog:visible")
        print("dialog visible:", d2.count())
        if d2.count():
            print(d2.inner_text()[:500])
            print("el-select-v2 count:", page.locator(".el-dialog .el-select-v2").count())
        msgs = page.locator(".el-message").all_inner_texts()
        print("toasts:", msgs)
        browser.close()


if __name__ == "__main__":
    try:
        main()
    finally:
        cleanup()
