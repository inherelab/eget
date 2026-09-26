import { useId, useRef, type ReactNode } from 'react'
import { useModal } from '../hooks/useModal'

interface DrawerProps {
  title: string
  onClose: () => void
  // placement picks the edge the drawer slides from: the side for a record that
  // belongs to a list, the bottom for content read alongside the page.
  placement?: 'right' | 'bottom'
  children: ReactNode
}

// Drawer shows a record beside the list it came from, so the list keeps its
// filters, its page and its scroll position while the record is open.
export default function Drawer({ title, onClose, placement = 'right', children }: DrawerProps) {
  const panel = useRef<HTMLElement>(null)
  const titleId = useId()
  const onKeyDown = useModal(panel, onClose)

  return (
    <div
      className={placement === 'bottom' ? 'drawer-backdrop bottom' : 'drawer-backdrop'}
      onMouseDown={(event) => {
        if (event.target === event.currentTarget) {
          onClose()
        }
      }}
    >
      <aside
        className={placement === 'bottom' ? 'drawer bottom' : 'drawer'}
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
