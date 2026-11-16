<template>
  <div class="binds-page">
    <el-card shadow="never">
      <template #header>
        <div class="card-head">
          <span>指纹 → POC 绑定关系（{{ rows.length }} 条{{ hasExternal ? '，含外置覆盖' : '，纯内置种子' }}）</span>
          <el-button type="primary" size="small" @click="openAdd">+ 添加绑定</el-button>
        </div>
      </template>

      <el-alert
        type="info"
        :closable="false"
        show-icon
        title="绑定产品名必须是指纹引擎会输出的名字——候选下拉是指纹库命名空间过滤（精确>前缀>子串排序），写引擎不会输出的名字绑定不会触发。增删改确认即写盘二进制旁 binds.yaml，重跑扫描器生效；无条件全打型（不依赖指纹）在 POC 编辑器 always 开关设置。"
        style="margin-bottom: 10px"
      />

      <!-- 只读展示（BL-POC-005）：浏览态零交互控件，编辑意图走行内按钮进弹窗 -->
      <el-table :data="pagedRows" size="small" border>
        <el-table-column label="产品名（指纹）" min-width="160">
          <template #default="scope">
            <span>{{ scope.row.product }}</span>
          </template>
        </el-table-column>
        <el-table-column label="绑定 POC" min-width="380">
          <template #default="scope">
            <el-tag v-for="p in scope.row.pocs" :key="p" size="small" class="poc-tag">{{ p }}</el-tag>
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
        :total="rows.length"
        :page-sizes="[10, 20, 50]"
        layout="total, sizes, prev, pager, next"
        style="margin-top: 10px; justify-content: flex-end"
      />
    </el-card>

    <!-- 添加/编辑共用弹窗：产品名候选与 POC 多选只在此出现 -->
    <el-dialog v-model="dialogVisible" :title="dialogMode === 'add' ? '添加绑定' : '编辑绑定'" width="560px">
      <div class="dlg-field">
        <label>产品名</label>
        <el-autocomplete
          v-model="form.product"
          :fetch-suggestions="queryProducts"
          placeholder="输入关键字，从指纹库命名空间筛选"
          style="flex: 1"
        />
      </div>
      <div class="dlg-hint">绑定命中 = 扫描指纹出的产品名包含此名字；引擎不会输出的名字绑定不生效</div>
      <div class="dlg-field">
        <label>POC 列表</label>
        <el-select
          v-model="form.pocs"
          multiple
          filterable
          collapse-tags
          collapse-tags-tooltip
          :reserve-keyword="false"
          placeholder="点击选择 POC"
          style="flex: 1"
        >
          <el-option v-for="n in allPocNames" :key="n" :label="n" :value="n" />
        </el-select>
      </div>
      <template #footer>
        <el-button size="small" @click="dialogVisible = false">取消</el-button>
        <el-button type="primary" size="small" :loading="saving" @click="confirmDialog">确认并写盘</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { bindsAddApi, bindsDeleteApi, bindsEditApi, getBindsListApi, listProductsApi } from '@/api/binds'
import { listPocsApi } from '@/api/poc'

defineOptions({ name: 'Binds' })

interface Row {
  product: string
  pocs: string[]
}

const rows = ref<Row[]>([])
const products = ref<string[]>([])
const allPocNames = ref<string[]>([])
const hasExternal = ref(false)
const saving = ref(false)

// 客户端分页（数据量=种子 15 行+用户增量，服务端分页属过度设计）
const page = ref(1)
const pageSize = ref(20)
const pagedRows = computed(() => rows.value.slice((page.value - 1) * pageSize.value, page.value * pageSize.value))

const dialogVisible = ref(false)
const dialogMode = ref<'add' | 'edit'>('add')
const editRow = ref<Row | null>(null)
const form = reactive({ product: '', pocs: [] as string[] })

