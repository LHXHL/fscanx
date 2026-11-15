import request from '@/request'

// fscanx-web POC 域 API（BL-POC-004 多步结构化迭代轮）
// 谓词树节点：组节点（logic+conditions，任意嵌套）或条件节点（target/op/key/value）

export interface PocItem {
  name: string
  source: 'built-in' | 'external'
  always: boolean
}

export interface CondNode {
  logic?: string
  conditions?: CondNode[]
  target?: string
  op?: string
  key?: string
  value?: string
}

// 单步 rule 的表单视图（全字段）
export interface StepView {
  method: string
  path: string
  headers: Record<string, string>
  body: string
  search: string
  followRedirects: boolean
  continue: boolean
  expression: string
  tree: CondNode | null // null=该步超出谓词子集，前端走 CEL 文本兜底
}

export interface PocDetail {
  name: string
  always: boolean
  set: Array<{ key: string; value: string }>
  sets: Array<{ key: string; values: string[] }>
  steps: StepView[]
  form: boolean // false=groups 等长尾形态，仅源码模式
  yaml: string
}

export interface PutStepPayload {
  method: string
  path: string
  headers: Record<string, string>
  body: string
  search: string
  followRedirects: boolean
  continue: boolean
  tree?: CondNode
  expression?: string
}

export interface PutPocPayload {
  mode: 'form' | 'raw'
  name: string
  // form
  always?: boolean
  set?: Array<{ key: string; value: string }>
  sets?: Array<{ key: string; values: string[] }>
  steps?: PutStepPayload[]
  baseYaml?: string
  // raw
  yaml?: string
}

export const listPocsApi = () => {
  return request.get<{ pocs: PocItem[] }>({ url: '/api/pocs' })
}

export const getPocApi = (name: string) => {
  return request.get<PocDetail>({ url: '/api/poc', params: { name } })
}

export const putPocApi = (data: PutPocPayload) => {
  return request.put<{ status: string; file: string }>({ url: '/api/poc', data })
}

export const deletePocApi = (name: string) => {
  return request.delete<{ status: string }>({ url: '/api/poc', params: { name } })
}

export const validateCelApi = (expression: string) => {
  return request.post<{ ok: boolean; error?: string }>({ url: '/api/validate', data: { expression } })
}
