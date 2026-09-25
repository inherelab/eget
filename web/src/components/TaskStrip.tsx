import { useEffect, useRef, useState } from 'react'
import { Link } from 'react-router-dom'
import { api, subscribeToTask } from '../api/client'
import type { TaskProgress, TaskStatus } from '../api/types'

interface TaskStripProps {
  taskId: string
  onDismiss: () => void
  // onFinished runs once with the terminal status: pages refresh what the task
  // changed, but never on their own initiative (see the Outdated page).
  onFinished?: (status: TaskStatus) => void
}

const statusClass: Record<TaskStatus, string> = {
  queued: 'tag',
  running: 'tag live',
  succeeded: 'tag',
  failed: 'tag fail',
  canceled: 'tag warn',
  interrupted: 'tag warn',
}

const finished = (status: TaskStatus) =>
  status === 'succeeded' || status === 'failed' || status === 'canceled' || status === 'interrupted'

// TaskStrip reports a submitted task where it was started, so a failed install
// or update does not cost a trip to the Tasks page. The full record stays one
// link away, and the strip is dismissed by hand so the outcome stays readable.
export default function TaskStrip({ taskId, onDismiss, onFinished }: TaskStripProps) {
  const [kind, setKind] = useState('')
  const [what, setWhat] = useState('')
  const [status, setStatus] = useState<TaskStatus>('queued')
  const [percent, setPercent] = useState(0)
  const [note, setNote] = useState('')
  const [failure, setFailure] = useState('')
  const reported = useRef(false)

  useEffect(() => {
    let current = true
    reported.current = false
    setKind('')
    setWhat('')
    setStatus('queued')
    setPercent(0)
    setNote('')
    setFailure('')

    void api
      .task(taskId)
      .then((task) => {
        if (!current) {
          return
        }
        setKind(task.kind)
        setWhat(taskSubject(task.params))
        setStatus(task.status)
        setPercent(task.progress?.percent ?? 0)
        setNote(task.logs?.at(-1)?.message ?? '')
        setFailure(task.error ?? '')
      })
      .catch(() => {
        // The stream still reports the outcome when the initial read fails.
      })

    const close = subscribeToTask(taskId, (name, payload) => {
      if (name === 'log') {
        setNote((payload as { message?: string }).message ?? '')
        return
      }
      if (name === 'progress') {
        setPercent((payload as TaskProgress).percent ?? 0)
        return
      }
      const frame = payload as { status?: TaskStatus; error?: string }
      if (frame.status) {
        setStatus(frame.status)
      }
      if (frame.error) {
        setFailure(frame.error)
      }
    })

    return () => {
      current = false
      close()
    }
  }, [taskId])

  const done = finished(status)

  useEffect(() => {
    if (!done || reported.current) {
      return
    }
    reported.current = true
    onFinished?.(status)
  }, [done, status, onFinished])

  return (
    <section className={done ? 'task-strip finished' : 'task-strip'} aria-live="polite">
      <div className="task-line">
        <span className={statusClass[status] ?? 'tag'}>{status}</span>
        <span className="task-what">{[kind, what].filter(Boolean).join(' ')}</span>
        <span className="task-percent">{done ? '' : `${Math.round(percent)}%`}</span>
        <Link to={`/tasks?task=${encodeURIComponent(taskId)}`}>Open in Tasks</Link>
        <button onClick={onDismiss}>Dismiss</button>
      </div>
      {failure ? (
        <p className="task-failure">{failure}</p>
      ) : (
        note && <p className="task-note">{note}</p>
      )}
      {!done && (
        <div className="task-track">
          <i style={{ width: `${Math.max(2, Math.min(100, percent))}%` }} />
        </div>
      )}
    </section>
  )
}

// taskSubject names what the task works on: a single target, a target list, or
// nothing when the task covers everything.
function taskSubject(params?: Record<string, unknown>): string {
  if (!params) {
    return ''
  }
  if (Array.isArray(params.targets) && params.targets.length > 0) {
    return params.targets.join(', ')
  }
  if (typeof params.target === 'string') {
    return params.target
  }
  return ''
}
