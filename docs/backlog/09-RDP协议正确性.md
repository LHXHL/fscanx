# RDP 协议正确性模块 Backlog

RDP 协议正确性模块：排查与修复 grdp 库（`mylib/grdp`，搬库自 tumi8/goplusRDPRemote 系）在 974150e/bd7e479 重构中的认证层回归嫌疑。BL-BLD-001 修复测试编译后，三个从未运行过的断言首跑失败（测试文件 2026-11-15 前因编译错误从未执行过，回归未被任何 CI 拦截）。

| ID | 标题 | 优先级 | 状态 | 验收状态 |
|----|------|--------|------|----------|
| BL-RDP-001 | RDP NLA 认证回归排查与修复 | P1 | 已完成 | 已验收 |

## BL-RDP-001 RDP NLA 认证回归排查与修复

- 模块：RDP 协议正确性
- 优先级：P1
- 状态：已完成
- 价值：恢复 RDP 登录/爆破的认证正确性——若回归确认，所有走 NTLMv2 NLA 的 RDP 目标认证必失败（功能静默失效）。
- 关联模块文档：docs/modules/09-RDP协议正确性模块.md
- 关联计划：plans/2026-11-15-BL-RDP-001-NLA修复.md
- 关联代码：mylib/grdp/protocol/nla/ntlm.go、mylib/grdp/core/rle.go、mylib/grdp/login/screen.go
- 验收状态：已验收

### 方案（立项时必填）

- **做什么**：三线排查与修复——① NTLMv2 响应构造（`ComputeResponseV2` blob 尾部多 4 字节 0 嫌疑）；② DERT/TSRequest 编码漂移（DER 包装多 8 字节）；③ rle decompress3 越界。①② 用真机动态裁定，③ 静态分析。
- **为什么**：BL-BLD-001 取证——`TestNTLMv2_ComputeResponse` 首跑失败但 LM 响应与期望完全一致（HMAC 基础正确），NT 摘要与 SessionBaseKey 不符且实际 blob 尾部多 4 字节 0；断言期望值来自上游实现，974150e 后从未运行过。
- **怎么做**：静态对照 [MS-NLMP] NTLMv2 blob 规范逐字节裁定 4 字节 0 是否多余 → 真机两阶段测试（用户提供 Win11 192.168.111.76：第一阶段 NLA 关验证基础链路，第二阶段 NLA 开裁定认证层）→ 按裁定修复 + 恢复被 skip 的断言 → 真机回归。
- **为什么这么做**：认证构造对错不能靠猜——规范比对给理论裁定，真机登录给实证闭合；两阶段矩阵隔离归因（基础链路 vs 认证层），避免"连不上"误判。

### 范围

- nla 包（ntlm.go/cssp.go）修复 + 断言恢复
- rle.go 越界修复（若可静态定位）
- 真机验证记录
- CHANGELOG

### 不做

- grdp 库整体升级/替换
- fscanx RDP 插件层逻辑（除非排查证明插件层也有问题）

### 验收条件

- [x] 静态裁定结论落档：4 字节 0 与 DERT 8 字节差异的规范依据（[MS-NLMP] 2.2.2.2——serverInfo 已含 MSV_AV_EOL，末尾不应再补 4 零；DERT 8 字节 = NTLMSSP Version 字段，旧期望源自上游无 Version 旧实现）
- [x] 真机阶段一（NLA 关）：基础链路行为记录（通——alice 登录链路 SUCCESS；且负对照实证错误口令/不存在用户也 SUCCESS，NLA 关时 client.Login 返回值不验凭据，爆破判定设计上只认 NLA 目标）
- [x] 真机阶段二（NLA 开）：修复前认证失败实证（单测向量不符即铁证，等价实证）+ 修复后登录成功实证（Win11 真机双对照：正确口令 flag=true，错误口令 flag=false tls access denied）
- [x] 被修复路径对应测试断言恢复真跑且通过（TestNTLMv2_ComputeResponse / TestEncodeDERTRequest / TestSum 三者恢复真跑全 PASS）
- [x] `go build ./...` 与 finger/grdp 域测试回归（grdp 全 ok、finger 6 包全 ok、全量构建通过）
- [x] docs/CHANGELOG.md 记录

### 状态记录

- 2026-11-15：立项（BL-BLD-001 发现移交）。用户提供 Win11 测试机 192.168.111.76（alice/baidu.com@123），两阶段矩阵。
- 2026-11-15：执行完成。核心修复 = ntlm.go ComputeResponseV2 删 tempBuff 末尾多余 4 零；三重裁定闭环（规范比对→单测向量→真机双对照）。附带：DERT 期望值按真机实证更新（Version 字段无害演进）；rle CVAL 补长度守卫（FreeRDP C 直译缺陷，合法路径零变化）。**影响面结论：修复前主程序 RDP 爆破对 NLA 目标全废（正确口令也必失败），修复后恢复**。意外发现：NLA 关闭目标不可爆破属设计使然（Crack 判定只认 HYBRID，SSL 层不验凭据，负对照实证）。探针沉淀 plans/scripts/go/rdpprobe、rdpcrack。
- 2026-11-15：用户以 fscanx 主程序对测试机（Win11/NLA）跑 RDP 爆破验收通过，闭合。
