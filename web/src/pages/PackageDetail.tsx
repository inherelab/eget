import { useState } from 'react'
import { Link, useNavigate, useParams } from 'react-router-dom'
import { api, formatTime } from '../api/client'
import Dialog from '../components/Dialog'
import StateBlock from '../components/StateBlock'
import { useAsync } from '../hooks/useAsync'

export default function PackageDetail() {
  const { name = '' } = useParams()
  const navigate = useNavigate()
  const { data, error, loading, reload } = useAsync(() => api.packageDetail(name), [name])
  const [actionError, setActionError] = useState<string | null>(null)
  const [submitting, setSubmitting] = useState(false)
  const [confirming, setConfirming] = useState(false)
  const [purge, setPurge] = useState(false)

  const runTask = async (submit: () => Promise<{ taskId: string }>) => {
    setSubmitting(true)
    setActionError(null)
    try {
      const accepted = await submit()
      navigate(`/tasks?task=${encodeURIComponent(accepted.taskId)}`)
    } catch (err) {
      setActionError(err instanceof Error ? err.message : String(err))
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <section className="page">
      <div className="page-head">
        <h1>{name}</h1>
        <div>
          <Link to="/packages" style={{ marginRight: 12 }}>
            ← Back
          </Link>
          <button
            onClick={() => void runTask(() => api.submitUpdate({ targets: [name] }))}
            disabled={submitting}
            style={{ marginRight: 8 }}
          >
            Update
          </button>
          <button
            onClick={() => {
              setPurge(false)
              setConfirming(true)
            }}
            disabled={submitting}
            style={{ marginRight: 8 }}
          >
            Uninstall
          </button>
          <button onClick={reload} disabled={loading}>
            Refresh
          </button>
        </div>
      </div>
      {actionError && <div className="alert">{actionError}</div>}
      {confirming && (
        <Dialog
          title={`Uninstall ${name}`}
          onClose={() => {
            setConfirming(false)
          }}
        >
          <p>
            eget stops managing this package, removes the installed binary and forgets the stored version.
          </p>
          <label className="check">
            <input
              type="checkbox"
              checked={purge}
              onChange={(event) => {
                setPurge(event.target.checked)
              }}
            />
            Also remove the package definition from config
          </label>
          <p className="muted">
            Without this the package stays in the config file, so it can be installed again with one command.
          </p>
          <div className="dialog-actions">
            <button
              onClick={() => {
                setConfirming(false)
              }}
              disabled={submitting}
            >
              Cancel
            </button>
            <button
              className="primary"
              disabled={submitting}
              onClick={() => {
                setConfirming(false)
                void runTask(() => api.submitUninstall({ target: name, purge }))
              }}
            >
              Uninstall
            </button>
          </div>
        </Dialog>
      )}
      <StateBlock loading={loading} error={error}>
        {data && (
          <>
            {data.desc && <div className="notice">{data.desc}</div>}
            <div className="panel">
              <h2>Package</h2>
              <dl className="detail-grid">
                <dt>Repo</dt>
                <dd className="mono">{data.repo || '-'}</dd>
                <dt>Source</dt>
                <dd>{data.source}</dd>
                <dt>Configured</dt>
                <dd>{data.configured ? 'yes' : 'no'}</dd>
                <dt>Installed</dt>
                <dd>{data.installed ? 'yes' : 'no'}</dd>
                <dt>Version</dt>
                <dd className="mono">{data.version || '-'}</dd>
                <dt>Tag</dt>
                <dd className="mono">{data.tag || '-'}</dd>
                <dt>Asset</dt>
                <dd className="mono">{data.asset || '-'}</dd>
                <dt>Install mode</dt>
                <dd>{data.installMode || '-'}</dd>
                <dt>Install target</dt>
                <dd className="mono">{data.installTarget || '-'}</dd>
                <dt>Config target</dt>
                <dd className="mono">{data.configTarget || '-'}</dd>
                <dt>Installed at</dt>
                <dd>{formatTime(data.installedAt)}</dd>
                <dt>Updated at</dt>
                <dd>{formatTime(data.updatedAt)}</dd>
                <dt>Ignore update</dt>
                <dd>{data.ignoreUpdate ? 'yes' : 'no'}</dd>
              </dl>
            </div>

            {data.homepage && (
              <div className="panel">
                <h2>Links</h2>
                <a href={data.homepage} target="_blank" rel="noreferrer noopener">
                  {data.homepage}
                </a>
              </div>
            )}

            {data.extractedFiles && data.extractedFiles.length > 0 && (
              <div className="panel">
                <h2>Extracted files</h2>
                <ul>
                  {data.extractedFiles.map((file) => (
                    <li key={file} className="mono">
                      {file}
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
