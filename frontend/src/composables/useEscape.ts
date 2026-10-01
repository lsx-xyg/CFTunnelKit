import { onMounted, onUnmounted, watch, type Ref } from 'vue'

// Close a modal/panel on Escape key press.
// Usage: useEscape(() => closeDialog(), isOpen)
export function useEscape(onEscape: () => void, isOpen: Ref<boolean>) {
  function handler(e: KeyboardEvent) {
    if (e.key === 'Escape' && isOpen.value) {
      onEscape()
    }
  }
  onMounted(() => document.addEventListener('keydown', handler))
  onUnmounted(() => document.removeEventListener('keydown', handler))
  watch(isOpen, (v) => {
    if (v) document.addEventListener('keydown', handler)
    else document.removeEventListener('keydown', handler)
  })
}
