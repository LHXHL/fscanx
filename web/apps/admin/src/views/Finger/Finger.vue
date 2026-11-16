<template>
  <div class="finger-page">
    <el-card shadow="never">
      <template #header>
        <div class="card-head">
          <span>指纹库（{{ file === 'goby' ? 'goby.json' : 'fingers_http.json' }}｜{{ total }} 条）</span>
          <div class="head-actions">
            <el-select v-model="file" size="small" style="width: 200px" @change="reload">
              <el-option label="goby.json（文本指纹）" value="goby" />
              <el-option label="fingers_http.json（hash 指纹）" value="fingers_http" />
            </el-select>
            <el-input v-model="keyword" size="small" placeholder="搜索名称..." clearable style="width: 200px" @keyup.enter="reload" @clear="reload" />
            <el-button size="small" @click="reload" :loading="loading">搜索</el-button>
            <el-button type="primary" size="small" @click="openAdd">新增条目</el-button>
            <el-button type="success" size="small" @click="pack" :loading="packing">一键重打包</el-button>
          </div>
        </div>
      </template>

      <el-alert
        type="info"
        :closable="false"
        show-icon
        title="条目化编辑：单条增删改复制，未动条目字节零漂移；改动落明文快照 → 一键重打包 → 重新编译 fscanx 生效（指纹保持编译嵌入）。手改明文 JSON 会被自动吸收进库。"
        style="margin-bottom: 10px"
      />

      <el-table :data="items" highlight-current-row height="520" size="small" @row-click="(row: FingerEntryItem) => openEdit(row)">
        <el-table-column prop="id" label="ID" width="70" />
        <el-table-column prop="name" label="名称" show-overflow-tooltip />
        <el-table-column label="操作" width="190">
          <template #default="scope">
            <el-button size="small" @click.stop="openEdit(scope.row)">编辑</el-button>
            <el-button size="small" @click.stop="copyEntry(scope.row)">复制</el-button>
            <el-button size="small" type="danger" plain @click.stop="removeEntry(scope.row)">删除</el-button>
          </template>
        </el-table-column>
      </el-table>
      <el-pagination
        v-model:current-page="page"
        v-model:page-size="size"
        :total="total"
        :page-sizes="[10, 20, 50]"
        layout="total, sizes, prev, pager, next"
        size="small"
        style="margin-top: 10px"
        @current-change="load"
        @size-change="reload"
      />

      <el-alert v-if="packLines.length" type="success" :closable="true" show-icon title="重打包完成" style="margin-top: 10px">
        <pre class="pack-out">{{ packLines.join('\n') }}</pre>
      </el-alert>
    </el-card>

    <!-- 条目编辑/新增弹窗：表单模式（schema 内）与原文模式（超 schema 降级/兜底） -->
    <el-dialog v-model="dlg.visible" :title="dlg.isAdd ? '新增条目' : `编辑条目 #${dlg.id}`" width="720px">
      <el-alert
        v-if="!dlg.isAdd && !dlg.schemaOk"
        type="warning"
        :closable="false"
        show-icon
        title="该条目含表单 schema 外字段或形态（如 default_port/非 favicon 规则），已锁定原文模式编辑——字段零丢失"
        style="margin-bottom: 8px"
      />
      <div class="dlg-field">
        <label>编辑模式</label>
        <el-radio-group v-model="dlg.mode" size="small" :disabled="dlg.isAdd || !dlg.schemaOk">
          <el-radio-button value="form">表单模式</el-radio-button>
          <el-radio-button value="raw">原文模式</el-radio-button>
        </el-radio-group>
        <span v-if="dlg.isAdd" class="sub">新增走表单结构化；原文形态条目请复制相近条目后切原文编辑</span>
      </div>

      <div v-if="dlg.mode === 'form'" v-show="dlg.schemaOk || dlg.isAdd">
        <template v-if="file === 'goby'">
          <div class="dlg-field">
            <label>名称</label>
            <el-input v-model="gobyForm.name" size="small" placeholder="产品名（如 Nginx）" style="flex: 1" />
          </div>
          <div class="dlg-field">
            <label>特别关注</label>
            <el-switch v-model="gobyForm.focus" size="small" />
            <span class="sub">focus：命中时输出 [*产品名] 高亮</span>
          </div>
          <div class="dlg-field">
            <label>logic</label>
            <el-input v-model="gobyForm.logic" size="small" placeholder="label 布尔组合，如 a||b 或 (a&&b)||c" style="flex: 1" class="mono" />
          </div>
          <div class="dlg-field">
            <label>规则</label>
            <el-button size="small" @click="gobyForm.rules.push({ label: '', feature: '', is_equal: true })">+ 规则</el-button>
          </div>
          <div v-for="(r, i) in gobyForm.rules" :key="i" class="dlg-field">
            <label>{{ String.fromCharCode(97 + i) }}</label>
            <el-input v-model="r.label" size="small" placeholder="label" style="width: 90px" />
            <el-input v-model="r.feature" size="small" placeholder="特征（响应中包含的子串）" style="flex: 1" class="mono" />
            <el-switch v-model="r.is_equal" size="small" active-text="全等" />
            <el-button link type="danger" size="small" @click="gobyForm.rules.splice(i, 1)">删</el-button>
          </div>
          <div class="hint">is_equal=false 为排除条件（响应含该特征则不命中）</div>
        </template>
        <template v-else>
          <div class="dlg-field">
            <label>名称</label>
            <el-input v-model="fingersForm.name" size="small" placeholder="产品名" style="flex: 1" />
          </div>
          <div class="dlg-field">
            <label>特别关注</label>
            <el-switch v-model="fingersForm.focus" size="small" />
          </div>
          <div class="dlg-field">
            <label>link</label>
            <el-input v-model="fingersForm.link" size="small" placeholder="参考链接（可空）" style="flex: 1" />
          </div>
          <div class="dlg-field">
            <label>tag</label>
            <el-input v-model="fingersForm.tagText" size="small" type="textarea" :rows="2" placeholder="一行一个标签（如 cloud）" style="flex: 1" />
          </div>
          <div class="dlg-field">
            <label>favicon md5</label>
            <el-input v-model="fingersForm.md5" size="small" placeholder="图标 md5（icon_hash.html 计算）" style="flex: 1" class="mono" />
          </div>
          <div class="dlg-field">
            <label>favicon mmh3</label>
            <el-input v-model="fingersForm.mmh3Text" size="small" type="textarea" :rows="2" placeholder="一行一个 mmh3 值（可为负）" style="flex: 1" class="mono" />
          </div>
        </template>
      </div>
      <div v-show="dlg.mode === 'raw'">
        <div class="hint" style="margin-left: 0; margin-bottom: 4px">整条 JSON 原文编辑（内容即所写；保存校验 JSON 对象 + name 非空）</div>
        <el-input v-model="dlg.rawText" type="textarea" :rows="16" spellcheck="false" class="mono" />
      </div>
      <template #footer>
        <el-button size="small" @click="dlg.visible = false">取消</el-button>
        <el-button type="primary" size="small" :loading="dlg.saving" @click="saveEntry">保存</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import {
  fingerAddApi,
  fingerCopyApi,
  fingerDeleteApi,
  fingerEditApi,
  fingerListApi,
  packApi,
  type FingerEntryItem
} from '@/api/finger'

