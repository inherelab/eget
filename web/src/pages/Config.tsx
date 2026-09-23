import { useState } from 'react'
import { api } from '../api/client'
import StateBlock from '../components/StateBlock'
import { useAsync } from '../hooks/useAsync'
import type { ConfigChange } from '../api/types'

export default function Config() {
  const { data, error, loading, reload } = useAsync(() => api.config(), [])
  const [key, setKey] = useState('')
  const [value, setValue] = useState('')
  const [preview, setPreview] = useState<ConfigChange[] | null>(null)
  const [message, setMessage] = useState<string | null>(null)
  const [actionError, setActionError] = useState<string | null>(null)
  const [busy, setBusy] = useState(false)

  const pending = (): Record<string, string> | null => {
    const trimmed = key.trim()
    if (!trimmed) {
      setActionError('Enter a config key such as global.proxy_url')
      return null
    }
    return { [trimmed]: value }
  }

  const validate = async () => {
    const set = pending()
    if (!set) {
      return
    }
    setBusy(true)
    setActionError(null)
    setMessage(null)
    try {
      const resp = await api.validateConfig(set)
      setPreview(resp.applied)
    } catch (err) {
      setPreview(null)
      setActionError(err instanceof Error ? err.message : String(err))
    } finally {
      setBusy(false)
    }
  }

  const save = async () => {
    const set = pending()
    if (!set) {
      return
    }
    setBusy(true)
    setActionError(null)
    setMessage(null)
    try {
      const resp = await api.updateConfig(set)
      setMessage(`Saved ${resp.applied.map((change) => change.key).join(', ')}`)
      setPreview(null)
      setKey('')
      setValue('')
      reload()
    } catch (err) {
      setActionError(err instanceof Error ? err.message : String(err))
    } finally {
      setBusy(false)
    }
  }

  return (
    <section className="page">
      <div className="page-head">
        <h1>Config</h1>
        <button onClick={reload} disabled={loading}>
          Refresh
        </button>
      </div>

      <StateBlock loading={loading} error={error}>
        {data && (
          <>
            <div className="panel">
              <h2>Location</h2>
              <dl className="detail-grid">
                <dt>Path</dt>
                <dd className="mono">{data.path}</dd>
                <dt>Exists</dt>
                <dd>{data.exists ? 'yes' : 'no'}</dd>
              </dl>
            </div>

            <div className="panel">
              <h2>Edit</h2>
              <div className="toolbar">
                <input
                  type="text"
                  placeholder="key, e.g. global.proxy_url"
                  value={key}
                  onChange={(event) => setKey(event.target.value)}
                  style={{ minWidth: 240 }}
                />
                <input
                  type="text"
                  placeholder="value"
                  value={value}
                  onChange={(event) => setValue(event.target.value)}
                  style={{ minWidth: 240 }}
                  onKeyDown={(event) => {
                    if (event.key === 'Enter') {
                      void validate()
                    }
                  }}
                />
                <button onClick={() => void validate()} disabled={busy}>
                  Preview
                </button>
                <button className="primary" onClick={() => void save()} disabled={busy}>
                  Save
                </button>
              </div>

              {actionError && <div className="alert">{actionError}</div>}
              {message && <div className="notice">{message}</div>}

              {preview && preview.length > 0 && (
                <table>
                  <thead>
                    <tr>
                      <th>Key</th>
                      <th>Current</th>
                      <th>New</th>
                    </tr>
                  </thead>
                  <tbody>
                    {preview.map((change) => (
                      <tr key={change.key}>
                        <td className="mono">{change.key}</td>
                        <td className="mono muted">{change.current ?? '-'}</td>
                        <td className="mono">{change.value === '' ? '(empty)' : change.value}</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              )}
            </div>

            <div className="panel">
              <h2>Content</h2>
              {data.exists ? (
                <pre>{data.content}</pre>
              ) : (
                <div className="notice">No config file yet. Run `eget config init` to create one.</div>
              )}
            </div>
          </>
        )}
      </StateBlock>
    </section>
  )
}
