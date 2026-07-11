import { ref } from 'vue'
import { GetExecutionState, IsStopping } from '../../wailsjs/go/main/App'
import { EventsOn } from '../../wailsjs/runtime/runtime'
import { models } from '../../wailsjs/go/models'

let cachedState: models.ExecutionState | null = null
let cachedStopping = false
let refreshPromise: Promise<void> | null = null
let eventsBound = false

function applyCache(
  state: ReturnType<typeof ref<models.ExecutionState | null>>,
  stopping: ReturnType<typeof ref<boolean>>,
  loading: ReturnType<typeof ref<boolean>>,
) {
  if (!cachedState) return
  state.value = cachedState
  stopping.value = cachedStopping
  loading.value = false
}

async function fetchExecutionState(): Promise<void> {
  if (refreshPromise) {
    await refreshPromise
    return
  }
  refreshPromise = (async () => {
    try {
      const [nextState, nextStopping] = await Promise.all([
        GetExecutionState(),
        IsStopping(),
      ])
      cachedState = nextState
      cachedStopping = nextStopping
    } finally {
      refreshPromise = null
    }
  })()
  await refreshPromise
}

function bindRefreshEvents(refresh: () => void) {
  if (eventsBound) return
  eventsBound = true
  EventsOn('backup-finished', () => { refresh() })
  EventsOn('backup-queued', () => { refresh() })
}

export function preloadExecutionState(): void {
  void fetchExecutionState()
}

export function useExecutionState() {
  const state = ref<models.ExecutionState | null>(cachedState)
  const stopping = ref(cachedStopping)
  const loading = ref(!cachedState)

  async function refresh() {
    await fetchExecutionState()
    state.value = cachedState
    stopping.value = cachedStopping
    loading.value = false
  }

  function prime() {
    applyCache(state, stopping, loading)
  }

  bindRefreshEvents(refresh)

  return { state, stopping, loading, refresh, prime }
}
