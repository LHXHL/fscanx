<template>
  <div class="catalog-page">
    <el-card shadow="never">
      <template #header>
        <div class="card-head">
          <span class="head-title">
            标准产品（{{ rows.length }} 条{{ hasExternal ? '，含外挂条目' : '，全部代码内置' }}）
            <el-tooltip placement="top" popper-class="help-tip">
              <template #content>
                这里维护「产品叫什么（别名）+ 测什么（测试名单）」：别名让识别引擎把各种指纹写法统一成一个标准名输出；测试名单是指纹命中后定向执行的 POC。条目有两个存放位置——代码源文件（下次 go build 生效，随版本分发）或外挂文件（重跑扫描器即生效），编辑时可选择。
              </template>
              <Icon icon="mdi:help-circle" :size="14" class="help-icon" />
            </el-tooltip>
          </span>
          <div class="head-actions">
            <el-input v-model="keyword" size="small" placeholder="搜索标准名/别名/测试名..." clearable style="width: 220px" />
            <el-button type="primary" size="small" @click="openAdd">+ 新建标准产品</el-button>
          </div>
        </div>
      </template>

      <el-table :data="pagedRows" size="small" border>
        <el-table-column label="标准名" min-width="180">
          <template #default="scope">
            <span>{{ scope.row.name }}</span>
            <el-tooltip v-if="scope.row.missing_alias" content="别名区为空且不是指纹输出名——识别引擎报不出这个名字，绑定不会命中（死条目）" placement="top">
              <el-tag type="warning" size="small" style="margin-left: 6px">缺别名</el-tag>
            </el-tooltip>
          </template>
        </el-table-column>
        <el-table-column label="别名（各指纹源写法）" min-width="260">
          <template #default="scope">
            <template v-if="aliasList(scope.row).length">
              <el-tooltip v-for="a in aliasList(scope.row)" :key="a.key" :content="`源：${a.key}`" placement="top">
                <el-tag size="small" class="alias-tag">{{ a.value }}</el-tag>
              </el-tooltip>
            </template>
            <span v-else class="empty-cell">—</span>
          </template>
        </el-table-column>
        <el-table-column label="测试名单" min-width="260">
          <template #default="scope">
            <template v-if="scope.row.pocs?.length">
              <el-tag v-for="p in scope.row.pocs" :key="p" size="small" type="success" class="poc-tag">{{ p }}</el-tag>
            </template>
            <span v-else class="empty-cell">—</span>
          </template>
        </el-table-column>
        <el-table-column label="存放位置" width="100" align="center">
          <template #default="scope">
            <el-tooltip
              :content="scope.row.origin === 'built-in' ? '定义在代码源文件（catalog.yaml），改动下次 go build 后生效、随版本分发' : '保存在 fscanx-web 旁的外挂文件，重跑扫描器即生效'"
              placement="top"
            >
              <el-tag :type="scope.row.origin === 'built-in' ? 'info' : 'success'" size="small">
                {{ scope.row.origin === 'built-in' ? '代码内置' : '外挂文件' }}
              </el-tag>
            </el-tooltip>
          </template>
        </el-table-column>
        <el-table-column label="操作" width="110" align="center">
          <template #default="scope">
            <el-button link type="primary" size="small" @click="openEdit(scope.row)">编辑</el-button>
            <el-button link type="danger" size="small" @click="removeRow(scope.row)">删除</el-button>
          </template>
        </el-table-column>
      </el-table>

      <el-pagination
        v-model:current-page="page"
        v-model:page-size="pageSize"
        :total="filtered.length"
        :page-sizes="[10, 20, 50]"
        layout="total, sizes, prev, pager, next"
        size="small"
        style="margin-top: 10px; justify-content: flex-end"
      />
    </el-card>

    <!-- 新建/编辑共用弹窗：别名候选辅助 + POC 多选（候选近万项，el-select-v2 虚拟化） -->
    <el-dialog v-model="dlg.visible" :title="dlg.isAdd ? '新建标准产品' : `编辑标准产品：${dlg.original}`" width="680px">
      <div class="dlg-field">
        <label>标准名</label>
        <el-input v-model="form.name" size="small" placeholder="产品的规范名（识别命中后扫描输出显示此名）" style="flex: 1" />
      </div>
      <div class="dlg-field">
        <label>厂商/模块</label>
        <el-input v-model="form.vendor" size="small" placeholder="厂商（可空，如 weaver）" style="width: 220px" />
        <el-input v-model="form.product" size="small" placeholder="产品模块名（可空，如 e-cology）" style="flex: 1" />
      </div>

      <div class="dlg-section">
        别名（识别引擎把以下写法归一到标准名）
        <el-tooltip placement="top" content="绑定命中依赖「识别输出 = 标准名」——产品在指纹库里的每个真实写法都应收进对应源的别名区">
          <Icon icon="mdi:help-circle" :size="14" class="help-icon" />
        </el-tooltip>
      </div>
      <div class="dlg-field">
        <label>候选辅助</label>
        <!-- el-select-v2：指纹写法全集近万项，普通 el-select 全量渲染会卡顿（BL-POC-008 前科） -->
        <el-select-v2
          v-model="aliasCandidate"
          filterable
          clearable
          :reserve-keyword="false"
          :options="productOptions"
          placeholder="从指纹库写法全集选择（可输入过滤）"
          style="width: 300px"
        />
        <el-select v-model="aliasCandidateSource" size="small" style="width: 130px">
          <el-option v-for="s in aliasSources" :key="s" :label="s" :value="s" />
        </el-select>
        <el-button size="small" @click="addAliasFromCandidate">添加</el-button>
      </div>
      <div v-for="s in aliasSourceRows" :key="s" class="dlg-field">
        <label>{{ s }}</label>
        <el-input
          v-model="aliasText[s]"
          size="small"
          type="textarea"
          :rows="2"
          :placeholder="`该源写法，一行一个（可空）`"
          style="flex: 1"
          class="mono"
        />
        <el-button v-if="!FIXED_ALIAS_SOURCES.includes(s)" link type="danger" size="small" @click="removeAliasSource(s)">删源</el-button>
      </div>

      <div class="dlg-section">
        测试名单（指纹命中后定向执行的 POC）
        <el-tooltip placement="top" content="绑定命中宽松匹配（含子串）：绑「泛微」可覆盖泛微系产品；命中后仅执行名单内 POC">
          <Icon icon="mdi:help-circle" :size="14" class="help-icon" />
        </el-tooltip>
      </div>
      <div class="dlg-field">
        <label>POC 列表</label>
        <el-select-v2
          v-model="form.pocs"
          multiple
          filterable
          collapse-tags
          collapse-tags-tooltip
          :reserve-keyword="false"
          :options="pocOptions"
          placeholder="点击选择 POC（可输入过滤）"
          style="flex: 1"
        />
      </div>

      <!-- 保存位置（新建/编辑都可选：外挂文件立即生效 / 代码源文件下次 go build 生效） -->
      <div class="dlg-field">
        <label>保存位置</label>
        <el-radio-group v-model="dlg.target" size="small">
          <el-radio-button value="built-in">代码源文件（下次 go build 生效）</el-radio-button>
          <el-radio-button value="external">外挂文件（重跑扫描器生效）</el-radio-button>
        </el-radio-group>
      </div>
      <div v-if="dlg.target === 'built-in'" class="dlg-hint">
        {{ dlg.isAdd ? '新条目直接写入代码里的 catalog.yaml，重新编译 fscanx 后生效（在此之前扫描器暂不识别该条目）' : dlg.origin === 'built-in' ? '写回代码里的 catalog.yaml 源文件，重新编译 fscanx 后生效；已部署扫描器不受影响' : '该条目将转正写入代码源文件（原外挂记录自动清理），重新编译 fscanx 后生效' }}
      </div>

      <template #footer>
        <el-button size="small" @click="dlg.visible = false">取消</el-button>
        <el-button type="primary" size="small" :loading="dlg.saving" @click="saveEntry">
          {{ dlg.target === 'built-in' ? '保存到代码源文件' : '保存到外挂文件' }}
        </el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { catalogAddApi, catalogDeleteApi, catalogEditApi, getCatalogListApi, type CatalogEntryView } from '@/api/catalog'
