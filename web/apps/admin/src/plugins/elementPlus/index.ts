import type { App } from 'vue'

import ElementPlus from 'element-plus'
import 'element-plus/dist/index.css'

// fscanx-web：维护端页面大量使用 el-table/el-form/el-select 等组件，
// 全量注册（原模板仅注册 ElLoading，导致组件解析失败被渲染为原生元素，
// scoped slot 被当 children 无参调用而报 Cannot read 'row' of undefined）。
export const setupElementPlus = (app: App<Element>) => {
  app.use(ElementPlus)
}
