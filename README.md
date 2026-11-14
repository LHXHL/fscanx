# fscanx

fscan 是一款开源的企业内网资产梳理软件，fscanx 是基于它的个人打磨版：单二进制、一条命令完成存活探测、端口扫描、协议指纹、web 指纹、目录发现、弱口令与 POC 检测。

定位是**内外兼修、轻车重炮**：把它当"车"用，直接投递到目标内网运行（icmp 快速测绘）；把它当"炮"用，通过原生 socks5 代理远程侦察——fscanx 专门优化了代理模式下的端口探测、协议识别、爆破与 POC（含 udp 代理、带认证代理、非标准 socks5 的误报兼容）。造轮子的原因见尾部[关于 fscanx](#关于-fscanx)。

## 功能一览

| 能力 | 说明 | 相关选项 |
|------|------|----------|
| 存活探测 | icmp（令牌桶限速防打崩路由器）+ tcp，vpn 场景可指定出网网卡 | `-np` `-prate` `-pt` `-iface` |
| 大网段智能探测 | A/B 段先用网关位采样（默认 1,2,253,254 的 80 端口 + icmp）筛出存活 C 段再细扫 | `-auto` `-am` `-ap` `-ai` `-atime` |
| 端口扫描 | 区间/列表/文件/默认 80 端口集，可增删 | `-p` `-pa` `-pn` `-portf` `-t` |
| 协议指纹识别 | gonmap 引擎，30+ 常见协议探针优选（http/ssh/smb/rdp/mysql/redis...），socks5 下无误报 | `-nmap` |
| web 指纹识别 | fingers + goby 双引擎，重点资产标记，指纹库可自行维护 | — |
| 目录扫描 | 每个存活站点用内置 70 条指纹导向路径探隐藏后台（/seeyon/、/druid/...），只报新指纹 | `-dir` `-dirf` |
| web 信息收集 | 标题、多形态 js 跳转提取、https 证书（CN/O/OU/域名）、邮箱、copyright | `-tn` `-wt` |
| ip 归属地 | 同目录放 qqwry.dat 自动加载，输出 ip 定位 | — |
| 弱口令爆破 | smb/ssh/ftp/rdp/mysql/mssql/redis/... 自定义用户口令字典 | `-br` `-bt` `-user` `-pwd` `-userf` `-pwdf` |
| smb/ftp 附加探测 | 匿名登录检测 + 共享目录/文件清单遍历 | — |
| rdp 探测/截图/爆破 | NLA 下 win7+ 版本探测、win2003+ 免登录截屏、全版本无假阳性爆破 | `-screen` |
| POC 扫描 | xray yaml 兼容引擎，按名称过滤、dnslog、shiro 全 key | `-poc` `-pocname` `-pocpath` `-full` `-dns` |
| 命令执行 | ssh / wmiexec 上线执行 | `-c` `-wmi` `-hash` |
| masscan 联动 | 管道 `masscan \| fscanx -std`，或 `-msc` 由 fscanx 自动拉起并托管 masscan | `-std` `-msc` `-mr` `-mp` `-margs` |
| 混合目标输入 | `-hf` 一文件混吃 ip / ip:port / cidr / cidr:port / url / 域名 / masscan 输出 | `-hf` `-pd` `-dp` |
| socks5 全链路 | 端口扫描+协议识别+web+poc+爆破全走代理，支持账号密码认证与 udp（netbios 等） | `-socks5` `-proxy`（http 代理仅 url 目标） |
| 输出 | result.txt / json / FscanOutput 表格化 | `-o` `-json` `-no` `-silent` |

指纹识别效果（重点资产自动标记）：

![指纹识别效果](README.assets/image-20251124132644895.png)

智能探测存活段与 `-msc` 进度行：

![智能探测](README.assets/image-20251124090732282.png)

![msc进度](README.assets/image-20261113184734586.png)

## 快速开始

本项目不提供可执行文件，自行编译（Windows/Linux/macOS 均可）：

```bash
# 方式一：直接 go build
go build .

# 方式二：build.bat（Windows）

# 方式三：goreleaser 多平台
go install github.com/goreleaser/goreleaser@latest
goreleaser build --snapshot --clean
```

三条最常用命令：

```bash
# 内网 C 段常规扫描（icmp 验活 + 端口 + 协议指纹）
fscanx -h 192.168.1.0/24 -nmap

# 公网信息收集：ip/域名/url 混到一个 txt，连带 c 段一起探
fscanx -hf target.txt -pd -nmap -np

# masscan 极速联动（fscanx 自己拉起 masscan，进度固定底部）
fscanx -msc -h 192.168.0.0/16 -p 80,443 -mr 600
```

## 使用案例

**（1）大网段快速扫描**。`-auto` 智能预扫描筛存活 C 段；`-np` 跳过后续 icmp（有些 ip 只开端口 ping 不到）：

```bash
fscanx -h 192.168.0.0/16 -auto -nmap -t 1000 -np
```

`-h 10` / `-h 172` / `-h 192` 分别等价于 10.0.0.0/8、172.16-31、192.168.0.0/16。A 段实测智能验活约 15 分钟（tcp 13min + icmp 2.5min，内存 ~120MB），B 段仅需 11 秒。已知网关在 .253、防火墙在 .128？`-ai 253,128 -ap 22,80` 精准点名。

**（2）公网信息收集**。收集到的 ip、ip 段、子域名、url 每行一个扔进 txt，一股脑探测；`-pd` 把 url/域名解析出的 ip 所在 C 段也加入端口扫描（cdn 节点 ip 场景自行斟酌）：

```bash
fscanx -hf target.txt -pd -nmap -np
```

**（3）批量 url web 扫描**。`-tn` 控制 web 并发（默认 60）：

```bash
fscanx -hf url.txt -tn 200
```

**（4）masscan 联动**。两种方式：

```bash
# 管道式（需要修改版 masscan：https://github.com/killmonday/masscan-own/releases）
masscan --rate 200 -p 80,443 192.168.0.0/16 | fscanx -std -nmap -t 200

# 内嵌式（推荐，避免两个程序抢终端输出）
fscanx -msc -h 192.168.0.0/16 -p 80,443 -mr 600
```

`-msc` 模式：masscan 原始输出不再打印，进度固定终端底部（写 stderr，重定向不受影响）；`-mp` 手动指定 masscan 路径（默认按 PATH → 当前目录 → fscanx 所在目录自动查找，按文件头识别系统架构）；`-margs` 只放 fscanx 未托管的额外参数（如 `--wait 3`），写 `-p/--ports`、`--rate`、`-iL`、`--exclude` 等已托管参数会被拒绝；`-hn` 排除网段；Ctrl+C 第一次停 masscan 最多等 3 秒收尾，超时或第二次立即清理临时文件并以 130 退出。

**（5）目录扫描（发现隐藏后台）**。对每个存活站点用内置 70 条指纹导向路径探测，识别出**新**产品指纹的路径并入结果：

```bash
fscanx -h 192.168.1.0/24 -p 80,8080 -dir
```

- 流量均摊：任意时刻单站点在途请求 ≤1，多站点交错，不轰单点；
- 误报防护：非 2xx 一律不参与识别；302 仅跟随同域名同前缀跳转；与首页 (指纹,标题) 完全相同的路径不重复报；标题空不算差异；
- 输出节奏：目录结果是站点级聚合，先扫完的站点先输出（目录行集中在扫描后段属预期）；
- 默认关闭（请求量放大约 70 倍），`-dirf` 可换外部字典。

**（6）代理模式**（高架炮远程侦察）：

```bash
fscanx -socks5 socks5://127.0.0.1:1080 -h 192.168.1.1/16 -auto

# 带账号密码认证（gost、3proxy 等自建服务端常开认证）
fscanx -socks5 socks5://user:pass@127.0.0.1:1080 -h 192.168.1.1/16 -auto
```

`-socks5` 覆盖端口扫描+协议识别+web+poc+爆破全链路（含 udp，netbios 走 udp 137 也能探）；`-proxy` 是 http 代理，仅 url 目标可用（http 代理本身不能路由 tcp/udp）。密码含 `@`、`:` 特殊字符一般可直接写，解析异常时用 URL 转义（`%40`、`%3A`）。clash/v2ray 这类非标准 socks5 不会误报——fscanx 启动时探测代理标准性，非标准代理用 http 探针二次验真，原理详见[发布说明的谜题分析](docs/releases/2025-11.md)。

**（7）漏洞扫描**：

```bash
fscanx -socks5 socks5://127.0.0.1:1080 -h 192.168.1.1/24 -poc -p 80,443,8080
```

**（8）弱口令爆破**：

```bash
fscanx -socks5 socks5://127.0.0.1:1080 -h 192.168.1.1/24 -br -p 21,22,445,3306,6379

# 自定义用户口令
fscanx -h 192.168.1.1/24 -br -p 445 -user administrator,admin -pwd 123456,admin
```

**（9）rdp 探测 / 截图 / 爆破**。`-screen` 对 win2003+ 的 RDP/SSL 协议免登录截屏（仅 NLA 协议的机器截屏为黑），rdp 传图延迟高，`-time` 调大些：

```bash
fscanx -h 192.168.181.1/24 -p 3389 -nmap -screen -time 18
```

截图保存在 `img/<时间戳>/`。爆破适配 win2003-win11 全版本无假阳性（原版对 NLA 机器全误报成功），注意 Windows 可能锁账号。

**（10）vpn 场景防错扫**。vpn 接入后探测流量可能走本地网卡扫到运营商网段，用 `-iface` 指定 vpn 虚拟网卡 ip：

```bash
fscanx -nmap -iface 10.202.100.100 -h 192.168.0.0/16
```

## 参数速查

以 `common/flag.go` 为准（默认值括号内）。

**目标与输入**

| 参数 | 说明 |
|------|------|
| `-h` | 目标：`192.168.11.11` / `192.168.11.11-255` / 逗号分隔 / cidr / `10`/`172`/`192` 简写 |
| `-hn` | 排除主机（支持 cidr） |
| `-hf` | 目标文件：ip / ip:port / cidr / cidr:port / url / 纯域名 / masscan 输出混合，每行一条 |
| `-u` / `-uf` | 单个 url / url 文件 |
| `-p` | 端口（默认 80 个常用端口集：21,22,80,443,445,3306,3389,6379,8080...） |
| `-pa` / `-pn` / `-portf` | 在默认端口集上追加 / 排除端口 / 端口文件 |
| `-pd` | url/域名解析 ip 的 C 段加入端口扫描 |
| `-dp` | 域名目标绑定全部 `-p` 端口生成 `http(s)://domain:port` |
| `-m` | 任务类型（all/icmp/web/webpoc...，影响探测端口与任务） |

**智能探测（大网段）**

| 参数 | 说明 |
|------|------|
| `-auto` | 开启智能预扫描（仅 /8、/16 与 `10`/`172`/`192` 简写） |
| `-am` | 验活协议（tcp,icmp；socks5 模式自动只用 tcp） |
| `-ap` | 验活 tcp 端口（80） |
| `-ai` | C 段采样 ip 位（1,2,253,254） |
| `-atime` | 验活 tcp 超时秒（3） |

**扫描行为与并发**

| 参数 | 说明 |
|------|------|
| `-t` | 端口扫描并发（512） |
| `-tn` | web 扫描并发（60；-std/-msc 模式由 -t 统一控制） |
| `-bt` | 爆破并发（24） |
| `-time` | tcp 超时秒（6；同时决定 gonmap 每端口 4 探针的总时长） |
| `-wt` | web 请求超时秒（9） |
| `-nmap` | 开启 gonmap 协议指纹识别 |
| `-np` | 跳过 icmp 验活 |
| `-ping` | 用系统 ping 命令替代 icmp conn |
| `-prate` | icmp 发包速率 M/s（0.1，实测最佳，调高可能漏资产） |
| `-pt` | ping 超时秒（5） |
| `-iface` | 指定出网网卡 ip |
| `-top` | 存活段展示数（10） |

**masscan 联动**

| 参数 | 说明 |
|------|------|
| `-std` | 从标准输入读 ip:port/url 逐行扫描 |
| `-msc` | fscanx 自动查找并启动 masscan，流式消费其输出 |
| `-mr` | masscan 发包速率（600 pps） |
| `-mp` | masscan 路径（空则自动查找） |
| `-margs` | 额外 masscan 参数（托管参数写入会被拒绝） |

**目录扫描**

| 参数 | 说明 |
|------|------|
| `-dir` | 开启指纹导向目录扫描（默认关） |
| `-dirf` | 外部路径字典（默认内置 70 条） |

**代理**

| 参数 | 说明 |
|------|------|
| `-socks5` | socks5 代理（支持 `socks5://user:pass@ip:port`），全链路生效 |
| `-proxy` | http 代理，仅 url 目标（web/poc） |

**爆破与命令执行**

| 参数 | 说明 |
|------|------|
| `-br` | 开启弱口令爆破 |
| `-user` / `-pwd` | 用户 / 口令（逗号分隔） |
| `-usera` / `-pwda` | 在默认字典上追加 |
| `-userf` / `-pwdf` | 用户 / 口令文件 |
| `-domain` | smb 域 |
| `-sshkey` | ssh 私钥文件 |
| `-hash` | hash 传递 |
| `-c` | 命令执行（ssh/wmiexec） |
| `-wmi` | wmi 模式 |
| `-path` | fcgi/smb 远程文件路径 |

**POC**

| 参数 | 说明 |
|------|------|
| `-poc` | 开启 POC 扫描 |
| `-pocname` | 按名称过滤（如 weblogic） |
| `-pocpath` | POC 文件路径 |
| `-full` | 全量扫描（如 shiro 100 key） |
| `-num` | poc 并发（30） |
| `-cookie` | POC 请求 cookie |
| `-dns` | 使用 dnslog poc |
| `-noredis` | 跳过 redis 安全测试 |
| `-rf` / `-rs` | redis 写 sshkey / 写 cron（如 `-rs 192.168.1.1:6666`） |
| `-sc` | ms17 shellcode |

**输出**

| 参数 | 说明 |
|------|------|
| `-o` | 结果文件（result.txt） |
| `-json` | json 输出 |
| `-no` | 不写文件 |
| `-silent` / `-nocolor` | 静默 / 无色 |
| `-debug` | 错误日志间隔秒（60） |

## 维护指南

**web 指纹**（fingers/goby 引擎，数据来自 [chainreactors/fingers](https://github.com/chainreactors/fingers)）：

1. 编辑明文：`mylib/finger/resources/fingers_http.json/fingers_http.json`、`goby.json/goby.json`
2. 重打包：项目根执行 `go run ./mylib/finger/cmd/pack`（纯 Go，无需 gzip 命令，三平台通用；内置 JSON 合法性校验）
3. `go build .` 生效

指纹编写参考 [fingers SDK 文档](https://wiki.chainreactors.red/libs/fingers/sdk/#%E5%8A%A8%E6%80%81%E6%B3%A8%E5%86%8C)。goby 库历史垃圾指纹多、易误报，已清理过两轮（2025.10 与 2026.11），遇到新误报继续清。

**协议探针**：修改 `mylib/finger/nmap/` 相关探针数据源可增删协议识别；注意每端口 gonmap 探测超时只够 4 个探针（`-time` 的 4 倍），加探针需同步评估。

**结果表格化**：FscanOutput 脚本把 result.txt 转成直观表格（端口/存活段/系统/弱口令/web 产品分表），打包成 exe 后直接把 result.txt 拖上去即可。

**项目文档**：`docs/` 下有完整文档体系——[CHANGELOG](docs/CHANGELOG.md)（逐条更新）、模块文档（设计细节）、[releases](docs/releases/)（大版本发布说明）。

## 更新历史

> 逐条细粒度更新进 [docs/CHANGELOG.md](docs/CHANGELOG.md)，此处保留版本级时间线。

- **2026.11.14**，fingers 指纹库误报清理：删除六类高危误报指纹 60 条 + 2 字中文公司名特征，4349→4289；新增纯 Go 重打包工具 `cmd/pack` 替代 gzip 命令依赖。
- **2026.11.14**，新增指纹导向目录扫描 `-dir`/`-dirf`：轮询环调度流量均摊、四道判定闸免疫软 404、站点聚合输出，发现根路径之外的隐藏后台。
- **2026.11.13**，新增 masscan 内嵌联动模式 `-msc`：fscanx 自己查找并启动 masscan，实时读取结果复用 `--std` 管线，进度固定终端底部，避免双进程抢输出。配套 `-mr`（速率，默认 600）、`-mp`（手动指定路径）、`-margs`（追加未托管参数）。
- **2026.08.13**，README 补充带认证 socks5 代理用法（socks5://用户名:密码@ip:端口），代码早已支持仅文档缺漏。
- **2026.06.10**，修复 socks5+nmap 探测对不开放端口误报为开放（新增超时逻辑漏判代理模式）；修复 netbios 信息收集一处报错。
- **2026.04.24**，修复 `-nmap` 下端口默认插件不比对协议的问题（3306 端口跑 rdp 的奇葩场景现在会用正确探针）。
- **2026.02.26**，修复 ftp 匿名登录在部分非 ftp 服务上卡死（底层库仅连接时设了超时）。
- **2026.02.07**，修复 gonmap 在探针超时后误判端口关闭（超时后补一次 tcp 连接验证真实开放）；协程池健壮性修复。
- **2025.11.25 大更新**，重写 README 的那次大版本：大网段智能探测（`-auto`）、内存优化（1.8G→百兆级，详见 fscanx/httpx/spray 三方实测对比）、指纹库换 fingers 引擎、`-hf` 混合输入与 `-pd`、qqwry ip 归属地、`-poc`/`-br` 开关反转、socks5 误报谜题的根因分析与兼容方案。完整自述见 [docs/releases/2025-11.md](docs/releases/2025-11.md)。
- **2025.10.11**，删除实战中遇到的垃圾指纹；修复原版 fscan 漏掉 http 响应体为空资产的 bug。
- **2025.09.14**，新增 `-std` 标准输入管线（masscan | fscanx 极速联动，配合修改版 masscan 的 `-y` 输出格式）；FscanOutput 适配。
- **2025.09.11**，修复 websocket 101 响应无 EOF 导致扫描卡住。

## 关于 fscanx

我有一个梦想🤡，让 fscan 再次伟大。

众所周知，fscan是一款开源的企业内网资产梳理软件，在无数个寂寞的夜晚，它陪伴安服仔们度过漫漫长夜——当靶标和我之间，只剩倾盆的思念，空无一人的设备房间，态势感知的告警却响彻天边，我独自一人和眼泪周旋，生如浮萍般卑微，爱却苍穹般壮烈。回头再看一眼这璀璨星空，灿烂火光就像盛夏烟火，历经摧残，就要璀璨。

### 为什么造轮子

即便在cmd里敲击了那么多遍，fscan依然有一些另我头疼的问题，大部分并非工具本身有质量问题，而是设计理念、应用场景和我的需求有所不同：

1. 工具直接落地到目标内网机器上运行，效果很好。但通过代理的方式扫描，就会出现大量误报，每个扫描的端口都会显示开放（不论使用sockscap还是proxychains），并且在代理模式下特定端口上的功能插件无法使用——ssh、smb、ftp 等协议使用的库本身没有写 socks5 支持，proxychains 之类也不知道为什么没有 hook 成功。像我这样的猥琐狗，是不可能落地运行的。维护一个代理工具的免杀就够呛了，其他用到网络的工具都可以通过代理程序对内网操作，做一个工具的免杀就能避免做无数个工具的免杀（本地提权类程序除外）。
2. ping扫描无法设定速率，在带宽允许下甚至飙升到10MB/s的icmp包，直接把菜一点的路由器打崩。而不加限制的ping扫描可能带来网络故障，ping扫描又极大提高内网测绘效率，不能不要。
3. 这端口开放，扫都扫了，不如把协议识别、web产品识别（产品、容器、框架、标题、https证书、版本）也做了，一步到位。
4. vpn连接目标内网扫描时，会把本地网络上层的同网段资产扫进来，造成误判干扰，有时甚至打错目标。

### 致谢

从 fscan、kscan 这些开源工具中学到了很多，对作者们怀揣志高敬意，感谢他们的优秀作品为后人奠定了扎实的地基。fscanx 的 web 指纹引擎来自 [chainreactors/fingers](https://github.com/chainreactors/fingers)（gogo 作者团队出品，罕见的匠心团队）。rdp 协议研究参考了[死磕RDP协议，从截图和爆破说起](https://zhuanlan.zhihu.com/p/625706098)与[先知社区的相关文章](https://xz.aliyun.com/t/11978)。

## 编译

为了避免不必要的麻烦，本项目不提供可执行文件，请自行编译。

```bash
# goreleaser 多平台
go install github.com/goreleaser/goreleaser@latest
goreleaser build --snapshot --clean

# 或 go build
go build .
```
