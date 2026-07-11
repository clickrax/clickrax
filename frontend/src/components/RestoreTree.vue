<script setup lang="ts">
import { computed } from 'vue'
import type { TreeNode } from '../utils/restoreTree'
import { checkState } from '../utils/restoreTree'
import RestoreTreeNode from './RestoreTreeNode.vue'

const props = defineProps<{
  nodes: TreeNode[]
  checked: Set<string>
  expanded: Set<string>
  depth?: number
}>()

const emit = defineEmits<{
  toggleCheck: [node: TreeNode, value: boolean]
  toggleExpand: [path: string]
}>()

const depth = computed(() => props.depth ?? 0)
</script>

<template>
  <ul class="tree-list" :class="{ root: depth === 0 }">
    <RestoreTreeNode
      v-for="node in nodes"
      :key="node.path"
      :node="node"
      :checked="checked"
      :expanded="expanded"
      :depth="depth"
      @toggle-check="(n, v) => emit('toggleCheck', n, v)"
      @toggle-expand="(p) => emit('toggleExpand', p)"
    />
  </ul>
</template>

<style scoped>
.tree-list {
  list-style: none;
  margin: 0;
  padding: 0;
}
.tree-list.root {
  min-width: 100%;
}
</style>
