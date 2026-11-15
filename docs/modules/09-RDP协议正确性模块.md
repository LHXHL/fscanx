# RDP 协议正确性模块

## 模块定位

维护 grdp 库（`mylib/grdp`，RDP/VNC 客户端协议实现，服务 fscanx 的 RDP 登录与口令爆破）的协议正确性。当前焦点：974150e/bd7e479 重构引入的认证层回归嫌疑（BL-BLD-001 发现，断言因编译错误从未运行过、回归未被拦截）。

## 关键事实（2026-11-15 取证）

- `ComputeResponseV2`（ntlm.go）：LM 响应与上游期望一致，NT 摘要不符且实际 blob 尾部多 4 字节 0（tempBuff 末尾多写一层）——嫌疑改坏 NTLMv2 NTAuthenticator，NLA 认证路径必失败。
- `EncodeDERTRequest`（cssp.go）：DER 包装比期望多 8 字节（NTLMSSP 内容一致），TSRequest 编码漂移。
- rle decompress3 对历史样例越界 panic（与认证无关）。
- 测试环境：用户 Win11 虚拟机 192.168.111.76（alice / baidu.com@123），两阶段测试矩阵（NLA 关→基础链路基线，NLA 开→认证裁定）。

## 关联

- Backlog：docs/backlog/09-RDP协议正确性.md
- 发现来源：docs/backlog/08-构建健康.md BL-BLD-001 状态记录
