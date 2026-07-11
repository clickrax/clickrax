<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { TreeNode, SortKey, SortDir } from '../utils/restoreTree'
import { checkState } from '../utils/restoreTree'
import { formatBytes, formatDateTime } from '../utils/format'

const props = defineProps<{
  items: TreeNode[]
  crumbs: { name: string; path: string }[]
  checked: Set<string>
  activePath: string | null
  sortKey: SortKey
  sortDir: SortDir
  searchMode: boolean
  canGoUp: boolean
}>()

const emit = defineEmits<{
  navigate: [path: string]
  goUp: []
  sort: [key: SortKey]
  toggleCheck: [node: TreeNode, value: boolean]
  activate: [path: string]
  open: [node: TreeNode]
}>()

const { t } = useI18n()

const allVisibleChecked = computed(() => {
  if (!props.items.length) return false
  return props.items.every((n) => checkState(n, props.checked) === 'checked')
})

const someVisibleChecked = computed(() =>
  props.items.some((n) => {
    const st = checkState(n, props.checked)
    return st === 'checked' || st === 'indeterminate'
  }),
)

function formatSize(n: number) {
  if (!n) return t('common.dash')
  return formatBytes(n)
}

function formatDate(s: string) {
  return formatDateTime(s)
}

function sortIcon(key: SortKey) {
  if (props.sortKey !== key) return ''
  return props.sortDir === 'asc' ? '▲' : '▼'
}

function onHeaderClick(key: SortKey) {
  emit('sort', key)
}

function onRowClick(node: TreeNode) {
  emit('activate', node.path)
}

function onRowDblClick(node: TreeNode) {
  if (node.isDir) emit('open', node)
}

function onCheck(node: TreeNode, e: Event) {
  emit('toggleCheck', node, (e.target as HTMLInputElement).checked)
}

function onCheckAll(e: Event) {
  const v = (e.target as HTMLInputElement).checked
  for (const n of props.items) emit('toggleCheck', n, v)
}

function rowState(node: TreeNode) {
  return checkState(node, props.checked)
}
</script>

<template>
  <div class="explorer">
    <div class="explorer-nav">
      <button type="button" class="nav-btn" :disabled="!canGoUp" :title="t('restore.go_up')" @click="emit('goUp')">
        <svg viewBox="0 0 20 20" fill="currentColor"><path d="M10 3l7 7H3l7-7zm0 14V8h2v9h-2z"/></svg>
      </button>
      <div class="breadcrumbs">
        <template v-for="(c, i) in crumbs" :key="c.path">
          <button type="button" class="crumb" :class="{ last: i === crumbs.length - 1 }" @click="emit('navigate', c.path)">
            {{ c.name }}
          </button>
          <span v-if="i < crumbs.length - 1" class="crumb-sep">›</span>
        </template>
      </div>
      <span v-if="searchMode" class="search-badge">{{ t('restore.search_results') }}</span>
    </div>

    <div class="explorer-header">
      <label class="h-check" @click.stop>
        <input
          type="checkbox"
          :checked="allVisibleChecked"
          :indeterminate.prop="!allVisibleChecked && someVisibleChecked"
          @change="onCheckAll"
        />
      </label>
      <button type="button" class="h-col name-col" @click="onHeaderClick('name')">
        {{ t('restore.col_name') }} <span class="sort-mark">{{ sortIcon('name') }}</span>
      </button>
      <button type="button" class="h-col owner-col" @click="onHeaderClick('owner')">
        {{ t('restore.col_owner') }} <span class="sort-mark">{{ sortIcon('owner') }}</span>
      </button>
      <button type="button" class="h-col size-col" @click="onHeaderClick('size')">
        {{ t('restore.col_size') }} <span class="sort-mark">{{ sortIcon('size') }}</span>
      </button>
      <button type="button" class="h-col date-col" @click="onHeaderClick('modified')">
        {{ t('restore.col_modified') }} <span class="sort-mark">{{ sortIcon('modified') }}</span>
      </button>
      <span class="h-col attrs-col">{{ t('restore.col_attributes') }}</span>
    </div>

    <div class="explorer-list" v-if="items.length">
      <div
        v-for="node in items"
        :key="node.path"
        class="explorer-row"
        :class="{
          active: activePath === node.path,
          checked: rowState(node) === 'checked',
          dir: node.isDir,
        }"
        @click="onRowClick(node)"
        @dblclick="onRowDblClick(node)"
      >
        <label class="row-check" @click.stop>
          <input
            type="checkbox"
            :checked="rowState(node) === 'checked'"
            :indeterminate.prop="rowState(node) === 'indeterminate'"
            @change="onCheck(node, $event)"
          />
        </label>
        <span class="row-icon" :class="node.isDir ? 'icon-dir' : 'icon-file'">
          <svg v-if="node.isDir" viewBox="0 0 20 20" fill="currentColor"><path d="M2 6a2 2 0 0 1 2-2h4.586a1 1 0 0 1 .707.293l1.414 1.414A1 1 0 0 0 11.414 6H16a2 2 0 0 1 2 2v8a2 2 0 0 1-2 2H4a2 2 0 0 1-2-2V6z"/></svg>
          <svg v-else viewBox="0 0 20 20" fill="currentColor"><path d="M4 4a2 2 0 0 1 2-2h4.586a1 1 0 0 1 .707.293L12.707 4H16a2 2 0 0 1 2 2v10a2 2 0 0 1-2 2H4a2 2 0 0 1-2-2V4z"/></svg>
        </span>
        <span class="row-name" :title="node.path">{{ node.name }}</span>
        <span class="row-owner" :title="node.owner || '—'">{{ node.owner || '—' }}</span>
        <span class="row-size">{{ formatSize(node.size) }}</span>
        <span class="row-date">{{ formatDate(node.modified) }}</span>
        <span class="row-attrs" :title="node.attributes || '—'">{{ node.attributes || '—' }}</span>
      </div>
    </div>
    <div v-else class="explorer-empty">
      <p>{{ t('restore.empty_folder') }}</p>
    </div>
  </div>
