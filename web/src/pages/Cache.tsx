import { useEffect, useMemo, useState } from 'react'
import { api, formatBytes, formatTime } from '../api/client'
import StateBlock from '../components/StateBlock'
import { useAsync } from '../hooks/useAsync'

// The cache can hold hundreds of files; the table pages through them so the
// page stays responsive and the file names remain scannable.
const PAGE_SIZE = 35

export default function Cache() {
  const [root, setRoot] = useState('all')
  const [keyword, setKeyword] = useState('')
  const [page, setPage] = useState(1)
  const status = useAsync(() => api.cacheStatus(), [])
  const list = useAsync(() => api.cache(root), [root])

  const files = useMemo(() => list.data?.files ?? [], [list.data])

  const filtered = useMemo(() => {
    const needle = keyword.trim().toLowerCase()
    if (!needle) {
      return files
    }
    return files.filter((file) => file.path.toLowerCase().includes(needle))
  }, [files, keyword])

  const totalPages = Math.max(1, Math.ceil(filtered.length / PAGE_SIZE))
  const currentPage = Math.min(page, totalPages)
  const visible = filtered.slice((currentPage - 1) * PAGE_SIZE, currentPage * PAGE_SIZE)

  // A new filter or scope starts from the first page.
  useEffect(() => {
    setPage(1)
  }, [keyword, root])

  return (
    <section className="page">
      <div className="page-head">
        <h1>Cache</h1>
        <button
          onClick={() => {
            status.reload()
            list.reload()
          }}
          disabled={list.loading}
        >
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
        <input
          type="search"
          placeholder="Filter by file path"
          value={keyword}
          onChange={(event) => setKeyword(event.target.value)}
          style={{ minWidth: 260 }}
        />
        {list.data && (
          <span className="muted">
            {filtered.length} of {files.length} files
          </span>
        )}
      </div>

      <StateBlock
        loading={list.loading}
        error={list.error}
        empty={list.data?.total_files === 0}
        emptyText="No cache files in this scope."
      >
        {list.data && filtered.length === 0 && (
          <div className="notice">No cache file matches this filter.</div>
        )}
        {visible.length > 0 && (
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
              {visible.map((file) => (
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

        {filtered.length > PAGE_SIZE && (
          <div className="pager">
            <button onClick={() => setPage(1)} disabled={currentPage === 1}>
              « First
            </button>
            <button onClick={() => setPage(currentPage - 1)} disabled={currentPage === 1}>
              ‹ Prev
            </button>
            <span className="muted">
              Page {currentPage} / {totalPages} · {PAGE_SIZE} per page
            </span>
            <button onClick={() => setPage(currentPage + 1)} disabled={currentPage >= totalPages}>
              Next ›
            </button>
            <button onClick={() => setPage(totalPages)} disabled={currentPage >= totalPages}>
              Last »
            </button>
          </div>
        )}
      </StateBlock>
    </section>
  )
}
