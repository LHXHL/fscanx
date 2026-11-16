import request from '@/request'

// fscanx-web POC 域 API（BL-POC-004 多步结构化迭代轮）
// 谓词树节点：组节点（logic+conditions，任意嵌套）或条件节点（target/op/key/value）

export interface PocItem {
  name: string
  source: 'built-in' | 'external'
  always: boolean
  file?: string // 文件名（内置=embed 真实文件名，内置源写回定位用）
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
  source: 'built-in' | 'external' // 本次定位来源（编辑上下文）
  file?: string // 文件名（内置=embed 真实文件名）
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
  target?: 'built-in' | 'external' // 保存目标：内置源（下次编译生效）/ 外置 pocs/（默认，立即生效）
  file?: string // 打开时的文件名（target=built-in 时作改名守卫）
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

export const getPocApi = (name: string, source?: string) => {
  return request.get<PocDetail>({ url: '/api/poc', params: { name, source } })
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
