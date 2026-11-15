// 谓词树共享模型（CondTree 组件与 Poc 编辑器共用；CEL 序列化镜像后端 predicate.go，
// 保存时以后端生成与编译校验为准，前端仅做实时预览）

export interface CondNode {
  logic?: string // 'and' | 'or'（组节点）
  conditions?: CondNode[] // 组节点子项（可任意嵌套）
  target?: string // status | body | header（条件节点）
  op?: string // eq | contains | regex
  key?: string // header 名
  value?: string
}

export const TARGETS: Record<string, string> = { status: '状态码', body: '响应体', header: 'Header' }
export const OPS: Record<string, string> = { eq: '等于', contains: '包含', regex: '正则' }
export const OPS_BY_TARGET: Record<string, string[]> = {
  status: ['eq'],
  body: ['contains', 'regex'],
  header: ['contains']
}

export const opsOf = (target?: string) => OPS_BY_TARGET[target ?? ''] ?? []

// condComplete 条件节点填写完整（序列化前过滤半成品）
export const condComplete = (n?: CondNode | null): boolean => {
  if (!n) return false
  if (n.logic) return (n.conditions ?? []).some(condComplete)
  if (!n.target) return false
  if (n.target === 'header') return !!n.key && n.value !== ''
  return n.value !== ''
}

// pruneTree 剪掉空组与半成品条件（返回 null 表示全空）
export const pruneTree = (n?: CondNode | null): CondNode | null => {
  if (!n) return null
  if (n.logic) {
    const kids = (n.conditions ?? []).map(pruneTree).filter(Boolean) as CondNode[]
    if (!kids.length) return null
    return { logic: n.logic, conditions: kids }
  }
  return condComplete(n) ? { ...n } : null
}

// nodeToCel 谓词树 → CEL 预览（镜像后端 ToCEL；bmatches 是参数反转形态）
export const nodeToCel = (n?: CondNode | null): string => {
  const t = pruneTree(n)
  if (!t) return ''
  if (t.logic) {
    const joiner = t.logic === 'or' ? ' || ' : ' && '
    return '(' + (t.conditions ?? []).map(nodeToCel).join(joiner) + ')'
  }
  if (t.target === 'status') return `response.status == ${t.value}`
  if (t.target === 'body' && t.op === 'contains') return `response.body.bcontains(b"${t.value}")`
  if (t.target === 'body' && t.op === 'regex') return `"${t.value}".bmatches(response.body)`
  if (t.target === 'header') return `response.headers["${t.key}"].contains("${t.value}")`
  return ''
}
