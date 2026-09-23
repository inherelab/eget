import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { api, formatTime } from '../api/client'
import StateBlock from '../components/StateBlock'
import type { OutdatedResponse } from '../api/types'

export default function Outdated() {
  const navigate = useNavigate()
  const [result, setResult] = useState<OutdatedResponse | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [loading, setLoading] = useState(false)
  const [submitting, setSubmitting] = useState(false)

  const updateAll = async () => {
    setSubmitting(true)
    setError(null)
    try {
      const accepted = await api.submitUpdate({ all: true })
      navigate(`/tasks?task=${encodeURIComponent(accepted.taskId)}`)
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err))
    } finally {
      setSubmitting(false)
    }
  }

  const check = async () => {
    setLoading(true)
    setError(null)
    try {
      setResult(await api.outdated())
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err))
    } finally {
      setLoading(false)
    }
  }

  return (
    <section className="page">
      <div className="page-head">
        <h1>Outdated</h1>
        <div>
          <button onClick={() => void updateAll()} disabled={submitting} style={{ marginRight: 8 }}>
            Update all
          </button>
          <button className="primary" onClick={check} disabled={loading}>
            {loading ? 'Checking…' : 'Check for updates'}
          </button>
        </div>
      </div>

      <div className="notice">
        Checking queries every installed package and each external manager, so it can take a while.
      </div>

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
                    <th>Name</th>
                    <th>Source</th>
                    <th>Current</th>
                    <th>Latest</th>
                    <th>Published</th>
                  </tr>
                </thead>
                <tbody>
                  {result.items.map((item) => (
                    <tr key={`${item.source}:${item.name}`}>
                      <td>{item.name}</td>
                      <td>
                        <span className="tag">{item.source}</span>
                      </td>
                      <td className="mono">{item.installedTag || '-'}</td>
                      <td className="mono">
                        <span className="tag warn">{item.latestTag}</span>
                      </td>
                      <td className="muted">{formatTime(item.publishedAt)}</td>
                    </tr>
                  ))}
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
