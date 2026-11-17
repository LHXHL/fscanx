<template>
  <div class="finger-page">
    <el-card shadow="never">
      <template #header>
        <div class="card-head">
          <span class="head-title">
            指纹库（{{ file === 'goby' ? 'goby.json' : 'fingers_http.json' }}｜{{ total }} 条）
            <el-tooltip placement="top" popper-class="help-tip">
              <template #content>
                条目化编辑：单条增删改复制，未动条目字节零漂移；改动落明文快照 → 一键重打包 → 重新编译 fscanx 生效（指纹保持编译嵌入）。手改明文 JSON 会被自动吸收进库。
              </template>
              <Icon icon="mdi:help-circle" :size="14" class="help-icon" />
            </el-tooltip>
          </span>
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

      <el-table :data="items" highlight-current-row height="520" size="small" border @row-click="(row: FingerEntryItem) => openEdit(row)">
        <el-table-column prop="id" label="ID" width="70" />
        <el-table-column prop="name" label="名称" show-overflow-tooltip />
        <el-table-column label="产品归属" width="150">
          <template #default="scope">
            <el-tooltip
              v-if="verdictOf(scope.row.name).relation === 'alias-of'"
              :content="`该写法经别名归一后扫描输出为标准名「${verdictOf(scope.row.name).owner}」`"
              placement="top"
            >
              <el-tag type="success" size="small">「{{ verdictOf(scope.row.name).owner }}」的别名</el-tag>
            </el-tooltip>
            <el-tag v-else-if="verdictOf(scope.row.name).relation === 'self'" type="primary" size="small">标准产品</el-tag>
            <el-tooltip v-else-if="verdictOf(scope.row.name).relation === 'unassigned'" content="该写法未归属任何标准产品——识别将按原名输出、不会触发任何产品绑定；如需归一或绑定请到「标准产品」页维护" placement="top">
              <el-tag type="warning" size="small">未归属</el-tag>
            </el-tooltip>
            <span v-else class="verdict-empty">—</span>
          </template>
        </el-table-column>
        <el-table-column label="所属产品测试" min-width="180">
          <template #default="scope">
            <template v-if="ownerPocsOf(scope.row.name).length">
              <el-tag v-for="p in ownerPocsOf(scope.row.name)" :key="p" size="small" type="success" class="poc-tag">{{ p }}</el-tag>
            </template>
            <span v-else class="verdict-empty">—</span>
          </template>
        </el-table-column>
        <el-table-column label="操作" width="250">
          <template #default="scope">
            <el-button size="small" @click.stop="openEdit(scope.row)">编辑</el-button>
            <el-button size="small" type="primary" plain @click.stop="openOwnership(scope.row)">归属</el-button>
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
    <el-dialog v-model="dlg.visible" :title="dlg.isAdd ? '新增条目' : `编辑条目 #${dlg.id}`" width="680px">
      <el-alert
        v-if="!dlg.isAdd && !dlg.schemaOk"
        type="warning"
        :closable="false"
        show-icon
        title="该条目含表单外字段或形态（如 default_port、多 favicon 哈希），已锁定原文模式编辑，字段零丢失"
        style="margin-bottom: 8px"
      />
      <div class="dlg-field">
        <label>编辑模式</label>
        <el-radio-group v-model="dlg.mode" size="small" :disabled="dlg.isAdd || !dlg.schemaOk">
          <el-radio-button value="form">表单模式</el-radio-button>
          <el-tooltip placement="top" content="整条 JSON 原文编辑（内容即所写；保存校验 JSON 对象 + name 非空）">
            <el-radio-button value="raw">原文模式</el-radio-button>
          </el-tooltip>
        </el-radio-group>
        <el-tooltip v-if="dlg.isAdd" placement="top" content="新增走表单结构化；原文形态条目请复制相近条目后切原文编辑">
          <Icon icon="mdi:help-circle" :size="14" class="help-icon" />
        </el-tooltip>
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
            <el-tooltip placement="top" content="focus：命中时输出 [*产品名] 高亮">
              <Icon icon="mdi:help-circle" :size="14" class="help-icon" />
            </el-tooltip>
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
            <el-tooltip placement="top" content="关=排除条件（响应含该特征则不命中）">
              <el-switch v-model="r.is_equal" size="small" active-text="全等" />
            </el-tooltip>
            <el-button link type="danger" size="small" @click="gobyForm.rules.splice(i, 1)">删</el-button>
          </div>
        </template>
        <template v-else>
          <div class="dlg-field">
            <label>名称</label>
            <el-input v-model="fingersForm.name" size="small" placeholder="产品名" style="flex: 1" />
          </div>
          <div class="dlg-field">
            <label>特别关注</label>
            <el-switch v-model="fingersForm.focus" size="small" />
            <el-tooltip placement="top" content="focus：命中时输出 [*产品名] 高亮">
              <Icon icon="mdi:help-circle" :size="14" class="help-icon" />
            </el-tooltip>
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
            <label>图标 md5</label>
            <el-input v-model="fingersForm.md5" size="small" placeholder="图标 md5（icon_hash.html 计算）" style="flex: 1" class="mono" />
          </div>
          <div class="dlg-field">
            <label>图标 mmh3</label>
            <el-input v-model="fingersForm.mmh3Text" size="small" type="textarea" :rows="2" placeholder="一行一个 mmh3 值（可为负）" style="flex: 1" class="mono" />
          </div>
        </template>
      </div>
      <div v-show="dlg.mode === 'raw'">
        <el-input v-model="dlg.rawText" type="textarea" :rows="16" spellcheck="false" class="mono" />
      </div>
      <template #footer>
        <el-button size="small" @click="dlg.visible = false">取消</el-button>
        <el-button type="primary" size="small" :loading="dlg.saving" @click="saveEntry">保存</el-button>
      </template>
    </el-dialog>

    <!-- 归属管理弹窗（BL-POC-009 指纹视角操作：指定归属 / 自立标准产品 / 顺手绑定） -->
    <el-dialog v-model="ownDlg.visible" title="产品归属" width="680px">
      <div class="dlg-field">
        <label>当前写法</label>
        <span class="dlg-value mono-text">{{ ownDlg.name }}（{{ engineSource }} 源）</span>
      </div>
      <div class="dlg-field">
        <label>当前归属</label>
        <div class="dlg-value">
          <el-tag v-if="ownVerdict.relation === 'self'" type="primary" size="small">标准产品：{{ ownDlg.name }}</el-tag>
          <el-tag v-else-if="ownVerdict.relation === 'alias-of' && ownEntry" type="success" size="small">「{{ ownVerdict.owner }}」的别名</el-tag>
          <el-tag v-else type="warning" size="small">未归属（识别按原名输出，不触发任何绑定）</el-tag>
        </div>
      </div>

      <!-- 已归属：显示测试名单 + 顺手绑定/移除 -->
      <template v-if="ownEntry">
        <div class="dlg-section">所属产品测试（指纹命中该产品后定向执行）</div>
        <div class="dlg-field">
          <label>测试名单</label>
          <div class="dlg-value">
            <template v-if="ownEntry.pocs?.length">
              <el-tag v-for="p in ownEntry.pocs" :key="p" size="small" type="success" class="poc-tag" closable @close="unbindPoc(p)">
                {{ p }}
              </el-tag>
            </template>
            <span v-else class="dlg-empty">无（尚未绑定任何测试）</span>
          </div>
        </div>
        <div class="dlg-field">
          <label>追加绑定</label>
          <!-- el-select-v2 虚拟化：POC 名单近万项，普通 el-select 全量渲染卡死弹窗（BL-POC-008 前科） -->
          <el-select-v2
            v-model="ownDlg.bindSelection"
            multiple
            filterable
            collapse-tags
            collapse-tags-tooltip
            :reserve-keyword="false"
            :options="pocOptions"
            placeholder="选择要追加的 POC（可输入过滤）"
            style="flex: 1"
          />
          <el-button type="primary" size="small" :loading="ownDlg.saving" @click="confirmBind">绑定</el-button>
          <el-tooltip placement="top" content="绑定保存到外挂产品册文件，重跑扫描器即生效；测试名单也可在「标准产品」页编辑">
            <Icon icon="mdi:help-circle" :size="14" class="help-icon" />
          </el-tooltip>
        </div>
      </template>

      <!-- 未归属：先选归属方式，再显示对应配置（避免两套表单平铺） -->
      <template v-else>
        <div class="dlg-field">
          <label>归属方式</label>
          <el-radio-group v-model="ownDlg.mode" size="small">
            <el-radio-button value="assign">挂为已有产品的别名</el-radio-button>
            <el-radio-button value="self">自立为新标准产品</el-radio-button>
          </el-radio-group>
        </div>
        <template v-if="ownDlg.mode === 'assign'">
          <div class="dlg-field">
            <label>目标产品</label>
            <el-select-v2
              v-model="ownDlg.assignTarget"
              filterable
              clearable
              :reserve-keyword="false"
              :options="stdOptions"
              placeholder="从标准产品选择（可输入过滤）"
              style="flex: 1"
            />
            <el-button type="primary" size="small" :loading="ownDlg.saving" @click="confirmAssignAlias">挂为别名</el-button>
          </div>
        </template>
        <template v-else-if="ownDlg.mode === 'self'">
          <div class="dlg-field">
            <label>标准名</label>
            <el-input v-model="ownDlg.selfStdName" size="small" placeholder="新标准产品的标准名（识别命中后输出此名）" style="flex: 1" />
            <el-tooltip placement="top" content="以该写法为别名新建标准产品；创建后可继续绑定测试">
              <Icon icon="mdi:help-circle" :size="14" class="help-icon" />
            </el-tooltip>
            <el-button type="success" size="small" :loading="ownDlg.saving" @click="confirmSelfStd">自立</el-button>
          </div>
        </template>
      </template>

      <template #footer>
        <el-button size="small" @click="ownDlg.visible = false">关闭</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
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
import { listPocsApi } from '@/api/poc'
import {
  catalogAddApi,
  catalogEditApi,
  catalogResolveApi,
  getCatalogListApi,
  type CatalogEntryView,
  type ResolveVerdict
} from '@/api/catalog'

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
    await resolveOwnership() // 产品归属判定依赖当页条目名，列表刷新后重算
  } catch (e: any) {
    if (!e?.code) ElMessage.error(String(e)) // API 错误由拦截器统一 toast
  } finally {
    loading.value = false
  }
}