defineOptions({ name: 'Finger' })

const file = ref<'goby' | 'fingers_http'>('goby')
const keyword = ref('')
const page = ref(1)
const size = ref(20)
const total = ref(0)
const items = ref<FingerEntryItem[]>([])
const loading = ref(false)
const packing = ref(false)
const packLines = ref<string[]>([])

const load = async () => {
  loading.value = true
  try {
    const res = await fingerListApi(file.value, keyword.value.trim(), page.value, size.value)
    items.value = res.data.items ?? []
    total.value = res.data.total ?? 0
  } catch (e: any) {
    if (!e?.code) ElMessage.error(String(e)) // API 错误由拦截器统一 toast
  } finally {
    loading.value = false
  }
}

const reload = () => {
  page.value = 1
  load()
}

// ---- 条目 schema 判定：超 schema 字段/形态 → 锁定原文模式（字段零丢失）----

interface GobyRule {
  label: string
  feature: string
  is_equal: boolean
}

const GOBY_KEYS = ['name', 'focus', 'logic', 'rule']
const FINGERS_KEYS = ['focus', 'link', 'name', 'rule', 'tag']

function isSchemaEntry(file: string, obj: any): boolean {
  if (!obj || typeof obj !== 'object' || typeof obj.name !== 'string' || !obj.name) return false
  const keys = Object.keys(obj)
  if (keys.some((k) => !(file === 'goby' ? GOBY_KEYS : FINGERS_KEYS).includes(k))) return false
  if (obj.focus !== undefined && typeof obj.focus !== 'boolean') return false
  if (!Array.isArray(obj.rule)) return false
  if (file === 'goby') {
    if (typeof obj.logic !== 'string') return false
    return obj.rule.every(
      (r: any) =>
        r &&
        typeof r === 'object' &&
        Object.keys(r).every((k) => ['label', 'feature', 'is_equal'].includes(k)) &&
        typeof r.label === 'string' &&
        typeof r.feature === 'string' &&
        typeof r.is_equal === 'boolean'
    )
  }
  if (obj.link !== undefined && typeof obj.link !== 'string') return false
  if (obj.tag !== undefined && !Array.isArray(obj.tag)) return false
  // 含 md5 的规则元素 >1 时表单只回填首个（md5 是单值输入），会丢哈希 → 降级原文模式
  const md5Elems = (obj.rule ?? []).filter((r: any) => r?.favicon?.md5).length
  if (md5Elems > 1) return false
  return obj.rule.every((r: any) => {
    if (!r || typeof r !== 'object' || Object.keys(r).length !== 1 || !r.favicon || typeof r.favicon !== 'object') return false
    const f = r.favicon
    const fk = Object.keys(f)
    if (fk.length === 0 || fk.some((k) => k !== 'md5' && k !== 'mmh3')) return false
    if (f.md5 !== undefined && typeof f.md5 !== 'string') return false
    if (f.mmh3 !== undefined && !Array.isArray(f.mmh3)) return false
    return true
  })
}

