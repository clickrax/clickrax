import { computed, ref, watch, type Ref } from 'vue'

const PAGE_SIZES = [10, 25, 50, 100] as const

export function usePagination<T>(items: Ref<T[]>, storageKey?: string, defaultSize = 25) {
  const savedSize = storageKey ? Number(localStorage.getItem(storageKey)) : 0
  const initialSize = PAGE_SIZES.includes(savedSize as (typeof PAGE_SIZES)[number]) ? savedSize : defaultSize

  const page = ref(1)
  const pageSize = ref(initialSize)

  const total = computed(() => items.value.length)
  const totalPages = computed(() => Math.max(1, Math.ceil(total.value / pageSize.value)))

  const pageItems = computed(() => {
    const start = (page.value - 1) * pageSize.value
    return items.value.slice(start, start + pageSize.value)
  })

  const rangeFrom = computed(() => (total.value === 0 ? 0 : (page.value - 1) * pageSize.value + 1))
  const rangeTo = computed(() => Math.min(page.value * pageSize.value, total.value))

  const pageNumbers = computed(() => {
    const tp = totalPages.value
    const cur = page.value
    if (tp <= 7) return Array.from({ length: tp }, (_, i) => i + 1)
    const pages = new Set<number>([1, tp, cur, cur - 1, cur + 1])
    return [...pages].filter((p) => p >= 1 && p <= tp).sort((a, b) => a - b)
  })

  watch(totalPages, (tp) => {
    if (page.value > tp) page.value = tp
  })

  watch(items, () => {
    if (page.value > totalPages.value) page.value = totalPages.value
  })

  function setPage(p: number) {
    page.value = Math.min(Math.max(1, p), totalPages.value)
  }

  function setPageSize(size: number) {
    pageSize.value = size
    page.value = 1
    if (storageKey) localStorage.setItem(storageKey, String(size))
  }

  function resetPage() {
    page.value = 1
  }

  return {
    page,
    pageSize,
    pageSizes: PAGE_SIZES,
    total,
    totalPages,
    pageItems,
    pageNumbers,
    rangeFrom,
    rangeTo,
    setPage,
    setPageSize,
    resetPage,
  }
}