// ---- 产品归属（BL-POC-009 指纹视角）：三态判定 + 测试预览 + 归属操作（挂别名/自立标准产品/顺手绑定）----

// 归属判定表（key=写法原名；同页重名条目判定结果一致，按名索引即可）
const verdicts = ref<Record<string, ResolveVerdict>>({})
// 产品册完整条目列表（归属操作与测试预览的数据源；出厂 50 条量小一次拉取）
const catalogEntries = ref<CatalogEntryView[]>([])

// resolve 的 source=引擎源名（alias 归一表的键空间）：goby.json→goby 引擎，fingers_http.json→fingers 引擎
const engineSource = computed(() => (file.value === 'goby' ? 'goby' : 'fingers'))

const resolveOwnership = async () => {
  const names = [...new Set(items.value.map((i) => i.name))]
  if (!names.length) {
    verdicts.value = {}
    return
  }
  try {
    const res = await catalogResolveApi(names.map((n) => ({ source: engineSource.value, name: n })))
    const map: Record<string, ResolveVerdict> = {}
    for (const v of res.data.verdicts ?? []) map[v.name] = v
    verdicts.value = map
  } catch {
    verdicts.value = {} // 归属判定失败不阻塞列表（列显示 —）；错误由拦截器 toast
  }
}

const verdictOf = (name: string): ResolveVerdict => verdicts.value[name] ?? ({ source: '', name, owner: '', relation: 'unassigned' } as ResolveVerdict)

