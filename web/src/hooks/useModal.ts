import { useEffect, type KeyboardEvent, type RefObject } from 'react'

// useModal gives a modal surface the behaviour one expects: Esc closes it, Tab
// cycles inside it, focus moves in on mount and returns to the trigger on
// unmount. The returned handler goes on the panel element.
export function useModal(panel: RefObject<HTMLElement | null>, onClose: () => void) {
  useEffect(() => {
    const previous = document.activeElement
    panel.current?.focus()
    return () => {
      if (previous instanceof HTMLElement) {
        previous.focus()
      }
    }
  }, [panel])

  return (event: KeyboardEvent<HTMLElement>) => {
    if (event.key === 'Escape') {
      onClose()
      return
    }
    if (event.key !== 'Tab') {
      return
    }
    const focusable = panel.current?.querySelectorAll<HTMLElement>(
      'button:not(:disabled), input:not(:disabled), select:not(:disabled), textarea:not(:disabled), a[href]',
    )
    if (!focusable || focusable.length === 0) {
      return
    }
    const first = focusable[0]
    const last = focusable[focusable.length - 1]
    if (event.shiftKey && document.activeElement === first) {
      event.preventDefault()
      last.focus()
    } else if (!event.shiftKey && document.activeElement === last) {
      event.preventDefault()
      first.focus()
    }
  }
}
