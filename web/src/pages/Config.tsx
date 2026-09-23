import { api } from '../api/client'
import StateBlock from '../components/StateBlock'
import { useAsync } from '../hooks/useAsync'

export default function Config() {
  const { data, error, loading, reload } = useAsync(() => api.config(), [])

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
