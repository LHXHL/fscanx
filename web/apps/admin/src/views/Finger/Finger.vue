<template>
  <div class="finger-page">
    <el-card shadow="never">
      <template #header>
        <div class="card-head">
          <span>指纹库明文编辑</span>
          <div>
            <el-select v-model="file" style="width: 220px">
              <el-option label="goby.json" value="goby" />
              <el-option label="fingers_http.json" value="fingers_http" />
            </el-select>
            <el-button size="small" @click="load" :loading="loading">加载</el-button>
            <el-button type="primary" size="small" @click="save" :loading="saving">校验并保存</el-button>
            <el-button type="success" size="small" @click="pack" :loading="packing">一键重打包</el-button>
          </div>
        </div>
      </template>

      <el-alert
        type="info"
        :closable="false"
        show-icon
        title="整文编辑：内容即所写（除目标改动外零格式漂移）；保存仅做 JSON 合法性校验；重打包产物 .gz 与命令行 go run ./mylib/finger/cmd/pack 字节一致，下次 go build 生效。"
        style="margin-bottom: 10px"
      />

      <el-input
        v-model="content"
        type="textarea"
        :rows="26"
        spellcheck="false"
        class="mono"
        placeholder="点「加载」拉取明文…"
      />

      <el-alert
        v-if="packLines.length"
        type="success"
        :closable="true"
        show-icon
        :title="'重打包完成：'"
        style="margin-top: 10px"
      >
        <pre class="pack-out">{{ packLines.join('\n') }}</pre>
      </el-alert>
    </el-card>
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import { ElMessage } from 'element-plus'
import { getFingerApi, packApi, putFingerApi } from '@/api/finger'

defineOptions({ name: 'Finger' })

const file = ref<'goby' | 'fingers_http'>('goby')
const content = ref('')
const loading = ref(false)
const saving = ref(false)
const packing = ref(false)
const packLines = ref<string[]>([])

const load = async () => {
  loading.value = true
  try {
    const res = await getFingerApi()
    content.value = res.data.files[file.value] ?? ''
    if (!content.value) ElMessage.warning('未加载到内容（检查 web 端 -root 是否指向仓库根）')
  } catch (e) {
    ElMessage.error(String(e))
  } finally {
    loading.value = false
  }
}

const save = async () => {
  saving.value = true
  try {
    const res = await putFingerApi(file.value, content.value)
    ElMessage.success('校验通过已保存：' + res.data.path)
  } catch (e) {
    ElMessage.error(String(e))
  } finally {
    saving.value = false
  }
}

const pack = async () => {
  packing.value = true
  try {
    const res = await packApi()
    packLines.value = (res.data.results ?? []).map(
      (r) => `${r.src} → ${r.dst}（${r.raw} → ${r.gz} B）`
    )
    ElMessage.success('重打包完成，回仓库执行 go build 生效')
  } catch (e) {
    ElMessage.error(String(e))
  } finally {
    packing.value = false
  }
}
</script>

<style scoped>
.finger-page {
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
.pack-out {
  margin: 0;
  font-family: Consolas, monospace;
  font-size: 12px;
  white-space: pre-wrap;
}
</style>
