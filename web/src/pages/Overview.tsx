import { api, formatBytes } from '../api/client'
import StateBlock from '../components/StateBlock'
import { useAsync } from '../hooks/useAsync'

export default function Overview() {
  const { data, error, loading, reload } = useAsync(() => api.overview(), [])

  return (
    <section className="page">
      <div className="page-head">
        <h1>Overview</h1>
        <button onClick={reload} disabled={loading}>
          Refresh
        </button>
      </div>
      <StateBlock loading={loading} error={error}>
        {data && (
          <>
            <div className="cards">
              <div className="card">
                <b>{data.packages}</b>
                <span>Packages</span>
              </div>
              <div className="card">
                <b>{data.installed}</b>
                <span>Installed</span>
              </div>
              <div className="card">
                <b>
                  {data.ext.filter((entry) => entry.available).length}/{data.ext.length}
                </b>
                <span>Ext managers ready</span>
              </div>
              <div className="card">
                <b>{data.cache ? formatBytes(data.cache.size) : '-'}</b>
                <span>Cache size</span>
              </div>
            </div>

            <div className="panel">
              <h2>Runtime</h2>
              <dl className="detail-grid">
                <dt>Version</dt>
                <dd>{data.version}</dd>
                <dt>Config</dt>
                <dd className="mono">
                  {data.configPath} {data.configExists ? '' : '(missing)'}
                </dd>
                <dt>Cache dir</dt>
                <dd className="mono">{data.cache?.dir ?? '-'}</dd>
                <dt>Cache files</dt>
                <dd>{data.cache?.files ?? 0}</dd>
                <dt>Tasks</dt>
                <dd>
                  running {data.tasks.running}, queued {data.tasks.queued}
                </dd>
              </dl>
            </div>

            <div className="panel">
              <h2>External managers</h2>
              <table>
                <thead>
                  <tr>
                    <th>Manager</th>
                    <th>Available</th>
                    <th>Bin</th>
                  </tr>
                </thead>
                <tbody>
                  {data.ext.map((entry) => (
                    <tr key={entry.manager}>
                      <td>
                        <span className="tag">{entry.manager}</span>
                      </td>
                      <td>{entry.available ? 'yes' : 'no'}</td>
                      <td className="mono muted">{entry.bin ?? '-'}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          </>
        )}
      </StateBlock>
    </section>
  )
}
