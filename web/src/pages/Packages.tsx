import { useState } from 'react'
import { api, formatTime } from '../api/client'
import Drawer from '../components/Drawer'
import PackagePanel from '../components/PackagePanel'
import StateBlock from '../components/StateBlock'
import { useAsync } from '../hooks/useAsync'

export default function Packages() {
  const [scope, setScope] = useState('eget')
  const [keyword, setKeyword] = useState('')
  const [installedOnly, setInstalledOnly] = useState(false)
  const [applied, setApplied] = useState({ scope: 'eget', q: '', installed: false })
  // The open package lives in a drawer, so the list behind it keeps its filters,
  // its rows and its scroll position.
  const [opened, setOpened] = useState('')

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
                <tr key={`${item.source}:${item.name}`} className="clickable" onClick={() => setOpened(item.name)}>
                  <td>
                    <button
                      className="row-open"
                      onClick={(event) => {
                        // The row itself opens the drawer; the name button makes
                        // the same thing reachable from the keyboard.
                        event.stopPropagation()
                        setOpened(item.name)
                      }}
                    >
                      {item.name}
                    </button>
                    {item.isGui && <span className="tag gui">gui</span>}
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

      {opened && (
        <Drawer
          title={opened}
          onClose={() => {
            setOpened('')
          }}
        >
          <PackagePanel
            name={opened}
            onChanged={(change) => {
              // Either way the row behind the drawer changed, so re-read the
              // list in place (filters and page are kept) and close on uninstall.
              void reload()
              if (change === 'uninstalled') {
                setOpened('')
              }
            }}
          />
        </Drawer>
      )}
    </section>
  )
}
