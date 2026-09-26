import { useId, useRef, type ReactNode } from 'react'
import { useModal } from '../hooks/useModal'

interface DialogProps {
  title: string
  onClose: () => void
  children: ReactNode
}

// Dialog is the console's modal shell for actions that need a decision: it owns
// the backdrop, Esc, the focus trap and focus restore, so callers only provide
// the body and its actions.
export default function Dialog({ title, onClose, children }: DialogProps) {
  const panel = useRef<HTMLDivElement>(null)
  const titleId = useId()
  const onKeyDown = useModal(panel, onClose)

  return (
    <div
      className="dialog-backdrop"
      onMouseDown={(event) => {
        if (event.target === event.currentTarget) {
          onClose()
        }
      }}
    >
      <div
        className="dialog"
        role="dialog"
        aria-modal="true"
        aria-labelledby={titleId}
        tabIndex={-1}
        ref={panel}
        onKeyDown={onKeyDown}
      >
        <h2 id={titleId}>{title}</h2>
        {children}
      </div>
    </div>
  )
}
