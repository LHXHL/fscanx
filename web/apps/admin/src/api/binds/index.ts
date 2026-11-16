import request from '@/request'

// fscanx-web 绑定关系域 API（BL-POC-005 SQLite 存储化：单条 CRUD，PUT 全量退役）

export interface Bind {
  product: string
  pocs: string[]
}

export interface BindsView {
  binds: Bind[]
  externalPath: string
  hasExternal: boolean
}

// 生效视图：内置种子 ⊕ DB 外置层（展示用；服务端在每次读写前自动吸收 binds.yaml 手改）
export const getBindsListApi = () => {
  return request.get<BindsView>({ url: '/api/binds_list' })
}

// 添加绑定（upsert-append：产品已存在则把 pocs 追加去重）
export const bindsAddApi = (product: string, pocs: string[]) => {
  return request.post<{ status: string; path: string }>({ url: '/api/binds_add', data: { product, pocs } })
}

// 编辑绑定（original=编辑前产品名，product 可与 original 不同实现改名）
export const bindsEditApi = (original: string, product: string, pocs: string[]) => {
  return request.post<{ status: string; path: string }>({ url: '/api/binds_edit', data: { original, product, pocs } })
}

export const bindsDeleteApi = (product: string) => {
  return request.post<{ status: string; path: string }>({ url: '/api/binds_delete', data: { product } })
}

export const listProductsApi = () => {
  return request.get<{ products: string[] }>({ url: '/api/products' })
}
