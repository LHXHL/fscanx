# -*- coding: utf-8 -*-
r"""BL-FOC-004: 护网脆弱产品打标手术(451 条插入 focus:true) + 新增若依指纹条目

手术原则: token 级文本手术, 禁整体重序列化
  - 打标: 在目标对象的 "logic" 键行前插入 "focus": true 行(键序 name/focus/logic/rule)
  - 新增: 文件尾 "}]\" 前追加 ", {新条目}" 文本, 保持末尾无换行
验证前置(全过才落盘):
  [1] 解析元素数 == 7075
  [2] 重算命中清单 == 已确认的 451 条(与 scan_hw_focus.py 同源逻辑)
  [3] loads(after) == 期望结果(before + focus 键 + 若依条目) —— 深度相等强验证
  [4] LF-only / 无 BOM / 无尾换行
"""
import json

PATH = 'mylib/finger/resources/goby.json/goby.json'

# ---- 与 scan_hw_focus.py 同源的关键词与黑名单(复制保持自包含) ----
HW_CATEGORIES = {
    'Web框架与危险组件': [
        'shiro', 'fastjson', 'jackson', 'struts', 'spring-boot', 'spring-cloud',
        'spring-security', 'druid', 'nacos', 'solr', 'swagger', 'log4j',
        'dubbo', 'kafka', 'ofbiz', 'weblogic', 'jboss', 'wildfly', 'glassfish',
        'axis2', 'resin', 'tomcat', 'thinkphp',
    ],
    '国产OA': [
        'weaver', '泛微', 'e-bridge', 'seeyon', '致远', 'landray', 'tongda', '通达',
        'wanhu', '万户', 'ezoffice', '金和',
    ],
    'ERP与财务': [
        'yonyou', 'yongyou', 'ufida', '用友', 'grp-u8', 'tplus', 'kingdee',
        'apusic', 'sap-', 'yunhai', 'government affairs',
    ],
    'VPN与安全设备管理面': [
        'sangfor', '深信服', 'forti', 'pulsesecure', 'citrix', 'netscaler',
        'ssl-vpn', 'sslvpn', 'ipsec', 'cisco-asa', 'cisco-vpn',
        '天融信', '启明星辰', 'venustech', '网康', '网御', '奇安信',
        'ruijie', 'array', 'o2security', 'nsfocus',
        'srx', 'netscreen', 'screenos', 'junos', 'juniper-vpn', 'juniper-gateway',
        'juniper-dx', 'juniper-load', 'juniper-firewall', 'juniper-web-device',
    ],
    '视频监控与物联': [
        'hikvision', '海康', 'ivms', 'dahua', '大华', 'uniview', '宇视',
        'dvr', 'nvr', 'cctv',
    ],
    '堡垒机与运维面板': [
        'jumpserver', '堡垒机', '宝塔', 'webmin', 'phpmyadmin', 'zabbix',
        'grafana', 'nagios', 'cacti', 'jenkins', 'gitlab', 'sonarqube',
        'nexus-repository',
    ],
    '邮件系统': [
        'coremail', 'microsoft-exchange', 'turbomail', 'mailserver',
    ],
    '报表与低代码平台': [
        'finereport', '帆软', 'jeecg', '禅道', 'confluence', 'jira', 'yapi',
    ],
    '远程控制与访问': [
        'teamviewer', 'guacamole',
    ],
}
BLACKLIST = {
    190, 241, 4851, 3191, 4486,
    3275, 3278, 3338, 6147, 6149, 6481,
}

# 若依新条目文本(用户提供特征; 缩进/键序与库内风格一致, 2 空格/层)
RUOYI_ENTRY = '''{
  "name": "若依-RuoYi",
  "focus": true,
  "logic": "(a&&b) || (c&&d) || (e&&f)",
  "rule": [{
    "label": "a",
    "feature": "正在加载系统资源",
    "is_equal": true
  }, {
    "label": "b",
    "feature": "请耐心等待",
    "is_equal": true
  }, {
    "label": "c",
    "feature": "正在加载系统资源",
    "is_equal": true
  }, {
    "label": "d",
    "feature": "elementUI",
    "is_equal": true
  }, {
    "label": "e",
    "feature": "正在加载系统资源",
    "is_equal": true
  }, {
    "label": "f",
    "feature": "loader-wrapper",
    "is_equal": true
  }]
}'''

dec = json.JSONDecoder()


def parse_elements(raw):
    """流式解析数组元素, 返回 [(start, end, obj)], raw 以 '[' 开头 ']' 结尾"""
    assert raw[0] == '[' and raw.rstrip()[-1] == ']'
    elems = []
    i = 1
    while True:
        while i < len(raw) and raw[i] in ' \t\r\n,':
            i += 1
        if raw[i] == ']':
            break
        obj, end = dec.raw_decode(raw, i)
        elems.append((i, end, obj))
        i = end
    return elems


before_bytes = open(PATH, 'rb').read()
before = before_bytes.decode('utf-8')
elements = parse_elements(before)

# [1] 元素数校验
assert len(elements) == 7075, 'expect 7075 elements, got %d' % len(elements)
print('[1] elements == 7075 PASS')

# [2] 重算命中清单
tagged = {}  # index -> name
for i, (s, e, obj) in enumerate(elements):
    if i in BLACKLIST:
        continue
    name = obj['name']
    for kws in HW_CATEGORIES.values():
        if any(kw.lower() in name.lower() for kw in kws):
            tagged[i] = name
            break
assert len(tagged) == 451, 'expect 451 tagged, got %d' % len(tagged)
print('[2] tagged == 451 PASS')

# 手术: 逐目标在对象文本内 "logic" 键行前插入 focus 行
# 对象文本首字符为 '{', 成员缩进 2 空格; 插入文本与 '"logic"' 同缩进
out = []
last = 0
for i in sorted(tagged):
    s, e, obj = elements[i]
    seg = before[s:e]
    anchor = '  "logic":'
    pos = seg.index(anchor)  # 对象文本内首次出现即本对象 logic 键
    seg = seg[:pos] + '  "focus": true,\n' + seg[pos:]
    out.append(before[last:s])
    out.append(seg)
    last = e
out.append(before[last:])
after = ''.join(out)

# 追加若依条目: 末尾 "]\" 前插入 ", {entry}"(源文件可能为 CRLF, 归一为 LF)
assert after.endswith('}]') and not after.endswith('\n')
after = after[:-1] + ', ' + RUOYI_ENTRY.replace('\r\n', '\n') + ']'

after_bytes = after.encode('utf-8')

# [3] 深度相等强验证
expected = []
for i, (s, e, obj) in enumerate(elements):
    o = dict(obj)
    if i in tagged:
        assert 'focus' not in o
        o['focus'] = True
    expected.append(o)
expected.append(json.loads(RUOYI_ENTRY))
actual = json.loads(after)
assert actual == expected, 'deep equal FAIL'
assert len(actual) == 7076 and sum(1 for f in actual if f.get('focus')) == 452
print('[3] deep equal & counts (7076 / focus 452) PASS')

# [4] 形态约束
assert b'\r' not in after_bytes and not after_bytes.startswith(b'\xef\xbb\xbf')
assert after_bytes.endswith(b'}]') and not after_bytes.endswith(b'\n')
print('[4] LF-only / no BOM / no trailing newline PASS')

open(PATH, 'wb').write(after_bytes)
print('WRITTEN, bytes:', len(before_bytes), '->', len(after_bytes))
