<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'

const props = defineProps<{
  page: number
  pageSize: number
  pageSizes: readonly number[]
  total: number
  totalPages: number
  rangeFrom: number
  rangeTo: number
  pageNumbers: number[]
}>()

const emit = defineEmits<{
  'update:page': [number]
  'update:pageSize': [number]
}>()

const { t } = useI18n()

const showBar = computed(() => props.total > 0)

function displayPages(nums: number[]) {
  const out: Array<number | '…'> = []
  let prev = 0
  for (const n of nums) {
    if (prev && n - prev > 1) out.push('…')
    out.push(n)
    prev = n
  }
  return out
}

const pages = computed(() => displayPages(props.pageNumbers))
</script>

<template>
  <footer v-if="showBar" class="pagination-bar">
    <div class="pagination-meta">
      <span>{{ t('pagination.showing', { from: rangeFrom, to: rangeTo, total }) }}</span>
      <label class="pagination-size">
        <span>{{ t('pagination.per_page') }}</span>
        <select
          :value="pageSize"
          @change="emit('update:pageSize', Number(($event.target as HTMLSelectElement).value))"
        >
          <option v-for="s in pageSizes" :key="s" :value="s">{{ s }}</option>
        </select>
      </label>
    </div>
    <nav v-if="totalPages > 1" class="pagination-nav" :aria-label="t('pagination.nav')">
      <button type="button" class="btn sm ghost icon-only" :disabled="page <= 1" :title="t('pagination.first')" @click="emit('update:page', 1)">«</button>
      <button type="button" class="btn sm ghost icon-only" :disabled="page <= 1" :title="t('pagination.prev')" @click="emit('update:page', page - 1)">‹</button>
      <template v-for="(p, i) in pages" :key="i">
        <span v-if="p === '…'" class="pagination-ellipsis">…</span>
        <button
          v-else
          type="button"
          class="btn sm ghost pagination-page"
          :class="{ active: p === page }"
          @click="emit('update:page', p)"
        >{{ p }}</button>
      </template>
      <button type="button" class="btn sm ghost icon-only" :disabled="page >= totalPages" :title="t('pagination.next')" @click="emit('update:page', page + 1)">›</button>
      <button type="button" class="btn sm ghost icon-only" :disabled="page >= totalPages" :title="t('pagination.last')" @click="emit('update:page', totalPages)">»</button>
    </nav>
  </footer>
</template>
