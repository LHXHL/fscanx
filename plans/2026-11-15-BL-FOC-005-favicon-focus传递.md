# BL-FOC-005 favicon 哈希命中路径的 focus 传递

## 目标

fingers 库 focus:true 的指纹经 favicon 哈希（md5/mmh3）命中时，输出 `[*产品名]`（与 goby 路径行为一致）。

## 当前状态

- 断裂点定位（2026-11-15 用户真机实测发现）：
  - fingers/engine.go:89-101 favicon 编译把指纹压成 hash→name map（focus 丢失）
  - favicon/favicon.go:32-41 HashMatch 创建 Framework 不设 IsFocus
  - 对照正确路径：fingers.go:121/151/200（文本匹配）与 goby.go:115 均正确置位
  - 聚合处 mylib/finger/engine.go Compile 只搬 hash→name 两个 map

## 执行计划

1. favicon/favicon.go：FaviconsEngine 加 FocusNames map[string]bool（NewFavicons 初始化；HashMatch 命中后按 name 置 IsFocus）
2. fingers/engine.go:89-101：编译循环 finger.Focus 时填充 FocusNames
3. mylib/finger/engine.go Compile：fingers 的 FocusNames 拷入主 FaviconEngine（fingerprinthub/ehole 无 focus 概念不搬）
4. 测试：favicon 包 HashMatch focus 单测 + Plugins 层端到端断言 `[*name]`

## 风险

- 低：纯增量字段，不改既有 map 结构与匹配语义；未注入 FocusNames 时行为与现状完全一致

## 验证方式

- 单测两路径置位/不置位断言；go build + finger 域测试；Plugins 层 `[*name]` 端到端

## 决策记录

- 按 name 存 focus 而非按 hash：同名多哈希去重、md5/mmh3 通用、聚合拷贝量小
- 不改 Md5Fingers/Mmh3Fingers 值类型（保持 map[string]string）：fingerprinthub/ehole 共用结构无 focus 概念，避免三个引擎连坐改动
