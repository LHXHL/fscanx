# BL-POC-004 fscanx-web 维护端

## 目标

独立二进制维护端：poc 表单化编辑（零手写 yaml/CEL）、绑定关系编辑、指纹明文编辑与一键重打包。

## 交付物结构

- `mylib/finger/packlib/`：从 cmd/pack 提取的共享包（确定性 gzip + JSON 校验），pack 命令与 web 端同源（字节等价前提）。提取后产物 md5 与提取前一致（实测）。
- `cmd/fscanx-web/`：标准库 net/http（零新依赖）+ go:embed 原生前端单页。
  - `predicate.go`：谓词树↔CEL 双向。子集四类条件（status==N / body.bcontains / "re".bmatches(response.body) / headers[K].contains）；**bmatches 注册签名是 string receiver**（lib/eval.go string_bmatches_bytes），生态统一参数反转形态。反解析 cel-go AST 白名单遍历，子集外 nil → 前端源码模式。
  - `api_poc.go`：列表（内置 embed+外置）/读（yaml 原文+谓词树）/写（form 生成模板 或 raw 整文，落盘前 yaml 解析+每条 expression 编译双重校验）/删（仅外置）。
  - `api_binds.go`：GET 合并视图（SeedBinds+外置 MergeBinds）；PUT round-trip 校验后写外置。
  - `api_finger.go`：明文 textarea 读写（JSON 校验防线，内容即所写=零漂移）；POST pack（packlib.Run）；GET products（两明文 name 全集，绑定下拉数据源）。
  - `static/index.html`：三 tab（POC/绑定/指纹），谓词表单（target/op 下拉联动、header 名输入、全局 AND/OR、行删、实时预览+校验按钮）。

## 验证结果（2026-11-15，全过）

| 验收 | 证据 |
|------|------|
| ① 表单新建→落盘→执行 | webform-demo.yml 生成正确；fscanx -pocname 命中 + 目标 GET /webform-demo-marker |
| ② 绑定编辑→定向生效 | thinkphp→[webform-demo] 覆盖种子；空 -pocname 定向只发新 poc，原 3 个内置 thinkphp 路径消失 |
| ③ 非法拦截 4 例 | 表单值非法/坏 yaml/CEL 编译失败(int==string)/空 content 全 400 |
| ④ pack 字节等价 | web /api/pack 产物 md5 与共享包基线一致（d30955d2…） |
| ⑤ 指纹零漂移 | API 取文→原样回写→三方 md5 一致（70b7630d…） |

## 执行中发现

- Bind 结构缺 json tag（只有 yaml tag），API 大写字段与前端不匹配——已修复。
- go1.20 无 `"GET /path"` 方法路由模式，handler 内分发。
- Windows python 不认 MSYS /tmp 路径，验收脚本一体化进内存（urllib）。
- `-silent` 会吞 PocScan 成功行输出（验收计数陷阱）。

## 范围裁剪（二期候选）

- 谓词表单嵌套树 UI（一期扁平+全局 AND/OR，嵌套走源码模式）。
- 指纹结构化单条目编辑（一期 textarea 整文）。
