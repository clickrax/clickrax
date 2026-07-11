import { onMounted, ref, type Ref } from 'vue'

export type ColumnDef = {
  key: string
  /** Percent of table width (excluding fixed columns). Omit for flex column. */
  defaultPercent?: number
  minPercent?: number
  /** Fixed pixel width (e.g. actions). */
  fixedPx?: number
  flex?: boolean
}

export function useResizableColumns(
  storageKey: string,
  columns: ColumnDef[],
  tableRef: Ref<HTMLElement | null>,
) {
  const fixedCols = columns.filter((c) => c.fixedPx)
  const percentCols = columns.filter((c) => c.defaultPercent != null)
  const flexCol = columns.find((c) => c.flex)

  const defaults: Record<string, number> = {}
  for (const c of percentCols) {
    defaults[c.key] = c.defaultPercent!
  }

  const widths = ref<Record<string, number>>({ ...defaults })

  onMounted(() => {
    try {
      const raw = localStorage.getItem(storageKey)
      if (raw) {
        const saved = JSON.parse(raw) as Record<string, number>
        widths.value = { ...defaults, ...saved }
      }
    } catch {
      /* ignore */
    }
  })

  function colStyle(key: string): Record<string, string> | undefined {
    const fixed = fixedCols.find((c) => c.key === key)
    if (fixed?.fixedPx) return { width: `${fixed.fixedPx}px` }
    if (flexCol?.key === key) return undefined
    const p = widths.value[key]
    return p != null ? { width: `${p}%` } : undefined
  }

  function startResize(key: string, e: MouseEvent) {
    const def = percentCols.find((c) => c.key === key)
    if (!def) return

    e.preventDefault()
    e.stopPropagation()
    const min = def.minPercent ?? 5
    const tableW = tableRef.value?.clientWidth ?? 800
    const fixedPx = fixedCols.reduce((s, c) => s + (c.fixedPx ?? 0), 0)
    const flexArea = Math.max(tableW - fixedPx, 200)
    const startX = e.clientX
    const startP = widths.value[key] ?? def.defaultPercent ?? min

    const onMove = (ev: MouseEvent) => {
      const deltaPx = ev.clientX - startX
      const deltaP = (deltaPx / flexArea) * 100
      const otherSum = percentCols
        .filter((c) => c.key !== key)
        .reduce((s, c) => s + (widths.value[c.key] ?? c.defaultPercent ?? 0), 0)
      const maxP = Math.max(min, 100 - otherSum - (flexCol ? 8 : 0))
      widths.value[key] = Math.min(maxP, Math.max(min, startP + deltaP))
    }

    const onUp = () => {
      localStorage.setItem(storageKey, JSON.stringify(widths.value))
      document.removeEventListener('mousemove', onMove)
      document.removeEventListener('mouseup', onUp)
      document.body.style.cursor = ''
      document.body.style.userSelect = ''
    }

    document.body.style.cursor = 'col-resize'
    document.body.style.userSelect = 'none'
    document.addEventListener('mousemove', onMove)
    document.addEventListener('mouseup', onUp)
  }

  return { widths, colStyle, startResize }
}
