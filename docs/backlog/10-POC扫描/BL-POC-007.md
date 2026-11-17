# BL-POC-007 fscanx-web 登录认证（默认账号）

- 模块：POC 扫描
- 优先级：P1
- 状态：已完成
- 当前阶段：持续迭代
- 价值：维护端具备真实写能力（binds/poc/指纹改写），从免登录硬注入改为最小认证门——部署端口不再裸奔，写接口有身份门槛。
- 关联模块文档：docs/modules/10-POC扫描模块.md
- 关联计划：plans/2026-11-16-BL-POC-007-web登录认证.md
- 关联代码：cmd/fscanx-web/auth.go、cmd/fscanx-web/main.go、web/apps/admin/src/main.ts、web/apps/admin/src/api/login/index.ts
- 验收状态：已验收

### 方案（立项时必填）

- **做什么**：后端 `/api/user/login`（校验默认账号 admin/admin123，签发 crypto/rand 内存会话 token）+ `/api/user/logout`（销毁 token）；中间件对 `/api/*`（除 login）校验 Authorization token，无效返回 200+{code:401,message} 信封——前端拦截器既有链路自动 logout+回登录页（零前端守卫改动）。前端：登录 API 指向真实端点、删 main.ts 免登录硬注入、logoutApi 指向真实登出。
- **为什么**：用户指令"现在需要登录，搞一个默认账号 admin/admin123"。免登录是 BL-POC-004 期临时形态（main.ts 注入 accessToken:'local'），维护端写能力增强后（005 起含 SQLite 主存 CRUD）认证缺位成为裸奔面。
- **怎么做**：凭据常量 + 内存 token 表（进程生命周期，重启即失效，前端 401 信封自动登出再登录）；token 走既有 Authorization 头（拦截器已发送 userStore.token，接受裸 token 与 Bearer 前缀）。
- **为什么这么做**：本地单用户维护端——否决多用户/角色/密码哈希存储/防爆破限速（成本与威胁不匹配；凭据即配置，改密码=改常量重编译或后续再参数化）；复用 401 信封让前端"会话过期自动登出"链路零改动（permission.ts 守卫与拦截器均已按该契约实现）；静态资源不设防（登录页本身要能加载）。

### 范围

- cmd/fscanx-web/auth.go（新：凭据/会话/登录登出/中间件）+ auth_test.go
- cmd/fscanx-web/main.go（路由注册 + mux 包中间件）
- web/apps/admin/src/api/login/index.ts（真实端点）、src/main.ts（删注入）

### 不做

- 多用户/角色权限模型
- 密码可配置化（flag/env/存储）——后续有需要再参数化
- HTTPS / 防爆破限速（本地回环工具语义）
- 注册流程（框架模板残留组件不接后端）

### 验收条件

- [x] 错误凭据登录被拒并提示；admin/admin123 登录进入且 /api/routes 正常加载（实测 headless：wrongpass 提示"fscanx-web: 用户名或密码错误"，admin/admin123 登录后离开登录页、绑定页 15 种子行加载）
- [x] 未登录直接访问页面路由重定向登录页；登录后刷新保持会话；服务端重启后旧 token 401 自动登出（实测 A1 重定向 /login?redirect=/binds/list；A5 刷新仍在；curl 登录→重启→同 token 调 /api/binds_list 返回 401）
- [x] 未带/伪造 token 调 /api/* 返回 {code:401} 信封（实测伪造 token 注入 localStorage 刷新被清回登录页；curl 无 token 调 binds_add 返回 401——跨站盲写面同步封死）
- [x] logout 后服务端 token 失效（单测 TestUserLogout_InvalidatesToken + headless A6/A7 登出回登录页、直访再被拦）
- [x] go test 单测过（登录成功/失败/Content-Type 拒绝/登出失效/中间件拦截与放行/Bearer 前缀/静态与 login 白名单，7 测试全过）
- [x] bash web/build.sh 产物版 headless：登录全链 9/9 零非预期 console 错误（错误密码的拦截器 throw 属预期行为已排除口径）

### 依赖

- BL-POC-005（写接口是被保护对象；401 信封机制先例）

### 状态记录

- 2026-11-16：用户指令"现在需要登录，那就搞一个默认账号admin/admin123"直接立项（视为范围批准，P1 立即执行）。免登录是 BL-POC-004 期临时形态（main.ts 注入 accessToken:'local'），permission.ts 守卫与拦截器的 401 登出链路本就按真实认证设计，本次只是补上服务端。
- 2026-11-16：执行完成。交付：auth.go（凭据常量 admin/admin123 + crypto/rand 内存会话 token + /api/user/login|logout + authMiddleware：/api/* 除 login 白名单，无效 token 返 200+{code:401} 信封）；main.go 路由+中间件+横幅打印账号；前端登录/登出 API 换真实端点、删 main.ts 注入与未用 import。验证：单测 7/7、headless 产物版登录全链 9/9（重定向/错误提示/登录/加载/刷新保持/登出/再拦截/伪造 token 401 自动登出/零非预期错误）、curl 重启失效与授权写/无 token 写 401。**意外发现（方法学）**：构建顺序坑——go build 先于 web/build.sh 时二进制 embed 旧前端，浏览器拿旧 UI 造成假失败（登录提交打到 /mock/user/login），正确顺序=前端产物先行再 go build；与 go:embed 产物版终验教训同源已入记忆。附带收益：认证门同时封死了此前 reviewer 提到的跨站无预检盲写面。置已完成+待验收。
- 2026-11-16：验收闭合（用户浏览器实测确认测试通过）。置已验收。
- 2026-11-16：**性能回归修复轮（用户报告：POC 页点"绑定产品"卡顿 7-8 秒）**。排查证据：服务端三 API 合计 <150ms（binds_list 2-12ms、products 50-73ms、pocs 47-52ms）排除后端；headless 实测点击→弹窗 15.58s——瓶颈为弹窗内 `el-select multiple` 对产品名全集（goby 7200+fingers 554 去重后近万项）全量渲染 el-option DOM（Element Plus 非虚拟化选择器的挂载开销）。修复：换虚拟化选择器 `el-select-v2`（options 数组形态，只渲染可视区），实测点击→弹窗 **0.15s**；展开/输入过滤（近万项仅渲染 3 项可视项）/多选/确认写盘功能链复测通过；POC 页 11 项 UI 回归零破坏。绑定关系页产品名控件为 el-autocomplete（输入才过滤）不受同类问题影响。教训入档：**下拉候选可能到近万量级时禁用普通 el-select 全量 v-for**，须用 select-v2 虚拟化或 autocomplete 惰性渲染。