// 候选=指纹库命名空间过滤（绑定命中前提：引擎会输出该名字）；排序 精确>前缀>子串，贴近输入的名字排前
const queryProducts = (queryString: string, cb: (results: { value: string }[]) => void) => {
  const q = queryString.trim().toLowerCase()
  if (!q) {
    cb(products.value.slice(0, 20).map((value) => ({ value })))
    return
  }
  const exact: string[] = []
  const prefix: string[] = []
  const sub: string[] = []
  for (const p of products.value) {
    const lp = p.toLowerCase()
    if (lp === q) exact.push(p)
    else if (lp.startsWith(q)) prefix.push(p)
    else if (lp.includes(q)) sub.push(p)
  }
  cb([...exact, ...prefix, ...sub].slice(0, 20).map((value) => ({ value })))
}

const load = async () => {
  const [bindsRes, prodsRes, pocsRes] = await Promise.all([getBindsListApi(), listProductsApi(), listPocsApi()])
  hasExternal.value = bindsRes.data.hasExternal
  products.value = prodsRes.data.products ?? []
  allPocNames.value = (pocsRes.data.pocs ?? []).map((p) => p.name)
  rows.value = (bindsRes.data.binds ?? []).map((b) => ({
    product: b.product,
    pocs: b.pocs ?? []
  }))
  // 数据行数变化后当前页可能越界（如删到本页为空），回退到有效页
  const maxPage = Math.max(1, Math.ceil(rows.value.length / pageSize.value))
  if (page.value > maxPage) page.value = maxPage
}

const openAdd = () => {
  dialogMode.value = 'add'
  editRow.value = null
  form.product = ''
  form.pocs = []
  dialogVisible.value = true
}

const openEdit = (row: Row) => {
  dialogMode.value = 'edit'
  editRow.value = row
  form.product = row.product
  form.pocs = [...row.pocs]
  dialogVisible.value = true
}

// 同名产品拦截：与引擎 MergeBinds 按 product 去重语义对齐（同名两条会被后者整条覆盖，界面不允许制造）
const duplicateProduct = (): string => {
  const p = form.product.trim()
  const i = rows.value.findIndex((r) => r.product === p)
  if (i < 0) return ''
  if (dialogMode.value === 'edit' && rows.value[i] === editRow.value) return ''
  return p
}

const confirmDialog = async () => {
  const product = form.product.trim()
  if (!product) return ElMessage.warning('产品名不能为空')
  if (!form.pocs.length) return ElMessage.warning('至少选择一个 POC')
  const dup = duplicateProduct()
  if (dup) return ElMessage.warning(`产品「${dup}」已存在绑定记录，请在其行上点编辑修改`)
  saving.value = true
  try {
    if (dialogMode.value === 'add') {
      // upsert-append 语义：服务端同名兜底合并不丢数据（前端拦截只是更友好的提示）
      await bindsAddApi(product, form.pocs)
    } else if (editRow.value) {
      await bindsEditApi(editRow.value.product, product, form.pocs)
    }
    ElMessage.success('已保存到 binds.yaml，重跑扫描器生效')
    dialogVisible.value = false
    await load()
  } catch {
    // 错误 toast 由请求拦截器 onError 统一展示（含服务端 message），此处不重复弹
  } finally {
    saving.value = false
  }
}

const removeRow = async (row: Row) => {
  try {
    await ElMessageBox.confirm(`删除产品「${row.product}」的绑定（${row.pocs.length} 个 POC）？`, '确认', { type: 'warning' })
  } catch {
    return
  }
  try {
    await bindsDeleteApi(row.product)
    ElMessage.success('已删除并写盘 binds.yaml，重跑扫描器生效')
    await load()
  } catch {
    // 同上：拦截器 onError 已展示服务端错误消息
    await load()
  }
}

onMounted(load)
</script>

<style scoped>
.binds-page {
  padding: 12px;
}
.card-head {
  display: flex;
  justify-content: space-between;
  align-items: center;
}
.poc-tag {
  margin: 1px 4px 1px 0;
}

/* 弹窗字段行：60px 右对齐 label + 控件随行（与 POC 编辑器排版纪律一致） */
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
</style>
