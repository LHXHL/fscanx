<template>
  <div class="poc-page">
    <!-- 左：POC 列表 -->
    <el-card class="list-card" shadow="never">
      <template #header>
        <div class="card-head">
          <span>POC 库（{{ filtered.length }}）</span>
          <el-button type="primary" size="small" @click="newPoc">新建</el-button>
        </div>
      </template>
      <el-input v-model="keyword" placeholder="搜索名称..." clearable size="small" style="margin-bottom: 8px" />
      <el-table :data="filtered" highlight-current-row height="620" size="small" @row-click="(row: PocItem) => openPoc(row.name)">
        <el-table-column prop="name" label="名称" show-overflow-tooltip />
        <el-table-column label="来源" width="110" prop="source" :formatter="(_: unknown, __: unknown, row: PocItem) => (row.source === 'external' ? '外置' : '内置') + (row.always ? '·always' : '')" />
      </el-table>
    </el-card>

    <!-- 右：编辑器 -->
    <el-card class="edit-card" shadow="never">
      <template #header>
        <div class="card-head">
          <span>编辑器{{ selName ? `：${selName}` : '' }}</span>
          <el-radio-group v-model="mode" size="small">
            <el-radio-button value="form">表单模式</el-radio-button>
            <el-radio-button value="raw">源码模式</el-radio-button>
          </el-radio-group>
        </div>
      </template>

      <div v-show="mode === 'form'">
        <!-- 基本 -->
        <div class="form-grid">
          <div class="field grow">
            <label>名称</label>
            <el-input v-model="form.name" size="small" placeholder="如 poc-yaml-xxx-rce（poc-yaml- 前缀）" />
          </div>
          <div class="field">
            <label>无条件扫描</label>
            <el-switch v-model="form.always" size="small" />
          </div>
        </div>
        <div class="hint">always：不依赖指纹命中，空 -pocname 智能定向时也执行（如 .git 目录探测）</div>

        <!-- 变量（set/sets） -->
        <el-collapse style="margin: 4px 0 10px">
          <el-collapse-item :title="`变量 set（${setRows.length}）/ sets（${setListRows.length}）`" name="vars">
            <div class="vars-grid">
              <div>
                <div class="vars-head">
                  <span>set：单值变量（值是表达式，如 randomInt(1,9999)）</span>
                  <el-button size="small" @click="setRows.push({ key: '', value: '' })">+ 变量</el-button>
                </div>
                <div v-for="(kv, i) in setRows" :key="i" class="cond-row">
                  <el-input v-model="kv.key" size="small" placeholder="变量名" style="width: 180px" />
                  <el-input v-model="kv.value" size="small" placeholder="randomInt(1, 9999) / newReverse() / 字符串" style="flex: 1" />
                  <el-button link type="danger" size="small" @click="setRows.splice(i, 1)">删</el-button>
                </div>
              </div>
              <div>
                <div class="vars-head">
                  <span>sets：列表变量（值一行一个）</span>
                  <el-button size="small" @click="setListRows.push({ key: '', valuesText: '' })">+ 变量</el-button>
                </div>
                <div v-for="(kv, i) in setListRows" :key="i" class="cond-row">
                  <el-input v-model="kv.key" size="small" placeholder="变量名" style="width: 180px" />
                  <el-input v-model="kv.valuesText" size="small" type="textarea" :rows="2" placeholder="一行一个值" style="flex: 1" />
                  <el-button link type="danger" size="small" @click="setListRows.splice(i, 1)">删</el-button>
                </div>
              </div>
            </div>
          </el-collapse-item>
        </el-collapse>

        <!-- 步骤（多步利用链，顺序执行） -->
        <div class="section-title">
          <span>请求步骤（{{ form.steps.length }} 步）<span class="sub">｜按序执行：任一步不满足即停止、整体不命中；每步都满足才算验证成功</span></span>
          <el-button type="primary" size="small" @click="addStep">+ 添加步骤</el-button>
        </div>
        <el-tabs v-model="activeStep" type="card" closable @tab-remove="removeStep">
          <el-tab-pane v-for="(st, i) in form.steps" :key="i" :name="String(i)" :label="`步骤 ${i + 1}`">

          <div class="form-grid">
            <div class="field">
              <label>请求</label>
              <el-select v-model="st.method" size="small" style="width: 92px">
                <el-option value="GET" />
                <el-option value="POST" />
                <el-option value="PUT" />
                <el-option value="DELETE" />
                <el-option value="HEAD" />
              </el-select>
            </div>
            <div class="field grow">
              <label>路径</label>
              <el-input v-model="st.path" size="small" placeholder="请求路径，可引用变量 {{rand}}" />
            </div>
          </div>
          <div class="form-grid">
            <div class="field">
              <label>跟随跳转</label>
              <el-switch v-model="st.followRedirects" size="small" />
            </div>
            <div class="field" v-if="isFuzzMode">
              <label>遍历输出</label>
              <el-switch v-model="st.cont" size="small" />
            </div>
          </div>
          <div v-if="isFuzzMode" class="hint">continue（爆破模式专用）：组合命中后继续尝试其余组合并逐条输出</div>

          <!-- headers -->
          <div class="section-title">
            <span>Headers<template v-if="st.headers.length">（{{ st.headers.length }}）</template></span>
            <el-button size="small" @click="st.headers.push({ k: '', v: '' })">+ Header</el-button>
          </div>
          <div v-for="(h, j) in st.headers" :key="j" class="cond-row">
            <el-input v-model="h.k" size="small" placeholder="如 Content-Type" style="width: 220px" />
            <el-input v-model="h.v" size="small" placeholder="值" style="flex: 1" />
            <el-button link type="danger" size="small" @click="st.headers.splice(j, 1)">删</el-button>
          </div>

          <!-- body / search -->
          <div v-if="st.method !== 'GET' && st.method !== 'HEAD'" class="field field-block">
            <label>请求体</label>
            <el-input v-model="st.body" size="small" type="textarea" :rows="2" placeholder="POST body，可引用变量 {{rand}}" class="mono" />
          </div>
          <div class="field field-block">
            <label>变量提取</label>
            <el-input v-model="st.search" size="small" placeholder="search 正则：提取变量供后续步骤引用；留空跳过，填了但提取不到则该步即失败" />
          </div>

          <!-- 匹配条件 -->
          <div class="section-title">
            <span>匹配条件</span>
            <el-tag v-if="st.advanced" type="warning" size="small">超出表单子集，CEL 文本编辑</el-tag>
          </div>
          <div v-if="st.advanced" class="cond-row">
            <el-input v-model="st.expression" size="small" type="textarea" :rows="3" spellcheck="false" class="mono" placeholder="CEL 表达式原文" />
          </div>
          <template v-else>
            <CondTree :node="st.tree" @remove="resetTree(i)" />
            <el-input
              :model-value="nodeToCel(st.tree) || '（条件不完整）'"
              size="small"
              type="textarea"
              :rows="2"
              readonly
              class="cel-preview"
              style="margin-top: 4px"
            />
          </template>
          </el-tab-pane>
        </el-tabs>

        <div style="margin-top: 12px">
          <el-button size="small" :loading="checking" @click="checkCel">校验当前页步骤表达式</el-button>
        </div>
      </div>

      <div v-show="mode === 'raw'">
        <el-alert
          v-if="!formLoaded || rawNote"
          type="info"
          :closable="false"
          show-icon
          :title="rawNote || '整份 yaml 源码编辑，保存前自动做结构解析与 CEL 编译校验'"
          style="margin-bottom: 8px"
        />
        <el-input v-model="rawYaml" type="textarea" :rows="26" spellcheck="false" class="mono" />
      </div>

      <div style="margin-top: 12px">
        <el-button type="primary" :loading="saving" @click="save">保存到外置 pocs/</el-button>
        <el-button type="danger" plain :disabled="!selName" @click="remove">删除外置</el-button>
      </div>
    </el-card>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import {
  deletePocApi,
  getPocApi,
  listPocsApi,
  putPocApi,
  validateCelApi,
  type CondNode,
  type PocItem
} from '@/api/poc'
import CondTree from './CondTree.vue'
import { nodeToCel, pruneTree } from './predicates'