import { listProductsApi } from '@/api/finger'
import { listPocsApi } from '@/api/poc'

defineOptions({ name: 'Catalog' })

const rows = ref<CatalogEntryView[]>([])
const hasExternal = ref(false)
const builtInPath = ref('')
const keyword = ref('')

// 客户端搜索 + 分页（数据量=出厂 50 条+用户增量，服务端分页属过度设计，与 Binds 页先例一致）
const page = ref(1)
const pageSize = ref(20)
const filtered = computed(() => {
  const q = keyword.value.trim().toLowerCase()
  if (!q) return rows.value
  return rows.value.filter((r) => {
    if (r.name.toLowerCase().includes(q)) return true
    if ((r.pocs ?? []).some((p) => p.toLowerCase().includes(q))) return true
    return Object.values(r.alias ?? {}).some((names) => names.some((n) => n.toLowerCase().includes(q)))
  })
})
const pagedRows = computed(() => filtered.value.slice((page.value - 1) * pageSize.value, page.value * pageSize.value))
// 搜索条件变化后当前页可能越界，回退到有效页
watch(filtered, () => {
  const maxPage = Math.max(1, Math.ceil(filtered.value.length / pageSize.value))
  if (page.value > maxPage) page.value = maxPage
})

// alias map 扁平化为标签列表（tooltip 显示所属源）
const aliasList = (row: CatalogEntryView) => {
  const out: Array<{ key: string; value: string }> = []
  for (const [key, names] of Object.entries(row.alias ?? {})) {
    for (const n of names ?? []) out.push({ key, value: n })
  }
  return out
}

