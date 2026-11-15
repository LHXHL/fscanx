<template>
  <div class="binds-page">
    <el-card shadow="never">
      <template #header>
        <div class="card-head">
          <span>指纹 → POC 绑定关系（{{ rows.length }} 条{{ hasExternal ? '，含外置覆盖' : '，纯内置种子' }}）</span>
          <div>
            <el-button size="small" @click="addRow">+ 添加绑定</el-button>
            <el-button type="primary" size="small" :loading="saving" @click="save">保存到外置 binds.yaml</el-button>
          </div>
        </div>
      </template>

      <el-alert
        type="info"
        :closable="false"
        show-icon
        title="product 与内置种子同名则整条覆盖；POC 从下拉点选（内置+外置全集，可输入过滤）；无条件全打型（不依赖指纹）在 POC 编辑器的 always 开关设置。保存后重跑扫描器生效。"
        style="margin-bottom: 10px"
      />

      <el-table :data="rows" size="small" border>
        <el-table-column label="产品名（指纹）" min-width="200">
          <template #default="scope">
            <el-autocomplete
              v-model="scope.row.product"
              :fetch-suggestions="queryProducts"
              placeholder="输入或从下拉选择产品名"
              style="width: 100%"
            />
          </template>
        </el-table-column>
        <el-table-column label="POC 列表（点选）" min-width="360">
          <template #default="scope">
            <el-select
              v-model="scope.row.pocs"
              multiple
              filterable
              collapse-tags
              collapse-tags-tooltip
              :reserve-keyword="false"
              placeholder="点击选择 POC"
              style="width: 100%"
            >
              <el-option v-for="n in allPocNames" :key="n" :label="n" :value="n" />
            </el-select>
          </template>
        </el-table-column>
        <el-table-column label="" width="60" align="center">
          <template #default="scope">
            <el-button link type="danger" size="small" @click="rows.splice(scope.$index, 1)">删</el-button>
          </template>
        </el-table-column>
      </el-table>
    </el-card>
  </div>
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { ElMessage } from 'element-plus'
import { getBindsApi, listProductsApi, putBindsApi, type Bind } from '@/api/binds'
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

const queryProducts = (queryString: string, cb: (results: { value: string }[]) => void) => {
  const q = queryString.trim().toLowerCase()
  const hits = products.value.filter((p) => p.toLowerCase().includes(q)).slice(0, 20)
  cb(hits.map((value) => ({ value })))
}

const addRow = () => {
  rows.value.push({ product: '', pocs: [] })
}

const load = async () => {
  const [bindsRes, prodsRes, pocsRes] = await Promise.all([getBindsApi(), listProductsApi(), listPocsApi()])
  hasExternal.value = bindsRes.data.hasExternal
  products.value = prodsRes.data.products ?? []
  allPocNames.value = (pocsRes.data.pocs ?? []).map((p) => p.name)
  rows.value = (bindsRes.data.binds ?? []).map((b: Bind) => ({
    product: b.product,
    pocs: b.pocs ?? []
  }))
}

const save = async () => {
  const binds: Bind[] = []
  for (const r of rows.value) {
    if (!r.product.trim() || !r.pocs.length) continue
    binds.push({ product: r.product.trim(), pocs: r.pocs })
  }
  if (binds.length !== rows.value.length) {
    ElMessage.warning('有 ' + (rows.value.length - binds.length) + ' 行缺少 product 或 pocs，将不保存这些行')
  }
  saving.value = true
  try {
    const res = await putBindsApi(binds)
    ElMessage.success('已保存：' + res.data.path)
  } catch (e) {
    ElMessage.error(String(e))
  } finally {
    saving.value = false
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
.mono :deep(textarea) {
  font-family: Consolas, monospace;
  font-size: 12px;
}
</style>
