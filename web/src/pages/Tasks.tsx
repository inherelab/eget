import { useCallback, useEffect, useState } from 'react'
import { useSearchParams } from 'react-router-dom'
import { api, formatTime, subscribeToTask } from '../api/client'
import type { Task, TaskLogLine, TaskProgress, TaskStatus } from '../api/types'
import Drawer from '../components/Drawer'
import StateBlock from '../components/StateBlock'
import { markTasksSeen, taskSubject } from '../lib/tasks'
import { useAsync } from '../hooks/useAsync'

const statusClass: Record<TaskStatus, string> = {
  queued: 'tag',
  running: 'tag live',
  succeeded: 'tag',
  failed: 'tag fail',
  canceled: 'tag warn',
  interrupted: 'tag warn',
}

export default function Tasks() {
  const { data, error, loading, reload } = useAsync(() => api.tasks(50), [])
  const [selected, setSelected] = useState('')
  const [detail, setDetail] = useState<Task | null>(null)
  const [logs, setLogs] = useState<TaskLogLine[]>([])
  const [live, setLive] = useState(false)
  const [actionError, setActionError] = useState<string | null>(null)

  const openTask = useCallback(async (id: string) => {
    setSelected(id)
    setLogs([])
    setActionError(null)
    try {
      const task = await api.task(id)
      setDetail(task)
      setLogs(task.logs ?? [])
    } catch (err) {
      setActionError(err instanceof Error ? err.message : String(err))
    }
  }, [])

  // Pages link here with ?task=<id> for the full record of a task they reported
  // inline.
  const [searchParams, setSearchParams] = useSearchParams()
  const requestedTask = searchParams.get('task') ?? ''
  useEffect(() => {
    if (requestedTask) {
      void openTask(requestedTask)
    }
  }, [requestedTask, openTask])

  // Keep the address in step with the open task, without stacking history.
  useEffect(() => {
    if (selected && selected !== requestedTask) {
      setSearchParams({ task: selected }, { replace: true })
    }
  }, [selected, requestedTask, setSearchParams])

  // Everything this page has shown counts as seen: the console's global notice
  // only reports failures that happened while the reader was elsewhere.
  useEffect(() => {
    if (data) {
      markTasksSeen()
    }
  }, [data])

  // One SSE stream per selected task: logs, progress and status frames keep the
  // panel current while the task runs.
  useEffect(() => {
    if (!selected) {
      return
    }
    setLive(true)
    const close = subscribeToTask(selected, (name, payload) => {
      if (name === 'log') {
        const line = payload as { level?: string; message?: string }
        setLogs((previous) => [
          ...previous.slice(-199),
          {
            time: new Date().toISOString(),
            level: line.level ?? 'info',
            message: line.message ?? '',
          },
        ])
        return
      }
      if (name === 'progress') {
        setDetail((previous) => (previous ? { ...previous, progress: payload as TaskProgress } : previous))
        return
      }
      const frame = payload as { status?: TaskStatus; error?: string }
      if (frame.status) {
        setDetail((previous) => (previous ? { ...previous, status: frame.status as TaskStatus } : previous))
      }
      if (frame.error) {
        setDetail((previous) => (previous ? { ...previous, error: frame.error } : previous))
      }
      if (name === 'done') {
        setLive(false)
        reload()
      }
    })
    return () => {
      close()
      setLive(false)
    }
  }, [selected, reload])

  const cancel = async () => {
    if (!selected) {
      return
    }
    setActionError(null)
    try {
      await api.cancelTask(selected)
      reload()
    } catch (err) {
      setActionError(err instanceof Error ? err.message : String(err))
    }
  }

  return (
    <section className="page">
      <div className="page-head">
        <h1>Tasks</h1>
        <button onClick={reload} disabled={loading}>
          Refresh
        </button>
      </div>

      {actionError && <div className="alert">{actionError}</div>}

      <StateBlock
        loading={loading}
        error={error}
        empty={data?.total === 0}
        emptyText="No task has run yet. Updates, uninstalls and cache cleaning appear here."
      >
        {data && (
          <table>
            <thead>
              <tr>
                <th>Task</th>
                <th>Kind</th>
                <th>Package</th>
                <th>Status</th>
                <th>Progress</th>
                <th>Created</th>
                <th />
              </tr>
            </thead>
            <tbody>
              {data.items.map((task) => (
                <tr key={task.id} className="clickable" onClick={() => void openTask(task.id)}>
                  <td className="mono">{task.id}</td>
                  <td>{task.kind}</td>
                  <td className="mono muted subject" title={taskSubject(task.params)}>
                    {taskSubject(task.params) || '-'}
                  </td>
                  <td>
                    <span className={statusClass[task.status] ?? 'tag'}>{task.status}</span>
                  </td>
                  <td>{Math.round(task.progress?.percent ?? 0)}%</td>
                  <td className="muted">{formatTime(task.createdAt)}</td>
                  <td>
                    <button onClick={(event) => { event.stopPropagation(); void openTask(task.id) }}>
                      Logs
                    </button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </StateBlock>

      {selected && (
        <Drawer
          placement="bottom"
          title={`task ${selected}`}
          onClose={() => {
            setSelected('')
          }}
        >
          <div className="task-line">
            <span className={detail ? statusClass[detail.status] ?? 'tag' : 'tag'}>
              {detail?.status ?? 'loading'}
            </span>
            <span className="task-what">{taskSubject(detail?.params)}</span>
            {live && <span className="tag live">live</span>}
            <span className="task-percent">{Math.round(detail?.progress?.percent ?? 0)}%</span>
            {detail?.status === 'running' || detail?.status === 'queued' ? (
              <button onClick={cancel}>Cancel</button>
            ) : null}
          </div>

          {detail?.error && <div className="alert">{detail.error}</div>}

          <div className="panel">
            <h2>Log</h2>
            <pre>
              {logs.length === 0
                ? 'No log lines yet.'
                : logs
                    .map((line) => `${new Date(line.time).toLocaleTimeString()} [${line.level}] ${line.message}`)
                    .join('\n')}
            </pre>
          </div>

          {detail?.result ? (
            <div className="panel">
              <h2>Result</h2>
              <pre>{JSON.stringify(detail.result, null, 2)}</pre>
            </div>
          ) : null}
        </Drawer>
      )}
    </section>
  )
}
