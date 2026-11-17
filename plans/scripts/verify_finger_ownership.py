# -*- coding: utf-8 -*-
"""指纹视角归属操作链 headless 验证（BL-POC-009 方案承诺补齐：指定归属/自立标准产品/顺手绑定）。
全程只碰自建测试条目（验收测试产品T），零真实污染；产出 plans/scripts/finger_ownership_result.txt
"""
from playwright.sync_api import sync_playwright

BASE = "http://127.0.0.1:7758"
OUT = r"E:\code\go\fscanx\plans\scripts\finger_ownership_result.txt"
TEST_STD = "验收测试产品T"

lines = []
console_errors = []


def cleanup_test_std():
    """自立前幂等清同名残留（前轮中断留下的条目会让自立被"已是标准产品"拒绝，分支不切）"""
    try:
        tok = json.loads(urllib.request.urlopen(urllib.request.Request(
            BASE + "/api/user/login",
            data=json.dumps({"username": "admin", "password": "admin123"}).encode(),
            headers={"Content-Type": "application/json"})).read())["data"]["accessToken"]
        req = urllib.request.Request(BASE + "/api/catalog_delete",
                                     data=json.dumps({"name": TEST_STD}).encode("utf-8"),
                                     headers={"Content-Type": "application/json", "Authorization": tok}, method="POST")
        json.loads(urllib.request.urlopen(req).read())
    except Exception:
        pass  # 不存在=正常态


def click_until(page, click_sel, check, tries=3, delay=700):
    for _ in range(tries):
        try:
            page.click(click_sel, timeout=4000)
        except Exception:
            pass
        page.wait_for_timeout(delay)
        if check():
            return True
    return False


def dlg_text(page):
    loc = page.locator(".el-dialog:visible")
    return loc.inner_text() if loc.count() else ""


def select_v2_pick(page, trigger_sel, keyword, popper_sel=".el-select-dropdown:visible, .el-select-v2__popper:visible"):
    """el-select-v2 headless 选择：点开 → 输入过滤 → 点首个选项
    注意：el-select-v2 渲染的 DOM 根类是 .el-select（Element Plus 共享样式类），
    选择器必须用复合形态（调试实证：.el-select-v2 单选 count=0）"""
    page.click(trigger_sel)
    page.wait_for_timeout(600)
    page.keyboard.type(keyword, delay=50)
    page.wait_for_timeout(900)
    opt = page.locator(f"{popper_sel} .el-select-dropdown__item").first
    if not opt.count():
        return False
    opt.click()
    page.wait_for_timeout(400)
    return True


def main():
    cleanup_test_std()
    try:
        run()
    except Exception as e:
        lines.append(f"[EXC] {type(e).__name__}: {str(e)[:150]}")
        lines.append("RESULT: EXC")
        with open(OUT, "w", encoding="utf-8") as f:
            f.write(chr(10).join(lines))
        raise


