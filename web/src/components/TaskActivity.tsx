import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { api } from '../api/client'
import { tasksSeenAt } from '../lib/tasks'

const pollMs = 5000

// TaskActivity watches the task queue from anywhere in the console: how many
// tasks are still running, and how many finished badly since the Tasks page was
// last opened. It is the console's only global notice, and it never asks the
// reader a question: an operation that needs a decision is stopped by the
// console itself, not left waiting for an answer.
export default function TaskActivity() {
  const [counts, setCounts] = useState({ running: 0, failed: 0 })

  useEffect(() => {
    let cancelled = false
    const poll = async () => {
      try {
        const data = await api.tasks(50)
        if (cancelled) {
          return
        }
        const seen = tasksSeenAt()
        setCounts({
          running: data.items.filter((task) => task.status === 'running' || task.status === 'queued').length,
          failed: data.items.filter(
            (task) =>
              (task.status === 'failed' || task.status === 'interrupted') &&
              Date.parse(task.finishedAt ?? task.createdAt) > seen,
          ).length,
        })
      } catch {
        // A read-only or unreachable console simply shows nothing.
      }
    }
    void poll()
    const timer = window.setInterval(poll, pollMs)
    return () => {
      cancelled = true
      window.clearInterval(timer)
    }
  }, [])

  if (counts.running === 0 && counts.failed === 0) {
    return null
  }

  return (
    <Link className="task-activity" to="/tasks" title="Tasks">
      {counts.running > 0 && (
        <span className="tag live">
          {counts.running}
          <span className="activity-word"> running</span>
        </span>
      )}
      {counts.failed > 0 && (
        <span className="tag fail">
          {counts.failed}
          <span className="activity-word"> failed</span>
        </span>
      )}
    </Link>
  )
}
