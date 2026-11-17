# -*- coding: utf-8 -*-
"""BL-POC-009 Phase 6：产物版 headless 三视角交互链验证（零 console 错误口径）。
交互模式：点击后校验效果 + 补点（Element Plus headless 点击竞态，交接坑 3）。
产出 plans/scripts/three_views_result.txt
"""
import json

from playwright.sync_api import sync_playwright

BASE = "http://127.0.0.1:7758"
OUT = r"E:\code\go\fscanx\plans\scripts\three_views_result.txt"

lines = []
console_errors = []


def click_until(page, click_sel, check, tries=3, delay=700):
    """点击 + 效果校验补点；check=callable()->bool，成功即返回 True"""
    for _ in range(tries):
        try:
            page.click(click_sel, timeout=4000)
        except Exception:
            pass
        page.wait_for_timeout(delay)
        if check():
            return True
    return False


def close_dialog(page):
    """关弹窗：Esc 兜底 + 取消按钮补点"""
    page.keyboard.press("Escape")
    page.wait_for_timeout(400)
    for _ in range(2):
        if not page.is_visible(".el-dialog"):
            return
        try:
            page.click('.el-dialog button:has-text("取消")', timeout=3000)
        except Exception:
            pass
        page.wait_for_timeout(500)


def dlg_text(page):
    loc = page.locator(".el-dialog:visible")
    return loc.inner_text() if loc.count() else ""


def main():
    with sync_playwright() as p:
        browser = p.chromium.launch(headless=True)
        page = browser.new_page()
        page.on("console", lambda m: console_errors.append(m.text) if m.type == "error" else None)
        page.on("pageerror", lambda e: console_errors.append(str(e)))

        # ---- 登录（登录页首个 input 是暗色模式开关，必须用 password 框定位）----
        page.goto(BASE, wait_until="networkidle")
        page.wait_for_selector('input[type="password"]', timeout=15000)
        page.fill('input[type="text"]', "admin")
        page.fill('input[type="password"]', "admin123")
        page.keyboard.press("Enter")
        page.wait_for_selector('input[placeholder*="搜索"]', timeout=15000)
        lines.append(f"[login] 登录跳转完成 url={page.url}")

        # ---- 标准产品页 ----
        page.goto(BASE + "/#/catalog/list", wait_until="networkidle")
        page.wait_for_timeout(2000)
        body = page.inner_text("body")
        # 50 条按名排序分页 20/页——首页是英文名条目（CTOP OA 首行），中文条目在第二页
        ok_list = "CTOP OA" in body and "标准产品（" in body
        lines.append(f"[catalog] 列表渲染（首页出厂条目可见）：{'PASS' if ok_list else 'FAIL'}")

        # 新建弹窗（强制挂别名交互：候选辅助 + 别名区）
        ok_add = click_until(page, 'button:has-text("新建标准产品")',
                             lambda: page.is_visible(".el-dialog") and "候选辅助" in dlg_text(page))
        lines.append(f"[catalog] 新建弹窗（标准名/别名/候选辅助）：{'PASS' if ok_add else 'FAIL'}")
        close_dialog(page)

        # 编辑弹窗（出厂条目双目标 radio）——泛微中文条目按名排序在第二页，先搜索过滤
        page.fill('input[placeholder*="搜索"]', "泛微")
        page.wait_for_timeout(800)
        body2 = page.inner_text("body")
        ok_search = "泛微 e-cology" in body2 and "coremail" not in body2
        lines.append(f"[catalog] 搜索过滤（泛微）：{'PASS' if ok_search else 'FAIL'}")
        ok_edit = click_until(page, 'tr:has-text("泛微 e-cology") button:has-text("编辑")',
                              lambda: page.is_visible(".el-dialog") and "保存位置" in dlg_text(page)
                              and "代码源文件" in dlg_text(page) and "外挂文件" in dlg_text(page))
        lines.append(f"[catalog] 编辑弹窗双位置（代码源文件/外挂文件）：{'PASS' if ok_edit else 'FAIL'}")
        close_dialog(page)

        # ---- 指纹页（产品归属列）----
        page.goto(BASE + "/#/finger/edit", wait_until="networkidle")
        page.wait_for_timeout(2500)
        body3 = page.inner_text("body")
        has_cols = "产品归属" in body3
        has_state = ("标准产品" in body3) or ("未归属" in body3) or ("的别名" in body3)
        lines.append(f"[finger] 产品归属列渲染：{'PASS' if has_cols else 'FAIL'}；三态之一可见：{'PASS' if has_state else 'FAIL'}")
        lines.append(f"[finger] 所属产品测试列：{'PASS' if '所属产品测试' in body3 else 'FAIL'}")

        # ---- POC 页（绑定标准产品弹窗）----
        page.goto(BASE + "/#/poc/list", wait_until="networkidle")
        page.wait_for_timeout(2500)
        # tbody 首行（tr:first 是表头行，点它不选中 POC、绑定按钮保持 disabled）
        page.locator("tbody tr").first.click()
        page.wait_for_timeout(1000)
        ok_bind = click_until(page, 'button:has-text("绑定产品")',
                              lambda: page.is_visible(".el-dialog") and "已绑产品" in dlg_text(page)
                              and "新增绑定" in dlg_text(page))
        lines.append(f"[poc] 绑定标准产品弹窗：{'PASS' if ok_bind else 'FAIL'}")
        # 下拉点开——候选应为标准产品（el-select-v2 虚拟列表只渲染首屏可见项：
        # 按名序首屏是 CTOP OA/coremail/discuz 等英文名条目，泛微在滚动后段）
        def _first_screen_opts():
            sel = ".el-select-dropdown:visible, .el-select-v2__popper:visible"
            if not page.locator(sel).count():
                return False
            txt = page.locator(sel).first.inner_text()
            return any(k in txt for k in ("CTOP OA", "coremail", "discuz", "LemonOA", "PHPOA"))
        ok_opts = click_until(page, ".el-dialog .el-select-v2, .el-dialog .el-select", _first_screen_opts)
        lines.append(f"[poc] 候选=标准产品：{'PASS' if ok_opts else 'FAIL（popper 空或无标准产品名）'}")
        page.keyboard.press("Escape")
        page.wait_for_timeout(400)
        close_dialog(page)

        browser.close()

    errs = [e for e in console_errors if "favicon" not in e.lower()]
    lines.append("")
    lines.append(f"[console] 错误数（除 favicon）：{len(errs)}")
    for e in errs[:10]:
        lines.append("  ERR: " + e[:200])
    fails = [l for l in lines if l.startswith("[") and "FAIL" in l]
    lines.append("")
    lines.append("RESULT: " + ("ALL PASS" if not fails and not errs else f"HAS FAIL x{len(fails)+len(errs)}"))

    with open(OUT, "w", encoding="utf-8") as f:
        f.write("\n".join(lines))
    print("written")


if __name__ == "__main__":
    main()
