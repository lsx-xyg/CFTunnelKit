import { onMounted, onUnmounted, watch, type Ref } from 'vue'

const FOCUSABLE = 'a[href], button:not([disabled]), input:not([disabled]), select:not([disabled]), textarea:not([disabled]), [tabindex]:not([tabindex="-1"])'

// Trap focus inside a modal while it's open.
// Usage: useFocusTrap(modalRef, isOpen)
export function useFocusTrap(elRef: Ref<HTMLElement | null>, isOpen: Ref<boolean>) {
  let previouslyFocused: HTMLElement | null = null

  function getFocusable(): HTMLElement[] {
    if (!elRef.value) return []
    return Array.from(elRef.value.querySelectorAll<HTMLElement>(FOCUSABLE)).filter(
      (el) => el.offsetParent !== null,
    )
  }

  function onKeydown(e: KeyboardEvent) {
    if (e.key !== 'Tab' || !elRef.value) return
    const focusable = getFocusable()
    if (focusable.length === 0) return
    const first = focusable[0]
    const last = focusable[focusable.length - 1]
    const active = document.activeElement as HTMLElement

    if (e.shiftKey) {
      if (active === first || !elRef.value.contains(active)) {
        e.preventDefault()
        last.focus()
      }
    } else {
      if (active === last || !elRef.value.contains(active)) {
        e.preventDefault()
        first.focus()
      }
    }
  }

  watch(isOpen, (open) => {
    if (open) {
      previouslyFocused = document.activeElement as HTMLElement
      requestAnimationFrame(() => {
        const focusable = getFocusable()
        if (focusable.length) focusable[0].focus()
      })
      document.addEventListener('keydown', onKeydown)
    } else {
      document.removeEventListener('keydown', onKeydown)
      previouslyFocused?.focus?.()
    }
  })

  onUnmounted(() => {
    document.removeEventListener('keydown', onKeydown)
  })
}