// ---- 编辑/新增弹窗 ----

const dlg = reactive({
  visible: false,
  isAdd: false,
  id: 0,
  schemaOk: true,
  mode: 'form' as 'form' | 'raw',
  rawText: '',
  saving: false
})

const gobyForm = reactive({
  name: '',
  focus: false,
  logic: 'a',
  rules: [] as GobyRule[]
})

const fingersForm = reactive({
  name: '',
  focus: false,
  link: '',
  tagText: '',
  md5: '',
  mmh3Text: ''
})

const resetForms = () => {
  gobyForm.name = ''
  gobyForm.focus = false
  gobyForm.logic = 'a'
  gobyForm.rules = [{ label: 'a', feature: '', is_equal: true }]
  fingersForm.name = ''
  fingersForm.focus = false
  fingersForm.link = ''
  fingersForm.tagText = ''
  fingersForm.md5 = ''
  fingersForm.mmh3Text = ''
}

// 原文 → 表单回填（调用前已过 schema 判定）
const fillForm = (obj: any) => {
  if (file.value === 'goby') {
    gobyForm.name = obj.name
    gobyForm.focus = !!obj.focus
    gobyForm.logic = obj.logic ?? 'a'
    gobyForm.rules = (obj.rule ?? []).map((r: GobyRule) => ({ ...r }))
    if (!gobyForm.rules.length) gobyForm.rules = [{ label: 'a', feature: '', is_equal: true }]
  } else {
    fingersForm.name = obj.name
    fingersForm.focus = !!obj.focus
    fingersForm.link = obj.link ?? ''
    fingersForm.tagText = (obj.tag ?? []).join('\n')
    // 多个 favicon 元素：md5 取首个含 md5 的元素，mmh3 汇总所有元素的值
    fingersForm.md5 = (obj.rule ?? []).map((r: any) => r.favicon?.md5 ?? '').find((s: string) => s) ?? ''
    fingersForm.mmh3Text = (obj.rule ?? [])
      .flatMap((r: any) => r.favicon?.mmh3 ?? [])
      .join('\n')
  }
}

const formToEntry = (): Record<string, unknown> => {
  if (file.value === 'goby') {
    const entry: Record<string, unknown> = { name: gobyForm.name.trim() }
    if (gobyForm.focus) entry.focus = true
    entry.logic = gobyForm.logic
    entry.rule = gobyForm.rules
      .filter((r) => r.label.trim() && r.feature)
      .map((r) => ({ label: r.label.trim(), feature: r.feature, is_equal: r.is_equal }))
    return entry
  }
  const entry: Record<string, unknown> = { name: fingersForm.name.trim() }
  if (fingersForm.focus) entry.focus = true
  if (fingersForm.link.trim()) entry.link = fingersForm.link.trim()
  entry.rule = [
    ...(fingersForm.md5.trim() ? [{ favicon: { md5: fingersForm.md5.trim() } }] : []),
    ...(fingersForm.mmh3Text
      .split('\n')
      .map((s) => s.trim())
      .filter(Boolean).length
      ? [
          {
            favicon: {
              mmh3: fingersForm.mmh3Text
                .split('\n')
                .map((s) => s.trim())
                .filter(Boolean)
            }
          }
        ]
      : [])
  ]
  const tags = fingersForm.tagText
    .split('\n')
    .map((s) => s.trim())
    .filter(Boolean)
  if (tags.length) entry.tag = tags
  return entry
}

