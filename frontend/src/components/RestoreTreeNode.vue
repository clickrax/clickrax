<script setup lang="ts">
import { computed } from 'vue'
import type { TreeNode } from '../utils/restoreTree'
import { checkState } from '../utils/restoreTree'
import { formatBytes } from '../utils/format'
import RestoreTree from './RestoreTree.vue'

const props = defineProps<{
  node: TreeNode
  checked: Set<string>
  expanded: Set<string>
  depth: number
}>()

const emit = defineEmits<{
  toggleCheck: [node: TreeNode, value: boolean]
  toggleExpand: [path: string]
}>()

const state = computed(() => checkState(props.node, props.checked))
const isOpen = computed(() => props.expanded.has(props.node.path))
const isChecked = computed(() => state.value === 'checked')
const isIndeterminate = computed(() => state.value === 'indeterminate')

function formatSize(n: number) {
  if (!n) return '—'
  return formatBytes(n)
}

function onCheck(e: Event) {
  const el = e.target as HTMLInputElement
  emit('toggleCheck', props.node, el.checked)
}
</script>

<template>
  <li class="tree-node" :class="{ 'is-dir': node.isDir, open: isOpen }">
    <div
      class="tree-row"
      :class="{ checked: isChecked, indeterminate: isIndeterminate, dir: node.isDir }"
      :style="{ '--depth': depth }"
      @click="node.isDir && emit('toggleExpand', node.path)"
    >
      <div class="tree-indent">
        <span v-for="i in depth" :key="i" class="tree-guide" />
      </div>

      <button
        v-if="node.isDir"
        type="button"
        class="expand-btn"
        :class="{ open: isOpen }"
        :aria-expanded="isOpen"
        @click.stop="emit('toggleExpand', node.path)"
      >
        <svg viewBox="0 0 16 16" fill="currentColor"><path d="M6 4l4 4-4 4V4z"/></svg>
      </button>
      <span v-else class="expand-spacer" />

      <label class="tree-check" @click.stop>
        <input
          type="checkbox"
          :checked="isChecked"
          :indeterminate.prop="isIndeterminate"
          @change="onCheck"
        />
        <span class="check-ui" :class="{ checked: isChecked, indeterminate: isIndeterminate }" />
      </label>

      <span class="node-icon" :class="node.isDir ? 'icon-dir' : 'icon-file'">
        <svg v-if="node.isDir" viewBox="0 0 20 20" fill="currentColor">
          <path d="M2 6a2 2 0 0 1 2-2h4.586a1 1 0 0 1 .707.293l1.414 1.414A1 1 0 0 0 11.414 6H16a2 2 0 0 1 2 2v8a2 2 0 0 1-2 2H4a2 2 0 0 1-2-2V6z"/>
        </svg>
        <svg v-else viewBox="0 0 20 20" fill="currentColor">
          <path d="M4 4a2 2 0 0 1 2-2h4.586a1 1 0 0 1 .707.293L12.707 4H16a2 2 0 0 1 2 2v10a2 2 0 0 1-2 2H4a2 2 0 0 1-2-2V4z"/>
        </svg>
      </span>

      <span class="name" :title="node.path">{{ node.name }}</span>
      <span class="size">{{ formatSize(node.size) }}</span>
      <span class="mtime">{{ node.modified || '—' }}</span>
    </div>

    <ul v-if="node.isDir && isOpen && node.children.length" class="tree-children">
      <RestoreTreeNode
        v-for="child in node.children"
        :key="child.path"
        :node="child"
        :checked="checked"
        :expanded="expanded"
        :depth="depth + 1"
        @toggle-check="(n, v) => emit('toggleCheck', n, v)"
        @toggle-expand="(p) => emit('toggleExpand', p)"
      />
    </ul>
  </li>
</template>

<style scoped>
.tree-node {
  list-style: none;
}
.tree-children {
  margin: 0;
  padding: 0;
  list-style: none;
}
.tree-row {
  display: grid;
  grid-template-columns: auto 1.1rem 1.6rem 1.35rem 1fr 5rem 8.5rem;
  align-items: center;
  gap: 0.4rem;
  padding: 0.28rem 0.65rem 0.28rem 0;
  margin: 1px 0.35rem;
  border-radius: 8px;
  font-size: 0.86rem;
  transition: background 0.15s ease, box-shadow 0.15s ease;
  border: 1px solid transparent;
}
.tree-row.dir {
  cursor: pointer;
}
.tree-row:hover {
  background: rgba(51, 65, 85, 0.45);
}
.tree-row.checked {
  background: rgba(59, 130, 246, 0.12);
  border-color: rgba(59, 130, 246, 0.25);
}
.tree-row.indeterminate {
  background: rgba(59, 130, 246, 0.06);
}
.tree-indent {
  display: flex;
  width: calc(var(--depth) * 0.85rem);
  min-width: 0;
  flex-shrink: 0;
}
.tree-guide {
  width: 0.85rem;
  border-left: 1px solid rgba(71, 85, 105, 0.45);
  margin-left: 0.4rem;
}
.expand-btn {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 1.1rem;
  height: 1.1rem;
  padding: 0;
  border: none;
  border-radius: 4px;
  background: transparent;
  color: #94a3b8;
  cursor: pointer;
  transition: transform 0.15s ease, color 0.15s ease, background 0.15s ease;
}
.expand-btn:hover {
  background: rgba(100, 116, 139, 0.25);
  color: #e2e8f0;
}
.expand-btn.open {
  transform: rotate(90deg);
}
.expand-btn svg {
  width: 0.75rem;
  height: 0.75rem;
}
.expand-spacer {
  width: 1.1rem;
}
.tree-check {
  position: relative;
  display: flex;
  align-items: center;
  cursor: pointer;
}
.tree-check input {
  position: absolute;
  opacity: 0;
  width: 0;
  height: 0;
}
.check-ui {
  width: 1rem;
  height: 1rem;
  border-radius: 4px;
  border: 1.5px solid #475569;
  background: rgba(15, 23, 42, 0.6);
  transition: all 0.15s ease;
  display: flex;
  align-items: center;
  justify-content: center;
}
.check-ui.checked,
.check-ui.indeterminate {
  background: var(--primary);
  border-color: var(--primary);
}
.check-ui.checked::after {
  content: '';
  width: 0.35rem;
  height: 0.6rem;
  border: solid white;
  border-width: 0 2px 2px 0;
  transform: rotate(45deg) translate(-1px, -1px);
}
.check-ui.indeterminate::after {
  content: '';
  width: 0.5rem;
  height: 2px;
  background: white;
  border-radius: 1px;
}
.node-icon {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 1.35rem;
  height: 1.35rem;
  border-radius: 6px;
}
.node-icon svg {
  width: 1rem;
  height: 1rem;
}
.icon-dir {
  color: #60a5fa;
  background: rgba(59, 130, 246, 0.12);
}
.icon-file {
  color: #94a3b8;
  background: rgba(148, 163, 184, 0.08);
}
.name {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-weight: 500;
}
.tree-row.dir .name {
  font-weight: 600;
}
.size,
.mtime {
  color: var(--muted);
  font-size: 0.78rem;
  text-align: right;
  font-variant-numeric: tabular-nums;
}
</style>
