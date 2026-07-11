import { ref } from 'vue'

/** Закрытие по клику на фон: не срабатывает при перетаскивании мыши из окна наружу. */
export function useBackdropDismiss(onDismiss: () => void) {
  const pointerDownOnBackdrop = ref(false)

  function onBackdropPointerDown(e: PointerEvent) {
    pointerDownOnBackdrop.value = e.target === e.currentTarget
  }

  function onBackdropClick(e: MouseEvent) {
    if (pointerDownOnBackdrop.value && e.target === e.currentTarget) {
      onDismiss()
    }
    pointerDownOnBackdrop.value = false
  }

  return { onBackdropPointerDown, onBackdropClick }
}