defineOptions({ name: 'Poc' })

// 步骤行模型（headers 平铺为可编辑行；advanced=超出谓词子集走 CEL 文本）
interface StepRow {
  method: string
  path: string
  headers: Array<{ k: string; v: string }>
  body: string
  search: string
  followRedirects: boolean
  cont: boolean
  tree: CondNode
  expression: string
  advanced: boolean
}

const keyword = ref('')
const pocs = ref<PocItem[]>([])
const selName = ref('')
const mode = ref<'form' | 'raw'>('form')
const rawYaml = ref('')
const baseYaml = ref('') // 打开时原文：表单保存时 detail/groups 等未暴露段保真继承
const formLoaded = ref(false)
const saving = ref(false)
const checking = ref(false)
const rawNote = ref('')

const form = reactive({
  name: '',
  always: false,
  steps: [] as StepRow[]
})
const setRows = ref<Array<{ key: string; value: string }>>([])
const setListRows = ref<Array<{ key: string; valuesText: string }>>([])

const filtered = computed(() => pocs.value.filter((p) => p.name.includes(keyword.value)))

// 爆破模式（sets 列表变量存在时）：执行走 clusterpoc 笛卡尔积路径，continue 字段仅此模式生效
const isFuzzMode = computed(() => setListRows.value.some((kv) => kv.key.trim() !== ''))

