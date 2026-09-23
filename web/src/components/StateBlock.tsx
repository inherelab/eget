import type { ReactNode } from 'react'

interface StateBlockProps {
  loading: boolean
  error: string | null
  empty?: boolean
  emptyText?: string
  children?: ReactNode
}

// StateBlock renders the shared loading/error/empty skeleton around page data.
export default function StateBlock({ loading, error, empty, emptyText, children }: StateBlockProps) {
  if (error) {
    return <div className="alert">{error}</div>
  }
  if (loading) {
    return <div className="notice">Loading…</div>
  }
  if (empty) {
    return <div className="notice">{emptyText ?? 'Nothing to show.'}</div>
  }
  return <>{children}</>
}
