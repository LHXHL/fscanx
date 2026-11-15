<template>
  <div class="cond-tree">
    <!-- 组节点：AND/OR + 子项列表 -->
    <div v-if="node.logic" class="group">
      <div class="group-head">
        <el-radio-group :model-value="node.logic" size="small" @update:model-value="node.logic = String($event)">
          <el-radio-button value="and">且 AND</el-radio-button>
          <el-radio-button value="or">或 OR</el-radio-button>
        </el-radio-group>
        <el-button size="small" @click="addCond">+ 条件</el-button>
        <el-button size="small" @click="addGroup">+ 嵌套子组</el-button>
        <el-button v-if="removable" link type="danger" size="small" @click="emit('remove')">删组</el-button>
      </div>
      <div v-for="(c, i) in node.conditions" :key="i" class="child">
        <CondTree :node="c" removable @remove="node.conditions?.splice(i, 1)" />
      </div>
      <div v-if="!node.conditions?.length" class="empty">空组——添加条件或删除此组</div>
    </div>

    <!-- 条件节点 -->
    <div v-else class="cond-row">
      <el-select :model-value="node.target" size="small" style="width: 110px" @update:model-value="onTarget(String($event))">
        <el-option v-for="(v, k) in TARGETS" :key="k" :label="v" :value="k" />
      </el-select>
      <el-select :model-value="node.op" size="small" style="width: 96px" @update:model-value="node.op = String($event)">
        <el-option v-for="o in opsOf(node.target)" :key="o" :label="OPS[o]" :value="o" />
      </el-select>
      <el-input
        v-if="node.target === 'header'"
        :model-value="node.key"
        size="small"
        placeholder="Header 名"
        style="width: 160px"
        @update:model-value="node.key = String($event)"
      />
      <el-input
        :model-value="node.value"
        size="small"
        :placeholder="node.target === 'status' ? '如 200' : '匹配内容'"
        style="flex: 1"
        @update:model-value="node.value = String($event)"
      />
      <el-button v-if="removable" link type="danger" size="small" @click="emit('remove')">删</el-button>
    </div>
  </div>
</template>

<script setup lang="ts">
// 递归谓词树编辑器（BL-POC-004 多步结构化轮）：组节点 AND/OR 任意嵌套。
// SFC 自引用（文件名即组件名）实现递归渲染；节点直接原地变更（父组件持有数据）。
import { CondNode, TARGETS, OPS, opsOf } from './predicates'

defineOptions({ name: 'CondTree' })

const props = withDefaults(defineProps<{ node: CondNode; removable?: boolean }>(), { removable: false })

const emit = defineEmits<{ remove: [] }>()

// 切匹配项后操作归一化（status 只能 eq，header 只能 contains，body 两种保留）
const onTarget = (target: string) => {
  props.node.target = target
  const ops = opsOf(target)
  if (!ops.includes(props.node.op ?? '')) {
    props.node.op = ops[0]
  }
}

// 组内追加一个条件（默认状态码等于 200，用户再改）
const addCond = () => {
  props.node.conditions = props.node.conditions ?? []
  props.node.conditions.push({ target: 'status', op: 'eq', key: '', value: '' })
}

// 组内追加嵌套子组：子组自带且/或与一条初始条件，用于组合 (A && (B || C)) 类深度逻辑
const addGroup = () => {
  props.node.conditions = props.node.conditions ?? []
  props.node.conditions.push({
    logic: 'and',
    conditions: [{ target: 'status', op: 'eq', key: '', value: '' }]
  })
}
</script>

<style scoped>
.group {
  border: 1px dashed var(--el-border-color);
  border-radius: 6px;
  padding: 8px;
  margin: 4px 0;
}
.group-head {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 6px;
}
.child {
  margin-left: 18px;
}
.cond-row {
  display: flex;
  align-items: center;
  gap: 6px;
  margin: 4px 0;
}
.empty {
  color: var(--el-text-color-placeholder);
  font-size: 12px;
  margin-left: 18px;
}
</style>
