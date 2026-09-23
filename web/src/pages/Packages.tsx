import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { api, formatTime } from '../api/client'
import StateBlock from '../components/StateBlock'
import { useAsync } from '../hooks/useAsync'

export default function Packages() {
  const navigate = useNavigate()
  const [scope, setScope] = useState('all')
  const [keyword, setKeyword] = useState('')
  const [installedOnly, setInstalledOnly] = useState(false)
  const [applied, setApplied] = useState({ scope: 'all', q: '', installed: false })

  const { data, error, loading, reload } = useAsync(
    () => api.packages({ scope: applied.scope, q: applied.q, installed: applied.installed }),
    [applied.scope, applied.q, applied.installed],
  )

  const applyFilters = () => {
    setApplied({ scope, q: keyword.trim(), installed: installedOnly })
  }

  return (
    <section className="page">
      <div className="page-head">
        <h1>Packages</h1>
        <button onClick={reload} disabled={loading}>
          Refresh
        </button>
      </div>

      <div className="toolbar">
        <input
          type="search"
          placeholder="Filter by name or repo"
          value={keyword}
          onChange={(event) => setKeyword(event.target.value)}
          onKeyDown={(event) => {
            if (event.key === 'Enter') {
              applyFilters()
            }
          }}
        />
        <select value={scope} onChange={(event) => setScope(event.target.value)}>
          <option value="all">all sources</option>
          <option value="eget">eget only</option>
          <option value="ext">external only</option>
        </select>
        <label className="check">
          <input
            type="checkbox"
            checked={installedOnly}
            onChange={(event) => setInstalledOnly(event.target.checked)}
          />
          installed only
        </label>
        <button onClick={applyFilters}>Apply</button>
      </div>

      <StateBlock loading={loading} error={error} empty={data?.total === 0} emptyText="No packages match the filters.">
        {data && (
          <table>
            <thead>
              <tr>
                <th>Name</th>
                <th>Source</th>
                <th>Repo</th>
                <th>Version</th>
                <th>Installed</th>
                <th>Installed at</th>
              </tr>
            </thead>
            <tbody>
              {data.items.map((item) => (
                <tr
                  key={`${item.source}:${item.name}`}
                  className="clickable"
                  onClick={() => navigate(`/packages/${encodeURIComponent(item.name)}`)}
                >
                  <td>
                    {item.name}
                    {item.isGui && <span className="tag gui" style={{ marginLeft: 8 }}>gui</span>}
                  </td>
                  <td>
                    <span className="tag">{item.source}</span>
                  </td>
                  <td className="mono muted">{item.repo}</td>
                  <td className="mono">{item.version || item.tag || '-'}</td>
                  <td>{item.installed ? 'yes' : 'no'}</td>
                  <td className="muted">{formatTime(item.installedAt)}</td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </StateBlock>
    </section>
  )
}
