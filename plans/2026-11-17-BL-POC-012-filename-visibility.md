# plans/2026-11-17-BL-POC-012-filename-visibility.md

## 目标

fscanx-web 文件名可见性补齐（纯前端两文件）：POC 列表展示 yaml 文件名 + 搜索同时匹配名称/文件名 + 编辑器标题悬浮显示当前文件；标准产品页「存放位置」tag 悬浮与页头问号 tooltip 显示出厂/外挂两个维护文件的具体完整路径。

## 当前状态

- 2026-11-17：立项（用户批准：文件名进列表列、路径 tooltip 化）。数据契约核查完毕——/api/pocs 已返回 file、catalog_list 已返回 built_in_path+external_path，零后端改动。

## 执行计划

1. 重读 Poc.vue（用户已自行修改保存区布局，行号有变）。
2. Poc.vue：filtered 扩展 file 匹配、placeholder 改文案、列表加「文件名」列（show-overflow-tooltip）、存放位置列保持、左列卡片 380→430、编辑器标题 tooltip 显示 selFile。
3. Catalog.vue：load() 读 external_path 存 ref；存放位置 tag tooltip 换具体路径；页头问号 tooltip 补维护文件两行。
4. 构建验证：bash web/build.sh + go build。
5. Reviewer 验证 + deslop。
6. 同步矩阵收尾 + check-sync。

## 风险

- 左列卡片加列后拥挤 → 卡片 380→430 + 列溢出 tooltip 兜底；编辑器侧损失 50px 可接受（表单行为 flex 自适应）。
- 内置/外挂路径可能为空（external 无文件时 external_path 仍返回 catalogPath，非空；built_in_path 依赖 -root 指向仓库）→ tooltip 对空值兜底显示"—"。

## 验证方式

- 验收条件逐条 grep/产物实测；构建双链通过；check-sync 无 HIGH。

## 决策记录

- 文件名进列表列（用户拍板，肉眼可扫可复制）。
- 产品册路径 tooltip 化不常驻（用户拍板，遵守 011 文案纪律）。
- 搜索语义扩展为 name+file（用户痛点即"难搜索"）。