// 归属标准产品的绑定测试（alias-of 归到 owner 名下；self 是自身；未归属为空。
// 键 lower 化：owner 是引擎小写输出形态（alias.Compile ToLower），条目原名可能是混合大小写）
const ownerPocsOf = (name: string): string[] => {
  const v = verdictOf(name)
  if (v.relation === 'unassigned') return []
  const ownerName = v.owner || name
  return catalogEntries.value.find((e) => e.name.toLowerCase() === ownerName.toLowerCase())?.pocs ?? []
}

const loadCatalogEntries = async () => {
  try {
    const res = await getCatalogListApi()
    catalogEntries.value = res.data.entries ?? []
  } catch {
    // 预览列/归属操作数据缺失不影响主列表
  }
}

// ---- 归属管理弹窗（方案承诺：可指定归属、自立标准产品、顺手绑定）----

const ownDlg = reactive({
  visible: false,
  saving: false,
  name: '', // 当前写法（指纹条目名）
  mode: '' as '' | 'assign' | 'self', // 未归属态两步式：先选归属方式再显示对应配置
  assignTarget: '', // 挂为别名：选中的标准产品
  selfStdName: '', // 自立标准产品：标准名预填（可改）
  bindSelection: [] as string[] // 顺手绑定：POC 多选
})

