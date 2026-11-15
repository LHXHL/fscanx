# RDP 协议正确性模块 Backlog

RDP 协议正确性模块：排查与修复 grdp 库（`mylib/grdp`，搬库自 tumi8/goplusRDPRemote 系）在 974150e/bd7e479 重构中的认证层回归嫌疑。BL-BLD-001 修复测试编译后，三个从未运行过的断言首跑失败（测试文件 2026-11-15 前因编译错误从未执行过，回归未被任何 CI 拦截）。

| ID | 标题 | 优先级 | 状态 | 验收状态 |
|----|------|--------|------|----------|
| BL-RDP-001 | RDP NLA 认证回归排查与修复 | P1 | 已完成 | 已验收 |
| BL-RDP-002 | RDP 截屏黑块修复（指令宣告退化 + 色深映射） | P1 | 已完成 | 已验收 |

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


## BL-RDP-002 RDP 截屏黑块修复（指令宣告退化 + 色深映射）

- 模块：RDP 协议正确性
- 优先级：P1
- 状态：已完成
- 价值：消除截屏黑块——Win2003"很多黑色色块"与新版 Windows"右侧中部固定黑块"。
- 关联模块文档：docs/modules/09-RDP协议正确性模块.md
- 关联计划：plans/2026-11-15-BL-RDP-002-截屏黑块.md
- 关联代码：mylib/grdp/protocol/pdu/pdu.go、mylib/grdp/login/screen.go
- 验收状态：已验收

### 方案（立项时必填）

- **做什么**：① 取消 DSTBLT/PATBLT/SCRBLT 绘制指令的能力宣告（客户端无绘制实现，宣告支持导致服务器用指令画的区域保持黑块）；② 修正 Bpp() 色深映射（15bpp 误映射 1 字节；8bpp 缺失；default 零值无兜底）+ ToRGBA 按原始位深分支 + 未知位深跳过绘制。
- **为什么**：代码审查取证（用户无 Win2003 环境）——orders 事件全仓零消费方、orders.go 零绘制代码（grep 计数 0）而能力协商宣告了三种绘制指令；Bpp() 映射 15→1（应 2）、8 无分支（FreeRDP 规范映射 8→1/15,16→2/24→3/32→4）；Win2003 降级发 15/16bpp 时解码全错。
- **怎么做**：pdu.go 注释三行 OrderSupport 赋值（数组零值即不支持）；screen.go Bpp() 改规范映射 + default 显式 0；Bitmap.BitsPerPixel 存原始位深，ToRGBA 按原始位深分支（8bpp 无调色板支持以灰度近似并注明 TODO）；拼图循环 step==0 跳过（防零步死循环）。
- **为什么这么做**：①是最小改动让服务器退化纯位图更新（黑块根因 A 直击，代价带宽略增）；②是色深正确性修复（根因 B），实现 orders 绘制（彻底方案）留待后续按需立项。

### 范围

- pdu.go OrderSupport 宣告、screen.go Bpp/ToRGBA/拼图循环
- Bpp 映射单测
- CHANGELOG

### 不做

- orders 绘制实现（PATBLT/SCRBLT 真正画图，工作量大另立项）
- 调色板同步（FASTPATH_UPDATETYPE_PALETTE，8bpp 仅灰度近似）
- Win2003 真机验证（用户无环境，2008R2 间接验证留待用户）

### 验收条件

- [x] 构建与 grdp/finger 域测试回归通过（实测：go build ./... 与主程序通过，grdp 全域 ok、finger 6 包 ok）
- [x] Bpp 映射单测（8/15/16/24/32/未知）PASS（实测：TestBpp/TestToRGBA PASS——15/16bpp 白色向量、8bpp 灰度、24bpp BGR 序断言）
- [x] Win11 真机截屏验证（实测：探针 rdpshot 实拍登录欢迎界面，视觉模型分析确认无黑色色块、无条纹/错位/色彩异常，图像完整清晰；桌面态二次截屏因画面静止未收到新 bitmap 未落图）
- [x] 用户多机场景最终确认（2026-11-15 用户真机测试通过；Win2003 无环境，留待 2008R2 间接验证）
- [x] docs/CHANGELOG.md 记录

### 状态记录

- 2026-11-15：立项（黑块排查报告获用户确认，批准修复 1+2；orders 绘制实现另行立项）。
- 2026-11-15：执行完成。改动三处：pdu.go 取消 DSTBLT/PATBLT/SCRBLT 能力宣告（黑块根因 A 直击，服务器退化纯位图更新）；screen.go Bpp() 修正为 FreeRDP 规范映射（8→1/15,16→2/24→3/32→4/未知→0，原 15→1 误映射 + 8 缺失）；ToRGBA 按原始位深分支（8bpp 灰度近似并注明调色板 TODO）+ 拼图 step==0 跳过（防零步死循环）。新增 login 包 screen_test.go（Bpp 映射 + ToRGBA 色彩向量）。
- 2026-11-15：探针沉淀 plans/scripts/go/rdpshot（实拍验证用，OutputDir 可传参）。观察：日志见 sec ERROR_ALERT ErrorCode 7（会话仍成功，疑似 license/安全层告警，未影响功能，留观）。
- 2026-11-15：用户真机测试通过，闭合。附带排查结论记录：`-screen` 主程序路径硬编码空凭据+SSL（Plugins/rdp.go:70），对 NLA 目标必然失败（CredSSP 在图形阶段前拒绝无效凭据，取证双协议均 tls: access denied 零 bitmap），且错误日志被注释静默——属设计限制非回归，用户决策不改造（NLA 目标截屏需有效凭据，用 rdpshot 类探针或未来增强）。
