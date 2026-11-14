# -*- coding: utf-8 -*-
r"""BL-FOC-003: goby.json \uXXXX 转义解码为 UTF-8 原文(token 级文本手术)

验证全过才落盘:
  [1] 代理对+中文人工样例单测(防御性, 现库无 BMP 外字符)
  [2] 转换前后 json.load 深度相等
  [3] 非字符串 token 区域逐字节不变(缩进/键序/结构)
  [4] 纯 LF / 无 BOM / 无尾换行
"""
import json
import re

PATH = 'mylib/finger/resources/goby.json/goby.json'


def fix_surrogates(val):
    """组合合法代理对为单一字符; 孤立代理返回 None(调用方保留原 token 文本)"""
    if any(0xD800 <= ord(c) <= 0xDFFF for c in val):
        try:
            return val.encode('utf-16', 'surrogatepass').decode('utf-16')
        except UnicodeDecodeError:
            return None
    return val


def convert(raw):
    """token 级手术: 非字符串区域原样保留, 字符串 token 解码后 ensure_ascii=False 重编码"""
    dec = json.JSONDecoder()
    out = []
    i, n = 0, len(raw)
    while i < n:
        nxt = raw.find('"', i)
        if nxt == -1:
            out.append(raw[i:])
            break
        if nxt > i:
            out.append(raw[i:nxt])
        val, end = dec.raw_decode(raw, nxt)
        fixed = fix_surrogates(val)
        if fixed is None:
            out.append(raw[nxt:end])  # 孤立代理: 保留原 token 文本
        else:
            out.append(json.dumps(fixed, ensure_ascii=False))
        i = end
    return ''.join(out)


def nonstring_regions(raw):
    """提取全部非字符串 token 区域文本(结构符/数字/布尔/空白)"""
    dec = json.JSONDecoder()
    parts = []
    i, n = 0, len(raw)
    while i < n:
        nxt = raw.find('"', i)
        if nxt == -1:
            parts.append(raw[i:])
            break
        if nxt > i:
            parts.append(raw[i:nxt])
        _, end = dec.raw_decode(raw, nxt)
        i = end
    return ''.join(parts)


# --- [1] 代理对防御性单测(人工构造样例) ---
# 注: CPython json 对"转义文本形式"的合法代理对会组合为单一字符(2026-11-14 文件级探测裁定);
#     孤立代理转义会解析为孤立代理字符(utf-8 编码会崩), fix_surrogates 对其保留原 token。
#     相等口径用解码后的真实字符判断(裸代理字符输入不组合, 属另一输入类别)。
ESC = '\\' + 'u'  # 防 shell/编辑器折叠, 显式拼出反斜杠+u
sample_in = '{"a":"' + ESC + 'd83d' + ESC + 'de00","b":"' + ESC + '4e2d' + ESC + '6587"}'
sample_out = convert(sample_in)
sample_back = json.loads(sample_out)
assert sample_back['a'] == '\U0001F600' and sample_back['b'] == '中文', '样例未解码为原文'
sample_out.encode('utf-8')
print('[1] surrogate+cjk sample PASS:', sample_out)

# --- 主转换(验证全过才落盘) ---
before_bytes = open(PATH, 'rb').read()
assert not before_bytes.startswith(b'\xef\xbb\xbf'), '原文含 BOM?'
before = before_bytes.decode('utf-8')
after = convert(before)
after_bytes = after.encode('utf-8')

assert json.loads(before) == json.loads(after), '前后 json.load 深度不等'
print('[2] deep equal PASS')

assert nonstring_regions(before) == nonstring_regions(after), '非字符串区域漂移'
print('[3] non-string tokens byte-identical PASS')

assert b'\r' not in after_bytes, '引入 CR'
assert not after_bytes.startswith(b'\xef\xbb\xbf'), '引入 BOM'
assert not after_bytes.endswith(b'\n') and after_bytes.endswith(b'}]'), '尾换行漂移'
print('[4] LF-only / no BOM / no trailing newline PASS')

esc_pattern = re.compile(re.escape(ESC) + '[0-9a-fA-F]{4}')
esc_before = len(esc_pattern.findall(before))
esc_after = len(esc_pattern.findall(after))
print('[5] u-escape count:', esc_before, '->', esc_after,
      '; bytes:', len(before_bytes), '->', len(after_bytes))

open(PATH, 'wb').write(after_bytes)
print('WRITTEN')