// 弹窗内当前归属（打开时从 catalogEntries 现算，操作后随刷新更新）
const ownVerdict = computed(() => verdictOf(ownDlg.name))
// 归属产品条目（alias-of 按 owner 匹配；self 按写法本身匹配；键 lower 化对齐引擎输出形态）
const ownEntry = computed(() => {
  const v = ownVerdict.value
  const key = (v.relation === 'self' ? ownDlg.name : v.owner || '').toLowerCase()
  return catalogEntries.value.find((e) => e.name.toLowerCase() === key)
})
// 标准产品候选（挂为别名的目标；排除已归属条目自身）
const stdOptions = computed(() =>
  catalogEntries.value
    .filter((e) => !(ownEntry.value && e.name === ownEntry.value.name))
    .map((e) => ({ label: e.name, value: e.name }))
)
// POC 候选（近万项，el-select-v2 虚拟化——普通 el-select 全量渲染卡死弹窗，BL-POC-008 前科）
const pocOptions = ref<Array<{ label: string; value: string }>>([])

const openOwnership = async (row: FingerEntryItem) => {
  ownDlg.name = row.name
  ownDlg.mode = ''
  ownDlg.assignTarget = ''
  ownDlg.selfStdName = row.name
  ownDlg.bindSelection = []
  await loadCatalogEntries() // 现拉保证拿到最新外置覆盖态
  if (!pocOptions.value.length) {
    try {
      const res = await listPocsApi()
      pocOptions.value = (res.data.pocs ?? []).map((p) => ({ label: p.name, value: p.name }))
    } catch {
      // POC 候选加载失败不阻塞弹窗（绑定区空候选）
    }
  }
  ownDlg.visible = true
}

// 整条写回辅助：读条目 → 变换 → catalog_edit external（整条全量写出铁律：别名区原样携带）
const writeEntry = async (target: string, mutate: (e: CatalogEntryView) => Partial<CatalogEntryView>) => {
  const e = catalogEntries.value.find((x) => x.name === target)
  if (!e) throw new Error(`条目「${target}」不存在`)
  await catalogEditApi(target, 'external', {
    name: e.name,
    vendor: e.vendor,
    product: e.product,
    alias: e.alias ?? {},
    pocs: e.pocs ?? [],
    ...mutate(e)
  })
}

// 挂为已有标准产品的别名（铁律 5：确认文案写明映射改显示名）
const confirmAssignAlias = async () => {
  if (ownDlg.saving) return // 防重入：进行中重复点击直接忽略（headless 补点竞态/手抖）
  const target = ownDlg.assignTarget
  if (!target) return ElMessage.warning('请先选择目标标准产品')
  const src = engineSource.value
  const e = catalogEntries.value.find((x) => x.name === target)
  const names = e?.alias?.[src] ?? []
  if (names.includes(ownDlg.name)) {
    return ElMessage.warning(`「${ownDlg.name}」已在「${target}」的 ${src} 源别名区`)
  }
  try {
    await ElMessageBox.confirm(
      `把写法「${ownDlg.name}」挂为标准产品「${target}」的别名（${src} 源）？映射后扫描输出将把该写法归一显示为标准名「${target}」（归一既有行为）。`,
      '挂为别名',
      { type: 'warning' }
    )
  } catch {
    return
  }
  ownDlg.saving = true
  try {
    await writeEntry(target, () => ({ alias: { ...(e?.alias ?? {}), [src]: [...names, ownDlg.name] } }))
    ElMessage.success(`已挂为「${target}」的别名（保存到外挂文件），重跑扫描器生效`)
    ownDlg.visible = false
    await refreshOwnership()
  } catch {
    // 错误 toast 由拦截器统一展示
  } finally {
    ownDlg.saving = false
  }
}

