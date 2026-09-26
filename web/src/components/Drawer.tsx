import { useId, useRef, type ReactNode } from 'react'
import { useModal } from '../hooks/useModal'

interface DrawerProps {
  title: string
  onClose: () => void
  children: ReactNode
}

// Drawer shows a record beside the list it came from, so the list keeps its
// filters, its page and its scroll position while the record is open.
export default function Drawer({ title, onClose, children }: DrawerProps) {
  const panel = useRef<HTMLElement>(null)
  const titleId = useId()
  const onKeyDown = useModal(panel, onClose)

  return (
    <div
      className="drawer-backdrop"
      onMouseDown={(event) => {
        if (event.target === event.currentTarget) {
          onClose()
        }
      }}
    >
      <aside
        className="drawer"
        role="dialog"
        aria-modal="true"
        aria-labelledby={titleId}
        tabIndex={-1}
        ref={panel}
        onKeyDown={onKeyDown}
      >
        <header className="drawer-head">
          <h2 id={titleId}>{title}</h2>
          <button onClick={onClose}>Close</button>
        </header>
        <div className="drawer-body">{children}</div>
      </aside>
    </div>
  )
}