const load = async () => {
  const res = await getCatalogListApi()
  rows.value = res.data.entries ?? []
  hasExternal.value = res.data.has_external
  builtInPath.value = res.data.built_in_path ?? ''
  const maxPage = Math.max(1, Math.ceil(rows.value.length / pageSize.value))
  if (page.value > maxPage) page.value = maxPage
}

// ---- 弹窗 ----

// 常用别名源（出厂条目实际出现的键）；条目里的其他源键动态追加展示
const FIXED_ALIAS_SOURCES = ['goby', 'ehole', 'fingerprinthub', 'fingers']
const aliasSources = FIXED_ALIAS_SOURCES

const dlg = reactive({
  visible: false,
  isAdd: false,
  saving: false,
  original: '', // 编辑定位名（可与 form.name 不同实现改名）
  origin: 'built-in' as 'built-in' | 'external',
  target: 'external' as 'built-in' | 'external',
  builtInPath: ''
})

const form = reactive({
  name: '',
  vendor: '',
  product: '',
  pocs: [] as string[]
})

// 别名编辑形态：源 → 写法文本（一行一个）
const aliasText = reactive<Record<string, string>>({})
const aliasSourceRows = ref<string[]>([])
const aliasCandidate = ref('') // 候选辅助当前选中写法
const aliasCandidateSource = ref('goby') // 候选写法的目标源

// 汇总别名区 → 请求 payload 的 alias map（空源不写）
const collectAlias = (): Record<string, string[]> => {
  const out: Record<string, string[]> = {}
  for (const s of aliasSourceRows.value) {
    const names = (aliasText[s] ?? '')
      .split('\n')
      .map((x) => x.trim())
      .filter(Boolean)
    if (names.length) out[s] = names
  }
  return out
}

const fillAlias = (alias: Record<string, string[]>) => {
  // 清掉旧源行再填充（弹窗复用同一 reactive，残留会串条目）
  for (const s of Object.keys(aliasText)) delete aliasText[s]
  const sources = new Set([...Object.keys(alias ?? {}), ...FIXED_ALIAS_SOURCES])
  aliasSourceRows.value = [...sources]
  for (const s of aliasSourceRows.value) {
    aliasText[s] = (alias?.[s] ?? []).join('\n')
  }
}

const openAdd = () => {
  dlg.isAdd = true
  dlg.original = ''
  dlg.origin = 'external'
  dlg.target = 'external'
  form.name = ''
  form.vendor = ''
  form.product = ''
  form.pocs = []
  fillAlias({})
  aliasCandidate.value = ''
  aliasCandidateSource.value = 'goby'
  dlg.visible = true
}

const openEdit = (row: CatalogEntryView) => {
  dlg.isAdd = false
  dlg.original = row.name
  dlg.origin = row.origin
  // 出厂条目默认写回内置源（BL-POC-008 先例：内置来源行默认内置目标）；外置条目仅外置
  dlg.target = row.origin === 'built-in' ? 'built-in' : 'external'
  dlg.builtInPath = builtInPath.value
  form.name = row.name
  form.vendor = row.vendor ?? ''
  form.product = row.product ?? ''
  form.pocs = [...(row.pocs ?? [])]
  fillAlias(row.alias ?? {})
  aliasCandidate.value = ''
  aliasCandidateSource.value = 'goby'
  dlg.visible = true
}

// 候选辅助：选中写法追加到目标源文本末尾
const addAliasFromCandidate = () => {
  const v = aliasCandidate.value.trim()
  if (!v) return ElMessage.warning('请先选择一个指纹写法')
  const s = aliasCandidateSource.value
  const cur = (aliasText[s] ?? '').split('\n').map((x) => x.trim()).filter(Boolean)
  if (cur.includes(v)) return ElMessage.warning(`「${v}」已在 ${s} 源别名中`)
  aliasText[s] = [...cur, v].join('\n')
  aliasCandidate.value = ''
}

const removeAliasSource = (s: string) => {
  aliasSourceRows.value = aliasSourceRows.value.filter((x) => x !== s)
  delete aliasText[s]
}