// 自立标准产品：以该写法为标准名新建条目（别名自动挂自己——天然满足防死条目铁律）
const confirmSelfStd = async () => {
  if (ownDlg.saving) return // 防重入：第一次请求进行中（含归属刷新）时忽略重复点击
  const name = ownDlg.selfStdName.trim()
  if (!name) return ElMessage.warning('标准名不能为空')
  if (catalogEntries.value.some((e) => e.name === name)) {
    return ElMessage.warning(`「${name}」已是标准产品，请在上方选择它挂为别名`)
  }
  ownDlg.saving = true
  try {
    await catalogAddApi({ name, alias: { [engineSource.value]: [ownDlg.name] }, pocs: [] })
    ElMessage.success('已保存到外挂文件（重跑扫描器生效），可在下方顺手绑定测试')
    await refreshOwnership() // 条目+归属判定都刷——verdict 不刷 ownEntry 仍 undefined，弹窗不切已归属分支（实测踩过）
    ownDlg.assignTarget = ''
  } catch {
    // 错误 toast 由拦截器统一展示（含后端守卫 message）
  } finally {
    ownDlg.saving = false
  }
}

// 顺手绑定：归属产品的 pocs 整条写回（追加去重）
const confirmBind = async () => {
  if (ownDlg.saving) return // 防重入
  const e = ownEntry.value
  if (!e) return
  const cur = e.pocs ?? []
  const added = ownDlg.bindSelection.filter((p) => !cur.includes(p))
  if (!added.length) return ElMessage.warning('请选择未绑定的 POC')
  ownDlg.saving = true
  try {
    await writeEntry(e.name, () => ({ pocs: [...cur, ...added] }))
    ElMessage.success(`已绑定 ${added.length} 项到「${e.name}」（保存到外挂文件），重跑扫描器生效`)
    ownDlg.bindSelection = []
    await refreshOwnership()
  } catch {
    // 错误 toast 由拦截器统一展示
  } finally {
    ownDlg.saving = false
  }
}

const unbindPoc = async (poc: string) => {
  const e = ownEntry.value
  if (!e) return
  try {
    await ElMessageBox.confirm(`从「${e.name}」的测试名单移除「${poc}」？`, '移除绑定', { type: 'warning' })
  } catch {
    return
  }
  ownDlg.saving = true
  try {
    await writeEntry(e.name, () => ({ pocs: (e.pocs ?? []).filter((p) => p !== poc) }))
    await refreshOwnership()
  } catch {
    // 错误 toast 由拦截器统一展示
  } finally {
    ownDlg.saving = false
  }
}

// 操作后刷新：归属列 + 测试预览列（catalogEntries 先刷，verdict 再判）
const refreshOwnership = async () => {
  await loadCatalogEntries()
  await resolveOwnership()
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

onMounted(() => {
  reload()
  loadCatalogEntries()
})
</script>

<style scoped>
/* 排版纪律同 POC 页：small 控件（24px）、60px label 右对齐、13px 区标题、零 console 容忍 */
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
.poc-tag {
  margin: 1px 4px 1px 0;
}
.verdict-empty {
  color: var(--el-text-color-secondary);
}

/* 归属弹窗（排版纪律同 POC 页弹窗：60px 右对齐 label + 区块小标题） */
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
.dlg-value {
  flex: 1;
  font-size: 12px;
  line-height: 24px;
  word-break: break-all;
}
.dlg-empty {
  color: var(--el-text-color-secondary);
}
.mono-text {
  font-family: Consolas, monospace;
}
</style>
