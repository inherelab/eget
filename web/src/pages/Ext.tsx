import { useState } from 'react'
import { api } from '../api/client'
import StateBlock from '../components/StateBlock'
import { useAsync } from '../hooks/useAsync'
import type { ExtPackage } from '../api/types'

export default function Ext() {
  const { data, error, loading, reload } = useAsync(() => api.ext(), [])
  const [selected, setSelected] = useState('')
  const [packages, setPackages] = useState<ExtPackage[]>([])
  const [packagesError, setPackagesError] = useState<string | null>(null)
  const [packagesLoading, setPackagesLoading] = useState(false)

  const openManager = async (manager: string) => {
    setSelected(manager)
    setPackagesLoading(true)
    setPackagesError(null)
    try {
      const result = await api.extPackages(manager)
      setPackages(result.packages)
    } catch (err) {
      setPackages([])
      setPackagesError(err instanceof Error ? err.message : String(err))
    } finally {
      setPackagesLoading(false)
    }
  }

  return (
    <section className="page">
      <div className="page-head">
        <h1>External managers</h1>
        <button onClick={reload} disabled={loading}>
          Refresh
        </button>
      </div>

      <StateBlock loading={loading} error={error}>
        {data && (
          <>
            <table>
              <thead>
                <tr>
                  <th>Manager</th>
                  <th>Available</th>
                  <th>Packages</th>
                  <th>Bin</th>
                  <th />
                </tr>
              </thead>
              <tbody>
                {data.managers.map((entry) => (
                  <tr key={entry.manager}>
                    <td>
                      <span className="tag">{entry.manager}</span>
                    </td>
                    <td>{entry.available ? 'yes' : 'no'}</td>
                    <td>{entry.packages}</td>
                    <td className="mono muted">{entry.bin ?? '-'}</td>
                    <td>
                      <button
                        onClick={() => openManager(entry.manager)}
                        disabled={!entry.available || packagesLoading}
                      >
                        Packages
                      </button>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>

            {data.failures && data.failures.length > 0 && (
              <div className="panel">
                <h2>Manager failures</h2>
                <ul>
                  {data.failures.map((failure) => (
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

      {selected && (
        <div className="panel" style={{ marginTop: 18 }}>
          <h2>{selected} packages</h2>
          <StateBlock
            loading={packagesLoading}
            error={packagesError}
            empty={packages.length === 0}
            emptyText="This manager reports no packages."
          >
            <table>
              <thead>
                <tr>
                  <th>Name</th>
                  <th>Version</th>
                  <th>Latest</th>
                </tr>
              </thead>
              <tbody>
                {packages.map((pkg) => (
                  <tr key={pkg.name}>
                    <td>{pkg.name}</td>
                    <td className="mono">{pkg.version || '-'}</td>
                    <td className="mono">{pkg.latest ? <span className="tag warn">{pkg.latest}</span> : '-'}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </StateBlock>
        </div>
      )}
    </section>
  )
}
