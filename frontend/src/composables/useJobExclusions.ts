import { computed, ref, type Ref } from 'vue'
import { GetDefaultExclusions, ListVolumeFolders, PickFolder } from '../../wailsjs/go/main/App'
import { models } from '../../wailsjs/go/models'

export function normPathKey(p: string) {
  return p.replace(/\\+$/, '').toLowerCase()
}

export function isChildOfVolume(path: string, volume: string) {
  if (!volume) return false
  const vol = normPathKey(volume.endsWith('\\') ? volume : `${volume}\\`)
  const p = normPathKey(path)
  const root = vol.slice(0, -1)
  if (!p.startsWith(root)) return false
  const rest = p.slice(root.length).replace(/^\\/, '')
  return rest !== '' && !rest.includes('\\')
}

export function splitJobExclusions(exclusions: string[], volume: string) {
  const folders: string[] = []
  const patterns: string[] = []
  for (const e of exclusions || []) {
    const line = (e || '').trim()
    if (!line) continue
    if (volume && isChildOfVolume(line, volume)) {
      folders.push(line)
    } else {
      patterns.push(line)
    }
  }
  return { folders, patterns }
}

export function useJobExclusions(
  sourceMode: Ref<'volume' | 'paths'>,
  volume: Ref<string>,
  sourcesList: Ref<string[]>,
  onChanged?: () => void,
) {
  const exclusion_patterns = ref('')
  const volumeFolders = ref<models.VolumeFolder[]>([])
  const volumeFolderExclude = ref<Set<string>>(new Set())

  const exclusionsList = computed(() => {
    const patterns = exclusion_patterns.value.split('\n').map((s) => s.trim()).filter(Boolean)
    const folders = [...volumeFolderExclude.value]
    return [...folders, ...patterns]
  })

  async function loadVolumeFolders() {
    if (sourceMode.value !== 'volume' || !volume.value) {
      volumeFolders.value = []
      return
    }
    try {
      volumeFolders.value = await ListVolumeFolders(volume.value)
    } catch {
      volumeFolders.value = []
    }
  }

  function toggleFolderExclude(path: string) {
    const next = new Set(volumeFolderExclude.value)
    if (next.has(path)) next.delete(path)
    else next.add(path)
    volumeFolderExclude.value = next
    onChanged?.()
  }

  function isFolderExcluded(path: string) {
    return volumeFolderExclude.value.has(path)
  }

  async function addExcludeFolder() {
    const p = await PickFolder()
    if (!p) return
    const next = new Set(volumeFolderExclude.value)
    next.add(p)
    volumeFolderExclude.value = next
    onChanged?.()
  }

  async function loadDefaultExclusions() {
    const defaults = await GetDefaultExclusions() || []
    exclusion_patterns.value = defaults
      .filter((d) => !d.includes('\\') && !d.includes('/'))
      .join('\n')
    onChanged?.()
  }

  function resetExclusions() {
    exclusion_patterns.value = ''
    volumeFolderExclude.value = new Set()
    volumeFolders.value = []
  }

  async function onSourceModeVolume() {
    await loadVolumeFolders()
    if (!exclusion_patterns.value.trim()) {
      await loadDefaultExclusions()
    }
    onChanged?.()
  }

  function onSourceModePaths() {
    volumeFolders.value = []
    volumeFolderExclude.value = new Set()
    onChanged?.()
  }

  return {
    exclusion_patterns,
    volumeFolders,
    volumeFolderExclude,
    exclusionsList,
    loadVolumeFolders,
    toggleFolderExclude,
    isFolderExcluded,
    addExcludeFolder,
    loadDefaultExclusions,
    resetExclusions,
    onSourceModeVolume,
    onSourceModePaths,
  }
}
