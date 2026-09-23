import { useState } from 'react'
import { api, formatBytes, formatTime } from '../api/client'
import StateBlock from '../components/StateBlock'
import { useAsync } from '../hooks/useAsync'

export default function Cache() {
  const [root, setRoot] = useState('all')
  const status = useAsync(() => api.cacheStatus(), [])
  const list = useAsync(() => api.cache(root), [root])

  return (
    <section className="page">
      <div className="page-head">
        <h1>Cache</h1>
        <button onClick={() => { status.reload(); list.reload() }} disabled={list.loading}>
          Refresh
        </button>
      </div>

      <StateBlock loading={status.loading} error={status.error}>
        {status.data && (
          <div className="cards">
            <div className="card">
              <b>{status.data.total_files}</b>
              <span>Files</span>
            </div>
            <div className="card">
              <b>{formatBytes(status.data.total_size)}</b>
              <span>Total size</span>
            </div>
            <div className="card">
              <b>{status.data.kinds['pkg']?.files ?? 0}</b>
              <span>Package files</span>
            </div>
            <div className="card">
              <b>{status.data.kinds['sdk']?.files ?? 0}</b>
              <span>SDK files</span>
            </div>
          </div>
        )}
      </StateBlock>

      {status.data && (
        <div className="panel">
          <h2>Mirror</h2>
          <dl className="detail-grid">
            <dt>Cache dir</dt>
            <dd className="mono">{status.data.cache_dir}</dd>
            <dt>Client mirror</dt>
            <dd>
              {status.data.cache_mirror.enable
                ? `${status.data.cache_mirror.url ?? ''} (fallback ${status.data.cache_mirror.fallback ? 'on' : 'off'})`
                : 'disabled'}
            </dd>
            <dt>Manifest</dt>
            <dd>
              <a href="/manifest.json">/manifest.json</a>
            </dd>
          </dl>
        </div>
      )}

      <div className="toolbar">
        <select value={root} onChange={(event) => setRoot(event.target.value)}>
          <option value="all">all</option>
          <option value="pkg">pkg</option>
          <option value="api">api</option>
          <option value="sdk">sdk</option>
          <option value="sdk-index">sdk-index</option>
        </select>
      </div>

      <StateBlock
        loading={list.loading}
        error={list.error}
        empty={list.data?.total_files === 0}
        emptyText="No cache files in this scope."
      >
        {list.data && (
          <table>
            <thead>
              <tr>
                <th>Kind</th>
                <th>Path</th>
                <th>Size</th>
                <th>Modified</th>
                <th />
              </tr>
            </thead>
            <tbody>
              {list.data.files.map((file) => (
                <tr key={file.path}>
                  <td>
                    <span className="tag">{file.kind}</span>
                  </td>
                  <td className="mono">{file.path}</td>
                  <td>{formatBytes(file.size)}</td>
                  <td className="muted">{formatTime(file.mod_time)}</td>
                  <td>
                    <a href={`/files/${file.path}`}>Download</a>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </StateBlock>
    </section>
  )
}