// 防死条目前端守卫（后端同款铁律）：别名区全空且标准名不在指纹输出名全集 → 拒绝提示
const emptyAliasGuard = (): boolean => {
  if (Object.keys(collectAlias()).length > 0) return true
  if (productOptions.value.some((o) => o.value === form.name.trim())) return true // 自标准名形态放行
  return false
}

const saveEntry = async () => {
  const name = form.name.trim()
  if (!name) return ElMessage.warning('标准名不能为空')
  if (!emptyAliasGuard()) {
    return ElMessage.warning('别名区为空且「' + name + '」不在指纹输出名全集——识别引擎永远报不出这个名字，绑定不会命中。请至少挂一条真实写法做别名')
  }
  const payload = {
    name,
    vendor: form.vendor.trim(),
    product: form.product.trim(),
    alias: collectAlias(),
    pocs: form.pocs
  }
  dlg.saving = true
  try {
    if (dlg.isAdd) {
      await catalogAddApi(payload, dlg.target)
      ElMessage.success(dlg.target === 'built-in' ? '已写入代码源文件（下次 go build 生效）' : '已保存到外挂文件，重跑扫描器生效')
    } else {
      const res = await catalogEditApi(dlg.original, dlg.target, payload)
      ElMessage.success(dlg.target === 'built-in' ? '已写入代码源文件（下次 go build 生效）' : '已保存到外挂文件，重跑扫描器生效')
      if (res.data?.hint) ElMessage.info(res.data.hint)
    }
    dlg.visible = false
    await load()
  } catch {
    // 错误 toast 由请求拦截器 onError 统一展示（含服务端守卫 message）
  } finally {
    dlg.saving = false
  }
}

const removeRow = async (row: CatalogEntryView) => {
  const msg =
    row.origin === 'built-in'
      ? `删除代码内置条目「${row.name}」？绑定将失效（不再发该产品的测试包），但别名归一保留（识别显示不受影响）。`
      : `删除外挂条目「${row.name}」？若代码里有同名内置定义，将回落到内置定义。`
  try {
    await ElMessageBox.confirm(msg, '确认', { type: 'warning' })
  } catch {
    return
  }
  try {
    await catalogDeleteApi(row.name)
    ElMessage.success('已删除')
    await load()
  } catch {
    await load()
  }
}

// ---- 下拉候选（el-select-v2 options 形态；两数据源近万项）----

interface SelectOption {
  label: string
  value: string
}

const productOptions = ref<SelectOption[]>([])
const pocOptions = ref<SelectOption[]>([])

onMounted(async () => {
  await load()
  try {
    const [prodsRes, pocsRes] = await Promise.all([listProductsApi(), listPocsApi()])
    productOptions.value = (prodsRes.data.products ?? []).map((n) => ({ label: n, value: n }))
    pocOptions.value = (pocsRes.data.pocs ?? []).map((p) => ({ label: p.name, value: p.name }))
  } catch {
    // 候选加载失败不阻塞列表；错误 toast 由拦截器统一展示
  }
})
</script>

<style scoped>
.catalog-page {
  padding: 12px;
}
.card-head {
  display: flex;
  justify-content: space-between;
  align-items: center;
}
.head-actions {
  display: flex;
  gap: 8px;
  align-items: center;
}
.alias-tag {
  margin: 1px 4px 1px 0;
}
.poc-tag {
  margin: 1px 4px 1px 0;
}
.empty-cell {
  color: var(--el-text-color-secondary);
}

/* 弹窗区块小标题（别名区/测试名单分组） */
.dlg-section {
  display: flex;
  align-items: center;
  gap: 4px;
  font-size: 13px;
  font-weight: 500;
  color: var(--el-text-color-primary);
  margin: 12px 0 4px;
  padding-left: 68px;
}
/* 页标题行 + 悬浮提示问号图标 */
.head-title {
  display: inline-flex;
  align-items: center;
  gap: 4px;
}
.help-icon {
  color: var(--el-text-color-secondary);
  cursor: help;
}

/* 弹窗字段行：68px 右对齐 label + 控件随行（与 Binds/POC 页排版纪律一致） */
.dlg-field {
  display: flex;
  align-items: flex-start;
  gap: 8px;
  margin: 8px 0;
}
.dlg-field > label {
  flex-shrink: 0;
  width: 60px;
  text-align: right;
  font-size: 12px;
  line-height: 24px;
  color: var(--el-text-color-regular);
}
.dlg-hint {
  font-size: 12px;
  color: var(--el-text-color-secondary);
  margin: 2px 0 4px 68px;
}
.mono :deep(textarea),
.mono :deep(input) {
  font-family: Consolas, monospace;
  font-size: 12px;
}
</style>