</template>

<style scoped>
.explorer {
  display: flex;
  flex-direction: column;
  min-height: 320px;
}
.explorer-nav {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  padding: 0.55rem 0.75rem;
  border-bottom: 1px solid rgba(51, 65, 85, 0.5);
  background: rgba(15, 23, 42, 0.4);
}
.nav-btn {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 2rem;
  height: 2rem;
  border: 1px solid rgba(71, 85, 105, 0.5);
  border-radius: 8px;
  background: rgba(30, 41, 59, 0.8);
  color: #cbd5e1;
  cursor: pointer;
}
.nav-btn:hover:not(:disabled) {
  background: rgba(51, 65, 85, 0.9);
  color: #fff;
}
.nav-btn:disabled {
  opacity: 0.35;
  cursor: default;
}
.nav-btn svg {
  width: 1rem;
  height: 1rem;
}
.breadcrumbs {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 0.15rem;
  min-width: 0;
  flex: 1;
}
.crumb {
  border: none;
  background: transparent;
  color: #93c5fd;
  font-size: 0.85rem;
  cursor: pointer;
  padding: 0.2rem 0.35rem;
  border-radius: 4px;
  max-width: 200px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.crumb:hover {
  background: rgba(59, 130, 246, 0.12);
}
.crumb.last {
  color: #e2e8f0;
  font-weight: 600;
  cursor: default;
}
.crumb.last:hover {
  background: transparent;
}
.crumb-sep {
  color: #64748b;
  font-size: 0.8rem;
}
.search-badge {
  font-size: 0.75rem;
  padding: 0.2rem 0.55rem;
  border-radius: 999px;
  background: rgba(234, 179, 8, 0.15);
  color: #fcd34d;
  border: 1px solid rgba(234, 179, 8, 0.3);
  white-space: nowrap;
}
.explorer-header {
  display: grid;
  grid-template-columns: 2rem 1fr 8.5rem 5rem 9rem 4.5rem;
  gap: 0.5rem;
  align-items: center;
  padding: 0.4rem 0.75rem;
  border-bottom: 1px solid rgba(51, 65, 85, 0.45);
  background: rgba(15, 23, 42, 0.25);
}
.h-check input {
  cursor: pointer;
}
.h-col {
  border: none;
  background: transparent;
  color: #94a3b8;
  font-size: 0.72rem;
  text-transform: uppercase;
  letter-spacing: 0.05em;
  font-weight: 600;
  cursor: pointer;
  text-align: left;
  padding: 0.2rem 0;
  display: flex;
  align-items: center;
  gap: 0.25rem;
}
.h-col:hover {
  color: #e2e8f0;
}
.owner-col {
  justify-content: flex-start;
  text-align: left;
}
.attrs-col {
  justify-content: flex-end;
  text-align: right;
  cursor: default;
}
.sort-mark {
  font-size: 0.65rem;
  color: var(--primary);
}
.explorer-list {
  flex: 1;
  overflow: auto;
  max-height: min(52vh, 520px);
}
.explorer-row {
  display: grid;
  grid-template-columns: 2rem 1.35rem 1fr 8.5rem 5rem 9rem 4.5rem;
  gap: 0.5rem;
  align-items: center;
  padding: 0.32rem 0.75rem;
  font-size: 0.86rem;
  border-bottom: 1px solid rgba(51, 65, 85, 0.2);
  cursor: default;
  user-select: none;
}
.explorer-row:hover {
  background: rgba(51, 65, 85, 0.35);
}
.explorer-row.active {
  background: rgba(59, 130, 246, 0.1);
  outline: 1px solid rgba(59, 130, 246, 0.25);
  outline-offset: -1px;
}
.explorer-row.checked {
  background: rgba(59, 130, 246, 0.08);
}
.explorer-row.dir {
  cursor: pointer;
}
.row-check input {
  cursor: pointer;
}
.row-icon {
  display: flex;
  align-items: center;
  justify-content: center;
}
.row-icon svg {
  width: 1rem;
  height: 1rem;
}
.icon-dir { color: #60a5fa; }
.icon-file { color: #94a3b8; }
.row-name {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-weight: 500;
}
.explorer-row.dir .row-name {
  font-weight: 600;
}
.row-size,
.row-date,
.row-owner,
.row-attrs {
  text-align: right;
  color: var(--muted);
  font-size: 0.8rem;
  font-variant-numeric: tabular-nums;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.row-owner {
  text-align: left;
}
.explorer-empty {
  padding: 3rem;
  text-align: center;
  color: var(--muted);
}
</style>
