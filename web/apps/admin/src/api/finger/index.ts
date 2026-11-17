import request from '@/request'

// fscanx-web 指纹库域 API（BL-POC-006 条目化 CRUD）
// 条目身份 = (file, id)；name 不构成身份（重名常态）。旧整文 GET/PUT /api/finger 已退役。

export interface FingerEntryItem {
  id: number
  name: string
  raw: string // 条目原始 JSON 文本（前端结构化回填/降级原文判定的数据源）
}

export interface FingerListResult {
  total: number
  page: number
  size: number
  items: FingerEntryItem[]
}

export interface PackResult {
  src: string
  dst: string
  raw: number
  gz: number
}

export const fingerListApi = (file: string, keyword = '', page = 1, size = 20) => {
  return request.get<FingerListResult>({ url: '/api/finger_list', params: { file, keyword, page, size } })
}

// 表单新增/编辑：entry 为所在源 schema 的结构化对象（后端按源序列化，canonical 键序）
export const fingerAddApi = (file: string, entry: Record<string, unknown>) => {
  return request.post<{ status: string }>({ url: '/api/finger_add', data: { file, entry } })
}

// 编辑：mode=form（结构化）或 mode=raw（原文文本，内容即所写，后端校验 JSON 对象+name 非空）
export const fingerEditApi = (
  file: string,
  id: number,
  payload: { mode: 'form'; entry: Record<string, unknown> } | { mode: 'raw'; text: string }
) => {
  return request.post<{ status: string }>({ url: '/api/finger_edit', data: { file, id, ...payload } })
}

export const fingerCopyApi = (file: string, id: number) => {
  return request.post<{ status: string }>({ url: '/api/finger_copy', data: { file, id } })
}

export const fingerDeleteApi = (file: string, id: number) => {
  return request.post<{ status: string }>({ url: '/api/finger_delete', data: { file, id } })
}

export const packApi = () => {
  return request.post<{ status: string; results: PackResult[] }>({ url: '/api/pack' })
}

// 两指纹明文的产品名全集（后端 handler 在 api_finger.go listProducts；BL-POC-009 起
// 供产品册候选辅助使用——注意这是"指纹写法"命名空间，与标准产品条目名不同）
export const listProductsApi = () => {
  return request.get<{ products: string[] }>({ url: '/api/products' })
}
