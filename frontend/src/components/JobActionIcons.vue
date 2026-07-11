<script setup lang="ts">
defineProps<{
  runTitle: string
  fullTitle: string
  editTitle: string
  deleteTitle: string
}>()
defineEmits<{
  run: []
  full: []
  edit: []
  remove: []
}>()
</script>

<template>
  <div class="job-actions">
    <button type="button" class="btn icon-only primary" :title="runTitle" :aria-label="runTitle" @click="$emit('run')">
      <svg viewBox="0 0 20 20" fill="currentColor" aria-hidden="true">
        <path d="M6.3 4.2a1 1 0 0 1 1.52-.85l7.5 4.48a1 1 0 0 1 0 1.7l-7.5 4.48A1 1 0 0 1 6.3 13V4.2z" />
      </svg>
    </button>
    <button type="button" class="btn icon-only ghost" :title="fullTitle" :aria-label="fullTitle" @click="$emit('full')">
      <svg viewBox="0 0 20 20" fill="currentColor" aria-hidden="true">
        <path fill-rule="evenodd" d="M4 2a1 1 0 0 1 1 1v2.101a7.002 7.002 0 0 1 11.601 2.566 1 1 0 1 1-1.885.666A5.002 5.002 0 0 0 5.999 7H9a1 1 0 0 1 0 2H4a1 1 0 0 1-1-1V3a1 1 0 0 1 1-1zm.008 9.057a1 1 0 0 1 1.276.61A5.002 5.002 0 0 0 13.999 13H11a1 1 0 1 1 0-2h5a1 1 0 0 1 1 1v5a1 1 0 1 1-2 0v-2.101a7.002 7.002 0 0 1-11.601-2.566 1 1 0 0 1 .61-1.276z" clip-rule="evenodd" />
      </svg>
    </button>
    <button type="button" class="btn icon-only ghost" :title="editTitle" :aria-label="editTitle" @click="$emit('edit')">
      <svg viewBox="0 0 20 20" fill="currentColor" aria-hidden="true">
        <path d="M13.586 3.586a2 2 0 0 1 2.828 2.828l-8.5 8.5a1 1 0 0 1-.397.242l-3.5 1.167a.5.5 0 0 1-.632-.632l1.167-3.5a1 1 0 0 1 .242-.397l8.5-8.5z" />
      </svg>
    </button>
    <button type="button" class="btn icon-only danger" :title="deleteTitle" :aria-label="deleteTitle" @click="$emit('remove')">
      <svg viewBox="0 0 20 20" fill="currentColor" aria-hidden="true">
        <path fill-rule="evenodd" d="M8.75 1A2.75 2.75 0 0 0 6 3.75v.443c-.795.077-1.584.176-2.365.298a.75.75 0 1 0 .23 1.482l.149-.022.841 9.52A2.75 2.75 0 0 0 7.596 18h4.807a2.75 2.75 0 0 0 2.742-2.53l.841-9.52.149.023a.75.75 0 0 0 .23-1.482A41.03 41.03 0 0 0 14 4.193V3.75A2.75 2.75 0 0 0 11.25 1h-2.5zM8 3.75c0-.138.112-.25.25-.25h3.5a.25.25 0 0 1 .25.25v.443a43.076 43.076 0 0 0-4 0v-.443zm1.49 5.653a.75.75 0 1 0-1.49.194L8.59 14.5H7.75a.75.75 0 0 0 0 1.5h4.5a.75.75 0 0 0 0-1.5h-.84l.19-5.153z" clip-rule="evenodd" />
      </svg>
    </button>
  </div>
</template>