const newTree = (): CondNode => ({
  logic: 'and',
  conditions: [{ target: 'status', op: 'eq', key: '', value: '200' }]
})

const activeStep = ref('0')

const addStep = () => {
  form.steps.push({
    method: 'GET',
    path: '/',
    headers: [],
    body: '',
    search: '',
    followRedirects: false,
    cont: false,
    tree: newTree(),
    expression: '',
    advanced: false
  })
  activeStep.value = String(form.steps.length - 1) // 新增后跳到新页签
}

// 页签 × 删除步骤（至少保留一步；删除后修正激活页使其跟随原内容）
const removeStep = (name: string | number) => {
  const i = Number(name)
  if (form.steps.length <= 1) {
    ElMessage.warning('至少保留一个请求步骤')
    return
  }
  form.steps.splice(i, 1)
  const cur = Number(activeStep.value)
  activeStep.value = String(i < cur ? cur - 1 : Math.min(cur, form.steps.length - 1))
}

// 顶层条件树只剩空组被"删组"时重置为初始单条件（步骤不能没有条件）
const resetTree = (i: number) => {
  form.steps[i].tree = newTree()
}

const loadList = async () => {
  const res = await listPocsApi()
  pocs.value = res.data.pocs ?? []
}

const openPoc = async (name: string) => {
  try {
    const res = await getPocApi(name)
    selName.value = name
    rawYaml.value = res.data.yaml
    baseYaml.value = res.data.yaml
    formLoaded.value = true
    form.name = res.data.name
    form.always = res.data.always
    setRows.value = (res.data.set ?? []).map((kv) => ({ key: kv.key, value: kv.value }))
    setListRows.value = (res.data.sets ?? []).map((kv) => ({ key: kv.key, valuesText: (kv.values ?? []).join('\n') }))
    activeStep.value = '0'
    form.steps = (res.data.steps ?? []).map((s) => ({
      method: s.method || 'GET',
      path: s.path,
      headers: Object.entries(s.headers ?? {}).map(([k, v]) => ({ k, v })),
      body: s.body,
      search: s.search,
      followRedirects: s.followRedirects,
      cont: s.continue,
      tree: s.tree ?? newTree(),
      expression: s.expression,
      advanced: s.tree === null
    }))
    if (!res.data.form) {
      mode.value = 'raw'
      rawNote.value = '该 poc 含 groups 等表单暂不承载的形态，已切源码模式（保存不丢字段）'
      ElMessage.info('该 poc 含 groups 分组形态，表单暂不承载，已切源码模式')
    } else {
      rawNote.value = ''
      mode.value = 'form'
    }
  } catch (e) {
    ElMessage.error(String(e))
  }
}

const newPoc = () => {
  selName.value = ''
  form.name = 'poc-yaml-'
  form.always = false
  setRows.value = []
  setListRows.value = []
  form.steps = []
  addStep()
  activeStep.value = '0'
  rawYaml.value = ''
  baseYaml.value = ''
  formLoaded.value = true
  rawNote.value = ''
  mode.value = 'form'
}

const checkCel = async () => {
  const st = form.steps[0]
  if (!st) return
  const expr = st.advanced ? st.expression : nodeToCel(st.tree)
  if (!expr) return ElMessage.warning('条件不完整')
  checking.value = true
  try {
    const res = await validateCelApi(expr)
    if (res.data.ok) ElMessage.success('表达式校验通过')
    else ElMessage.error('校验失败：' + res.data.error)
  } catch (e) {
    ElMessage.error(String(e))
  } finally {
    checking.value = false
  }
}

