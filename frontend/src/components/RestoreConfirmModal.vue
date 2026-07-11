<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { resolveOriginalDest } from '../utils/restoreDest'
import { useBackdropDismiss } from '../composables/useBackdropDismiss'

const props = defineProps<{
  open: boolean
  jobName: string
  sources: string[]
  filePaths: string[]
  itemCount: number
}>()

const emit = defineEmits<{
  confirm: []
  cancel: []
}>()

const { onBackdropPointerDown, onBackdropClick } = useBackdropDismiss(() => emit('cancel'))

const { t } = useI18n()

const preview = computed(() =>
  props.filePaths.slice(0, 12).map((p) => ({
    from: p,
    to: resolveOriginalDest(p, props.sources),
  })),
)

const moreCount = computed(() => Math.max(0, props.filePaths.length - 12))
</script>

<template>
  <Teleport to="body">
    <div
      v-if="open"
      class="confirm-backdrop"
      @pointerdown.self="onBackdropPointerDown"
      @click.self="onBackdropClick"
    >
      <div class="confirm-panel" role="dialog" aria-modal="true">
        <div class="confirm-icon" aria-hidden="true">
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.75">
            <path d="M12 9v4m0 4h.01M10.29 3.86 1.82 18a2 2 0 0 0 1.71 3h16.94a2 2 0 0 0 1.71-3L13.71 3.86a2 2 0 0 0-3.42 0z" stroke-linecap="round" stroke-linejoin="round"/>
          </svg>
        </div>
        <h3>{{ t('restore.confirm_original_title') }}</h3>
        <p class="confirm-lead">{{ t('restore.confirm_original_lead') }}</p>

        <div class="confirm-meta">
          <div class="meta-row">
            <span class="meta-label">{{ t('restore.confirm_job') }}</span>
            <span class="meta-value">{{ jobName }}</span>
          </div>
          <div class="meta-row" v-if="sources.length">
            <span class="meta-label">{{ t('restore.confirm_sources') }}</span>
            <ul class="meta-sources">
              <li v-for="s in sources" :key="s">{{ s }}</li>
            </ul>
          </div>
          <div class="meta-row">
            <span class="meta-label">{{ t('restore.confirm_files') }}</span>
            <span class="meta-value">{{ filePaths.length }} {{ t('restore.confirm_files_unit') }}</span>
          </div>
        </div>

        <div class="path-preview">
          <div class="path-preview-title">{{ t('restore.confirm_preview') }}</div>
          <ul>
            <li v-for="row in preview" :key="row.from">
              <span class="path-from" :title="row.from">{{ row.from }}</span>
              <span class="path-arrow">→</span>
              <span class="path-to" :title="row.to">{{ row.to }}</span>
            </li>
          </ul>
          <p v-if="moreCount" class="path-more">
            {{ t('restore.confirm_more', { n: moreCount }) }}
          </p>
        </div>

        <p class="confirm-warn">{{ t('restore.confirm_warn') }}</p>

        <div class="confirm-actions">
          <button type="button" class="btn ghost" @click="emit('cancel')">
            {{ t('restore.confirm_cancel') }}
          </button>
          <button type="button" class="btn primary" @click="emit('confirm')">
            {{ t('restore.confirm_proceed') }}
          </button>
        </div>
      </div>
    </div>
  </Teleport>
</template>

<style scoped>
.confirm-backdrop {
  position: fixed;
  inset: 0;
  z-index: 200;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 1.5rem;
  background: rgba(2, 6, 23, 0.72);
  backdrop-filter: blur(6px);
}
.confirm-panel {
  width: min(560px, 100%);
  max-height: min(90vh, 720px);
  overflow: auto;
  padding: 1.5rem 1.5rem 1.25rem;
  border-radius: 16px;
  border: 1px solid rgba(148, 163, 184, 0.18);
  background: linear-gradient(165deg, #1e293b 0%, #111827 100%);
  box-shadow: 0 24px 48px rgba(0, 0, 0, 0.45);
}
.confirm-icon {
  width: 2.5rem;
  height: 2.5rem;
  border-radius: 12px;
  display: flex;
  align-items: center;
  justify-content: center;
  background: rgba(234, 179, 8, 0.12);
  color: #fbbf24;
  margin-bottom: 0.85rem;
}
.confirm-icon svg {
  width: 1.35rem;
  height: 1.35rem;
}
h3 {
  margin: 0 0 0.5rem;
  font-size: 1.15rem;
  font-weight: 600;
}
.confirm-lead {
  margin: 0 0 1rem;
  color: var(--muted);
  font-size: 0.9rem;
  line-height: 1.5;
}
.confirm-meta {
  display: flex;
  flex-direction: column;
  gap: 0.65rem;
  padding: 0.85rem 1rem;
  border-radius: 12px;
  background: rgba(15, 23, 42, 0.55);
  border: 1px solid rgba(51, 65, 85, 0.7);
  margin-bottom: 1rem;
}
.meta-row {
  display: grid;
  grid-template-columns: 7.5rem 1fr;
  gap: 0.5rem;
  font-size: 0.85rem;
}
.meta-label {
  color: var(--muted);
}
.meta-value {
  word-break: break-all;
}
.meta-sources {
  margin: 0;
  padding: 0;
  list-style: none;
}
.meta-sources li {
  word-break: break-all;
  padding: 0.1rem 0;
}
.path-preview {
  margin-bottom: 0.85rem;
}
.path-preview-title {
  font-size: 0.72rem;
  text-transform: uppercase;
  letter-spacing: 0.06em;
  color: var(--muted);
  margin-bottom: 0.5rem;
  font-weight: 600;
}
.path-preview ul {
  margin: 0;
  padding: 0;
  list-style: none;
  max-height: 200px;
  overflow: auto;
  border-radius: 10px;
  border: 1px solid rgba(51, 65, 85, 0.6);
  background: rgba(15, 23, 42, 0.4);
}
.path-preview li {
  display: grid;
  grid-template-columns: 1fr auto 1fr;
  gap: 0.35rem;
  align-items: center;
  padding: 0.45rem 0.65rem;
  font-size: 0.78rem;
  border-bottom: 1px solid rgba(51, 65, 85, 0.35);
}
.path-preview li:last-child {
  border-bottom: none;
}
.path-from {
  color: var(--muted);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.path-arrow {
  color: #64748b;
  font-size: 0.7rem;
}
.path-to {
  color: #93c5fd;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-family: Consolas, 'Cascadia Mono', monospace;
}
.path-more {
  margin: 0.4rem 0 0;
  font-size: 0.8rem;
  color: var(--muted);
}
.confirm-warn {
  margin: 0 0 1.1rem;
  padding: 0.6rem 0.75rem;
  border-radius: 8px;
  background: rgba(127, 29, 29, 0.2);
  border: 1px solid rgba(185, 28, 28, 0.35);
  color: #fca5a5;
  font-size: 0.82rem;
  line-height: 1.45;
}
.confirm-actions {
  display: flex;
  justify-content: flex-end;
  gap: 0.6rem;
}
</style>
