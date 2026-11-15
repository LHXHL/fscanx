import request from '@/request'

// fscanx-web 绑定关系域 API（BL-POC-004 前端重做）

export interface Bind {
  product: string
  pocs: string[]
}

export interface BindsView {
  binds: Bind[]
  externalPath: string
  hasExternal: boolean
}

export const getBindsApi = () => {
  return request.get<BindsView>({ url: '/api/binds' })
}

export const putBindsApi = (binds: Bind[]) => {
  return request.put<{ status: string; path: string }>({ url: '/api/binds', data: { binds } })
}

export const listProductsApi = () => {
  return request.get<{ products: string[] }>({ url: '/api/products' })
}