const save = async () => {
  if (!form.name.trim()) return ElMessage.warning('name 不能为空')
  saving.value = true
  try {
    if (mode.value === 'form') {
      if (!form.steps.length) return ElMessage.warning('至少一个请求步骤')
      const steps = form.steps.map((st) => {
        const headers: Record<string, string> = {}
        for (const h of st.headers) {
          if (h.k.trim()) headers[h.k.trim()] = h.v
        }
        const base = {
          method: st.method,
          path: st.path,
          headers,
          body: st.body,
          search: st.search,
          followRedirects: st.followRedirects,
          continue: st.cont
        }
        return st.advanced ? { ...base, expression: st.expression } : { ...base, tree: pruneTree(st.tree) ?? undefined }
      })
      const payload = {
        mode: 'form' as const,
        name: form.name.trim(),
        always: form.always,
        set: setRows.value.filter((kv) => kv.key.trim()).map((kv) => ({ key: kv.key.trim(), value: kv.value })),
        sets: setListRows.value
          .filter((kv) => kv.key.trim())
          .map((kv) => ({ key: kv.key.trim(), values: kv.valuesText.split('\n').map((x) => x.trim()).filter(Boolean) })),
        steps,
        baseYaml: baseYaml.value
      }
      const res = await putPocApi(payload)
      ElMessage.success('已保存：' + res.data.file)
      selName.value = form.name.trim()
      await loadList()
      // 重新打开刷新 baseYaml（渲染产物成为新的保真基底）
      await openPoc(selName.value)
    } else {
      const res = await putPocApi({ mode: 'raw', name: form.name.trim() || selName.value, yaml: rawYaml.value })
      ElMessage.success('已保存：' + res.data.file)
      selName.value = form.name.trim() || selName.value
      await loadList()
      await openPoc(selName.value)
    }
  } catch (e) {
    ElMessage.error(String(e))
  } finally {
    saving.value = false
  }
}

const remove = async () => {
  try {
    await ElMessageBox.confirm(`删除外置 poc「${selName.value}」？`, '确认', { type: 'warning' })
  } catch {
    return
  }
  try {
    await deletePocApi(selName.value)
    ElMessage.success('已删除')
    selName.value = ''
    await loadList()
  } catch (e) {
    ElMessage.error(String(e))
  }
}

onMounted(loadList)
</script>

<style scoped>
/* 排版纪律（BL-POC-004 样式轮）：控件统一 small（12px）；区块标题 13px/500；
   行标签 60px 右对齐单列——所有行左基线一致；说明文字缩进到控件起点；
   垂直节奏 4px 基数。 */
.poc-page {
  display: flex;
  gap: 12px;
  padding: 12px;
  height: calc(100vh - 110px);
}
.list-card {
  width: 380px;
  flex-shrink: 0;
}
.edit-card {
  flex: 1;
  overflow: auto;
}
.card-head {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

/* 统一字段行：label 列 60px 右对齐，控件随行 */
.form-grid {
  display: flex;
  flex-wrap: wrap;
  gap: 6px 18px;
  align-items: center;
  margin: 4px 0;
}
.field {
  display: flex;
  align-items: center;
  gap: 8px;
  min-width: 0;
}
.field > label {
  flex-shrink: 0;
  width: 60px;
  text-align: right;
  font-size: 12px;
  color: var(--el-text-color-regular);
}
.field.grow {
  flex: 1;
}
.field.grow :deep(.el-input),
.field.grow :deep(.el-select) {
  flex: 1;
}
.field-block {
  margin: 4px 0;
}
.field-block :deep(.el-input) {
  flex: 1;
}

/* 区块标题：13px/500 主色 + 右侧动作按钮 */
.section-title {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin: 10px 0 6px;
  font-size: 13px;
  font-weight: 500;
  color: var(--el-text-color-primary);
}
.section-title .sub {
  font-size: 12px;
  font-weight: 400;
  color: var(--el-text-color-secondary);
}
.vars-head {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin: 10px 0 4px;
  font-size: 13px;
  font-weight: 500;
  color: var(--el-text-color-primary);
}
.vars-grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 16px;
}

/* 说明文字：缩进对齐控件起点（60px label + 8px gap） */
.hint {
  font-size: 12px;
  color: var(--el-text-color-secondary);
  margin: 2px 0 4px 68px;
}

/* 行内编辑行（headers/条件/变量） */
.cond-row {
  display: flex;
  align-items: center;
  gap: 6px;
  margin: 4px 0;
}

/* 步骤页签与折叠面板密度对齐 small 表单 */
:deep(.el-tabs__item) {
  height: 34px;
  line-height: 34px;
  font-size: 13px;
}
:deep(.el-tabs__content) {
  overflow: visible;
}
:deep(.el-collapse-item__header) {
  font-size: 13px;
  height: 36px;
  line-height: 36px;
}

.mono :deep(textarea) {
  font-family: Consolas, monospace;
  font-size: 12px;
}
.cel-preview :deep(textarea) {
  font-family: Consolas, monospace;
  color: var(--el-color-success);
}
</style>
