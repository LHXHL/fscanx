# -*- coding: utf-8 -*-
"""BL-FOC-004 v2: 策展后的护网脆弱产品命中清单(误伤剔除+泛词收紧)"""
import json

PATH = 'mylib/finger/resources/goby.json/goby.json'

# 决策记录(相对 v1 的收紧):
# - 删泛词 vpn(92 条含大量 SOHO 宽带路由) -> 保留 ssl-vpn/sslvpn/ipsec 等企业 VPN 词
# - 删 nc-(SONY 摄像机误伤)/eas(Easy* 杂牌误伤)/asa-(CASA 误伤)/owa(Secoway 误伤)
# - 删 e-office(HPE 误伤, 泛微-EOffice 由"泛微"覆盖)/exchange 改精确 microsoft-exchange
# - nexus 收紧为 nexus-repository; inspur 收紧为 yunhai/government affairs(服务器型号不打)
# - juniper 收紧(srx/netscreen/screenos/junos/vpn/gateway/dx/load/firewall/web-device), EX/QFX 交换机不打
# - 删 ipc(误伤 IPCop)/dss-(REACH 编码器)/harbor(条目质量存疑)
# - 新增: thinkphp(PHP RCE 高频)/yongyou(拼写变体)/ruijie/array/o2security/sap-(词边界排除 Sapido)
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

# 误伤黑名单(人工复核剔除, 词级收紧后仍混入的个别条目)
BLACKLIST = {
    190,    # With -isum420g3 disk array('array' 命中存储阵列)
    241,    # Kedacom-disk array(同上)
    4851,   # Kedacom-disk array system(同上)
    3191,   # Dell-EMC-Xtremio-full flash array(同上)
    4486,   # NETGEAR-SRX5308('srx' 命中 SOHO 路由)
    3275, 3278, 3338, 6147, 6149,  # QNO/5WAN SOHO 宽带 VPN 路由('ipsec'带入)
    6481,   # 'Eman - Made Manyongyouacturing' 数据脏条目(Manufacturing 被污染)
}

data = json.load(open(PATH, encoding='utf-8'))

by_entry = {}  # index -> (keywords, name)
for cat, kws in HW_CATEGORIES.items():
    for kw in kws:
        for i, f in enumerate(data):
            if kw.lower() in f['name'].lower():
                by_entry.setdefault(i, [set(), f['name']])[0].add(kw)

for i in BLACKLIST:
    by_entry.pop(i, None)

lines = []
total = 0
for cat, kws in HW_CATEGORIES.items():
    entries = []
    for i, (cats, name) in sorted(by_entry.items()):
        if cats & set(kws):
            entries.append((i, name))
    total += 0
    lines.append('== %s (%d)' % (cat, len(entries)))
    for i, name in entries:
        lines.append('  %d: %s' % (i, name))

print('\n'.join(lines))
print()
print('TOTAL unique entries to tag:', len(by_entry), '/', len(data))
