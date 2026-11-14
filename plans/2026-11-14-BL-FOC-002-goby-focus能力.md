# BL-FOC-002 goby 引擎内联 focus 字段

## 目标

goby.json 条目可声明 `"focus": true`，命中后与其他引擎的重点标记走同一输出通道（`[*产品名]`）。

## 方案

- goby.go `GobyFinger` 加 `Focus bool`（json `focus,omitempty`），`Match` 命中后置 `frame.IsFocus`
- 新建 mylib/finger/goby/goby_test.go：内存构造引擎断言 focus 置位/非 focus 不置位 + JSON 新旧格式解析
- 文档：rule.md focus 段落补 goby 支持说明；doc/README.md goby 节补 focus 扩展字段（goby 格式的实际文档位置，立项时误写为 rule.md）

## 风险

- 现库（7075 条无 focus 字段）兼容：`omitempty` + bool 零值 false，无行为变化。

## 验证方式

1. TestFocusMatch / TestFocusJSONField 断言
2. 一次性取证：NewGobyEngine 加载现库 Len==7075、nginx 特征抽样命中（已通过，临时测试已清理）
3. go build finger 域 + common/resources 测试

## 决策记录

- 内联字段 vs 外置名单：用户拍板内联（与 fingers 概念一致、数据自包含）。
- 测试构造注意：goby 的 logic 是 label 布尔表达式（"a"/"a&&b"），单条件指纹用 `"a"`，不能用 "and"（会被拆成 a/n/d 三个 label）。