def run():
    with sync_playwright() as p:
        browser = p.chromium.launch(headless=True)
        page = browser.new_page()
        page.on("console", lambda m: console_errors.append(m.text) if m.type == "error" else None)
        page.on("pageerror", lambda e: console_errors.append(str(e)))

        # 登录
        page.goto(BASE, wait_until="networkidle")
        page.wait_for_selector('input[type="password"]', timeout=15000)
        page.fill('input[type="text"]', "admin")
        page.fill('input[type="password"]', "admin123")
        page.keyboard.press("Enter")
        page.wait_for_selector('input[placeholder*="搜索"]', timeout=15000)

        # 指纹页
        page.goto(BASE + "/#/finger/edit", wait_until="networkidle")
        page.wait_for_timeout(2500)

        # 动态取前两个"未归属"行
        unassigned_rows = page.locator('tbody tr:has-text("未归属")')
        if unassigned_rows.count() < 2:
            lines.append("[rows] FAIL：未归属行不足 2（出厂归属覆盖变化？）")
            browser.close()
            with open(OUT, "w", encoding="utf-8") as f:
                f.write("\n".join(lines))
            return
        row_a = unassigned_rows.nth(0)
        row_b = unassigned_rows.nth(1)
        name_a = row_a.locator("td").nth(1).inner_text().strip()
        name_b = row_b.locator("td").nth(1).inner_text().strip()
        lines.append(f"[rows] 未归属行 A={name_a!r} B={name_b!r}")

        # ---- 链 1：行 A 归属弹窗（未归属分支）→ 自立标准产品 → 分支切换 ----
        ok_open = click_until(page, f'tbody tr:has-text("{name_a}") button:has-text("归属")',
                              lambda: page.is_visible(".el-dialog") and "当前写法" in dlg_text(page))
        lines.append(f"[open] 归属弹窗打开：{'PASS' if ok_open else 'FAIL'}")
        # 两步式初始态：仅"归属方式"选择可见，两套配置区均未展开（避免平铺）
        ok_unassigned = ("未归属" in dlg_text(page) and "归属方式" in dlg_text(page)
                         and "目标产品" not in dlg_text(page) and "标准名" not in dlg_text(page))
        lines.append(f"[branch] 未归属初始态（仅归属方式选择，配置不平铺）：{'PASS' if ok_unassigned else 'FAIL'}")

        # 两步式：先选"自立为新标准产品"方式（配置区按需展开），再填标准名
        ok_mode = click_until(page, '.el-dialog .el-radio-button:has-text("自立为新标准产品")',
                              lambda: page.is_visible('.el-dialog input[placeholder*="新标准产品"]'))
        lines.append(f"[mode] 选择归属方式（自立）：{'PASS' if ok_mode else 'FAIL'}")
        page.fill('.el-dialog:visible input[placeholder*="新标准产品"]', TEST_STD)
        page.wait_for_timeout(300)
        ok_self = click_until(page, '.el-dialog button:has-text("自立")',
                              lambda: "所属产品测试" in dlg_text(page) and "追加绑定" in dlg_text(page))
        lines.append(f"[self-std] 自立「{TEST_STD}」→ 已归属分支切换：{'PASS' if ok_self else 'FAIL'}")

        # 绑定：POC 下拉选 coremail → 绑定 → 测试名单 tag 出现
        ok_pick = select_v2_pick(page, '.el-dialog .el-select, .el-dialog .el-select-v2', "coremail")
        ok_bind = False
        if ok_pick:
            ok_bind = click_until(page, '.el-dialog button:has-text("绑定")',
                                  lambda: "poc-yaml-coremail" in dlg_text(page))
        lines.append(f"[bind] 顺手绑定 coremail poc：{'PASS' if ok_bind else 'FAIL'}")
        page.keyboard.press("Escape")
        page.wait_for_timeout(400)
        page.click('.el-dialog button:has-text("关闭")') if page.is_visible('.el-dialog button:has-text("关闭")') else None
        page.wait_for_timeout(500)

        # ---- 链 2：行 B 挂为「验收测试产品T」的别名（确认框 → 确认）----
        ok_open2 = click_until(page, f'tbody tr:has-text("{name_b}") button:has-text("归属")',
                               lambda: page.is_visible(".el-dialog") and "当前写法" in dlg_text(page))
        ok_pick2 = False
        ok_assign = False
        if ok_open2:
            # 两步式：先选"挂为已有产品的别名"方式，配置区展开后再选目标产品
            ok_mode2 = click_until(page, '.el-dialog .el-radio-button:has-text("挂为已有产品的别名")',
                                   lambda: page.is_visible('.el-dialog .el-select, .el-dialog .el-select-v2'))
            lines.append(f"[mode] 选择归属方式（挂别名）：{'PASS' if ok_mode2 else 'FAIL'}")
            ok_pick2 = ok_mode2 and select_v2_pick(page, '.el-dialog .el-select, .el-dialog .el-select-v2', TEST_STD)
            if ok_pick2:
                page.click('.el-dialog button:has-text("挂为别名")')
                page.wait_for_timeout(800)
                # ElMessageBox 确认（铁律 5 文案）
                box = page.locator(".el-message-box:visible")
                ok_confirm_text = "归一显示" in box.inner_text() if box.count() else False
                lines.append(f"[assign] 确认框映射改显示名文案（铁律5）：{'PASS' if ok_confirm_text else 'FAIL'}")
                if ok_confirm_text:
                    box.locator('button:has-text("确定")').click()
                    page.wait_for_timeout(1200)
                    ok_assign = page.locator(f'tbody tr:has-text("{name_b}")').first.inner_text().find("未归属") == -1
        lines.append(f"[assign] 行 B 挂为「{TEST_STD}」别名（归属列刷新）：{'PASS' if ok_pick2 and ok_assign else 'FAIL'}")

        browser.close()

    errs = [e for e in console_errors if "favicon" not in e.lower()]
    lines.append("")
    lines.append(f"[console] 错误数（除 favicon）：{len(errs)}")
    for e in errs[:8]:
        lines.append("  ERR: " + e[:180])
    fails = [l for l in lines if l.startswith("[") and "FAIL" in l]
    lines.append("")
    lines.append("RESULT: " + ("ALL PASS" if not fails and not errs else f"HAS FAIL x{len(fails)+len(errs)}"))
    with open(OUT, "w", encoding="utf-8") as f:
        f.write("\n".join(lines))
    print("written")


if __name__ == "__main__":
    main()
