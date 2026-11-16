import 'vue/jsx'

import 'virtual:uno.css'

// 初始化多语言
import { setupI18n } from '@/plugins/vueI18n'

// 引入状态管理
import { setupStore } from '@/store'

// 全局组件
import { setupGlobCom } from '@/components'

// 引入element-plus
import { setupElementPlus } from '@/plugins/elementPlus'

// 引入全局样式
import '@vea/styles'

// 路由
import { setupRouter } from './router'

import { createApp } from 'vue'

import App from './App.vue'

import { setupPermission } from './permission'

const setupMock = async () => {
  if (import.meta.env.PROD && import.meta.env.VITE_USE_MOCK === 'true') {
    const { setupProdMockServer } = await import('../mock/_browser')
    await setupProdMockServer()
  }
}

// 创建实例
const setupAll = async () => {
  await setupMock()

  const app = createApp(App)

  setupStore(app)

  // 登录认证（BL-POC-007）：会话由登录页 POST /api/user/login 建立，不再注入本地会话；
  // 未登录由 permission.ts 守卫重定向 /login，会话过期由拦截器 401 信封自动登出
  await setupPermission()

  setupI18n(app)

  setupGlobCom(app)

  setupElementPlus(app)

  setupRouter(app)

  app.mount('#app')
}

setupAll()