const openAdd = () => {
  resetForms()
  dlg.isAdd = true
  dlg.id = 0
  dlg.schemaOk = true
  dlg.mode = 'form'
  dlg.rawText = ''
  dlg.visible = true
}

const openEdit = (row: FingerEntryItem) => {
  let obj: any
  try {
    obj = JSON.parse(row.raw)
  } catch {
    obj = null
  }
  dlg.isAdd = false
  dlg.id = row.id
  dlg.schemaOk = isSchemaEntry(file.value, obj)
  dlg.mode = dlg.schemaOk ? 'form' : 'raw'
  dlg.rawText = row.raw
  resetForms()
  if (dlg.schemaOk) fillForm(obj)
  dlg.visible = true
}

const saveEntry = async () => {
  dlg.saving = true
  try {
    if (dlg.isAdd) {
      await fingerAddApi(file.value, formToEntry())
      ElMessage.success('已新增（明文快照已更新，重打包+重编译后生效）')
    } else if (dlg.mode === 'form') {
      await fingerEditApi(file.value, dlg.id, { mode: 'form', entry: formToEntry() })
      ElMessage.success('已保存（明文快照已更新，重打包+重编译后生效）')
    } else {
      await fingerEditApi(file.value, dlg.id, { mode: 'raw', text: dlg.rawText })
      ElMessage.success('已按原文保存（重打包+重编译后生效）')
    }
    dlg.visible = false
    await load()
  } catch (e: any) {
    if (!e?.code) ElMessage.error(String(e)) // API 错误由拦截器统一 toast
  } finally {
    dlg.saving = false
  }
}

const copyEntry = async (row: FingerEntryItem) => {
  try {
    await ElMessageBox.confirm(`复制条目「${row.name}」到库尾？`, '复制', { type: 'info' })
  } catch {
    return
  }
  try {
    await fingerCopyApi(file.value, row.id)
    ElMessage.success('已复制到库尾')
    await load()
  } catch (e: any) {
    if (!e?.code) ElMessage.error(String(e))
  }
}

const removeEntry = async (row: FingerEntryItem) => {
  try {
    await ElMessageBox.confirm(`删除条目「${row.name}」（#ID ${row.id}）？重名条目不受影响。`, '删除', { type: 'warning' })
  } catch {
    return
  }
  try {
    await fingerDeleteApi(file.value, row.id)
    ElMessage.success('已删除（重打包+重编译后从扫描器移除）')
    await load()
  } catch (e: any) {
    if (!e?.code) ElMessage.error(String(e))
  }
}

const pack = async () => {
  packing.value = true
  try {
    const res = await packApi()
    packLines.value = (res.data.results ?? []).map((r) => `${r.src} → ${r.dst}（${r.raw} → ${r.gz} B）`)
    ElMessage.success('重打包完成，回仓库执行 go build 生效')
  } catch (e: any) {
    if (!e?.code) ElMessage.error(String(e))
  } finally {
    packing.value = false
  }
}

onMounted(reload)
</script>

<style scoped>
/* 排版纪律同 POC/Binds 页：small 控件（24px）、60px label 右对齐、13px 区标题、零 console 容忍 */
.finger-page {
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
.mono :deep(textarea),
.mono :deep(input) {
  font-family: Consolas, monospace;
  font-size: 12px;
}
.pack-out {
  margin: 0;
  font-family: Consolas, monospace;
  font-size: 12px;
  white-space: pre-wrap;
}

/* 弹窗字段行：60px 右对齐 label（与 POC 页 dlg-field 纪律一致） */
.dlg-field {
  display: flex;
  align-items: flex-start;
  gap: 8px;
  margin: 8px 0;
}
.dlg-field > label {
  flex-shrink: 0;
  width: 100px;
  text-align: right;
  font-size: 12px;
  line-height: 24px;
  color: var(--el-text-color-regular);
}
.hint {
  font-size: 12px;
  color: var(--el-text-color-secondary);
  margin: 2px 0 4px 108px;
}
.sub {
  font-size: 12px;
  color: var(--el-text-color-secondary);
  line-height: 24px;
}
</style>
