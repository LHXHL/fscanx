import request from '@/request'

// fscanx-web 产品册域 API（BL-POC-009 三视角维护端：标准产品对象 = 标准名 + 别名 + 测试名单）
// 出厂（embed/-root 源文件）与外置（catalog.db + 二进制旁 catalog.yaml 快照）双轨合并视图。

export interface CatalogEntryView {
  name: string
  vendor: string
  product: string
  alias: Record<string, string[]> // 键=指纹源（goby/ehole/fingerprinthub/fingers），值=该源写法
  pocs: string[]
  origin: 'built-in' | 'external'
  missing_alias: boolean // 别名区为空且非出厂自标准名形态（可能死条目，警示标）
  has_pocs: boolean
}

export interface CatalogListResult {
  entries: CatalogEntryView[]
  external_path: string
  built_in_path: string
  has_external: boolean
  build_hint: string
}

// 生效视图：出厂 ⊕ 外置（同名整条覆盖；出厂删除=墓碑已过滤不展示）
export const getCatalogListApi = () => {
  return request.get<CatalogListResult>({ url: '/api/catalog_list' })
}

// 新建标准产品条目（必须挂别名防死条目；名字在指纹输出名全集时允许空别名——自标准名形态）。
// target: external（默认，外挂文件立即生效）| built-in（直接进代码源文件，下次 go build 生效）
export const catalogAddApi = (entry: CatalogEntryPayload, target: 'built-in' | 'external' = 'external') => {
  return request.post<{ status: string; path: string; hint?: string }>({ url: '/api/catalog_add', data: { target, entry } })
}

// 编辑条目：original 定位 + target 双目标——built-in 写回出厂源文件（重编译生效）/
// external 落 DB 外置层（立即生效）。整条全量写出（别名区+测试名单全量，禁止局部覆盖）。
export const catalogEditApi = (original: string, target: 'built-in' | 'external', entry: CatalogEntryPayload) => {
  return request.post<{ status: string; path: string; hint?: string }>({ url: '/api/catalog_edit', data: { original, target, entry } })
}

// 删除：出厂条目落墓碑（绑定失效、别名归一保留）；外置真删；幂等
export const catalogDeleteApi = (name: string) => {
  return request.post<{ status: string; path: string }>({ url: '/api/catalog_delete', data: { name } })
}

// 批量指纹归属判定（指纹视角"产品归属"列数据源）
export interface ResolveItem {
  source: string
  name: string
}

export interface ResolveVerdict {
  source: string
  name: string
  owner: string
  relation: 'alias-of' | 'self' | 'unassigned'
}

export const catalogResolveApi = (items: ResolveItem[]) => {
  return request.post<{ verdicts: ResolveVerdict[] }>({ url: '/api/catalog_resolve', data: { items } })
}

// 写端点条目载体（与后端 CatalogEntry 同构；alias 键=指纹源）
export interface CatalogEntryPayload {
  name: string
  vendor?: string
  product?: string
  alias: Record<string, string[]>
  pocs?: string[]
}
