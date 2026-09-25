import { useEffect, useRef, useState } from 'react'
import { api, formatTime } from '../api/client'
import StateBlock from '../components/StateBlock'
import TaskStrip from '../components/TaskStrip'
import { useAsync } from '../hooks/useAsync'
import type { OutdatedItem, OutdatedResponse } from '../api/types'

const rowKey = (item: OutdatedItem) => `${item.source}:${item.name}`

// An update target is what the CLI accepts: the configured name, the repo, or
// manager:package for a package owned by an external manager.
const updateTarget = (item: OutdatedItem) => item.target?.trim() || item.repo || item.name

export default function Outdated() {
  const [scope, setScope] = useState('eget')
  const [manager, setManager] = useState('')
  const [result, setResult] = useState<OutdatedResponse | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [loading, setLoading] = useState(false)
  const [submitting, setSubmitting] = useState(false)
  const [selected, setSelected] = useState<string[]>([])
  const [taskId, setTaskId] = useState('')
  const selectAll = useRef<HTMLInputElement>(null)
  const managers = useAsync(() => api.ext(), [])

  const items = result?.items ?? []
  const allSelected = items.length > 0 && selected.length === items.length

  useEffect(() => {
    if (selectAll.current) {
      selectAll.current.indeterminate = selected.length > 0 && !allSelected
    }
  }, [selected, allSelected])

  const check = async () => {
    setLoading(true)
    setError(null)
    setSelected([])
    try {
      setResult(await api.outdated({ scope, manager: scope === 'ext' ? manager : '' }))
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err))
    } finally {
      setLoading(false)
    }
  }

  const runUpdate = async (submit: () => Promise<{ taskId: string }>) => {
    setSubmitting(true)
    setError(null)
    try {
      const accepted = await submit()
      setTaskId(accepted.taskId)
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err))
    } finally {
      setSubmitting(false)
    }
  }

  const selectedTargets = items.filter((item) => selected.includes(rowKey(item))).map(updateTarget)

  const scopeLabel = scope === 'eget' ? 'eget packages' : scope === 'ext' ? 'external packages' : 'everything'

  return (
    <section className="page">
      <div className="page-head">
        <h1>Outdated</h1>
        <div>
          <button
            onClick={() => void runUpdate(() => api.submitUpdate({ all: true }))}
            disabled={submitting}
            style={{ marginRight: 8 }}
          >
            Update all
          </button>
          <button className="primary" onClick={check} disabled={loading}>
            {loading ? 'Checking…' : 'Check for updates'}
          </button>
        </div>
      </div>

      <div className="toolbar">
        <select value={scope} onChange={(event) => setScope(event.target.value)}>
          <option value="eget">eget packages only</option>
          <option value="ext">external packages only</option>
          <option value="all">everything</option>
        </select>
        {scope === 'ext' && (
          <select value={manager} onChange={(event) => setManager(event.target.value)}>
            <option value="">all managers</option>
            {(managers.data?.managers ?? [])
              .filter((entry) => entry.available)
              .map((entry) => (
                <option key={entry.manager} value={entry.manager}>
                  {entry.manager}
                </option>
              ))}
          </select>
        )}
        {selected.length > 0 && (
          <button
            className="primary"
            disabled={submitting}
            onClick={() => void runUpdate(() => api.submitUpdate({ targets: selectedTargets }))}
          >
            Update selected ({selected.length})
          </button>
        )}
      </div>

      <div className="notice">
        Checking queries {scopeLabel}
        {scope === 'ext' ? ' through their own manager commands' : ''}, so it can take a while. The result below
        reflects the scope selected at the last check.
      </div>

      {taskId && (
        <TaskStrip
          taskId={taskId}
          onDismiss={() => {
            setTaskId('')
          }}
          onFinished={(status) => {
            // The list just changed underneath the report, so ask again; the
            // check also clears the selection.
            if (status === 'succeeded') {
              void check()
            }
          }}
        />
      )}

      <StateBlock loading={loading} error={error} empty={result?.items.length === 0} emptyText="Everything is up to date.">
        {result && (
          <>
            <div className="cards">
              <div className="card">
                <b>{result.items.length}</b>
                <span>Outdated</span>
              </div>
              <div className="card">
                <b>{result.checked}</b>
                <span>Checked</span>
              </div>
              <div className="card">
                <b>{result.failures?.length ?? 0}</b>
                <span>Failures</span>
              </div>
            </div>

            {result.items.length > 0 && (
              <table>
                <thead>
                  <tr>
                    <th>
                      <input
                        ref={selectAll}
                        type="checkbox"
                        aria-label="Select every outdated package"
                        checked={allSelected}
                        onChange={(event) => setSelected(event.target.checked ? items.map(rowKey) : [])}
                      />
                    </th>
                    <th>Name</th>
                    <th>Source</th>
                    <th>Current</th>
                    <th>Latest</th>
                    <th>Published</th>
                    <th />
                  </tr>
                </thead>
                <tbody>
                  {items.map((item) => {
                    const key = rowKey(item)
                    return (
                      <tr key={key}>
                        <td>
                          <input
                            type="checkbox"
                            aria-label={`Select ${item.name}`}
                            checked={selected.includes(key)}
                            onChange={(event) =>
                              setSelected((previous) =>
                                event.target.checked
                                  ? [...previous, key]
                                  : previous.filter((entry) => entry !== key),
                              )
                            }
                          />
                        </td>
                        <td>{item.name}</td>
                        <td>
                          <span className="tag">{item.source}</span>
                        </td>
                        <td className="mono">{item.installedTag || '-'}</td>
                        <td className="mono">
                          <span className="tag warn">{item.latestTag}</span>
                        </td>
                        <td className="muted">{formatTime(item.publishedAt)}</td>
                        <td>
                          <button
                            disabled={submitting}
                            onClick={() => void runUpdate(() => api.submitUpdate({ targets: [updateTarget(item)] }))}
                          >
                            Update
                          </button>
                        </td>
                      </tr>
                    )
                  })}
                </tbody>
              </table>
            )}

            {result.failures && result.failures.length > 0 && (
              <div className="panel">
                <h2>Check failures</h2>
                <ul>
                  {result.failures.map((failure) => (
                    <li key={`${failure.name}:${failure.error}`} className="mono">
                      {failure.name}: {failure.error}
                    </li>
                  ))}
                </ul>
              </div>
            )}
          </>
        )}
      </StateBlock>
    </section>
  )
}
