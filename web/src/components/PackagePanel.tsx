import { useState } from 'react'
import { api, formatTime } from '../api/client'
import Dialog from './Dialog'
import StateBlock from './StateBlock'
import TaskStrip from './TaskStrip'
import { useAsync } from '../hooks/useAsync'

interface PackagePanelProps {
  name: string
  // onChanged reports a finished action that also changed the list behind this
  // panel, so a drawer can refresh and close itself.
  onChanged?: (change: 'updated' | 'uninstalled') => void
}

// PackagePanel is one package with its actions; the packages list shows it in a
// drawer and the /packages/:name route shows it as a page.
export default function PackagePanel({ name, onChanged }: PackagePanelProps) {
  const { data, error, loading, reload } = useAsync(() => api.packageDetail(name), [name])
  const [actionError, setActionError] = useState<string | null>(null)
  const [submitting, setSubmitting] = useState(false)
  const [confirming, setConfirming] = useState(false)
  const [purge, setPurge] = useState(false)
  const [task, setTask] = useState<{ id: string; kind: string }>({ id: '', kind: '' })

  const runTask = async (submit: () => Promise<{ taskId: string }>, kind: string) => {
    setSubmitting(true)
    setActionError(null)
    try {
      const accepted = await submit()
      setTask({ id: accepted.taskId, kind })
    } catch (err) {
      setActionError(err instanceof Error ? err.message : String(err))
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <>
      <div className="panel-actions">
        <button
          onClick={() => void runTask(() => api.submitUpdate({ targets: [name] }), 'update')}
          disabled={submitting}
        >
          Update
        </button>
        <button
          onClick={() => {
            setPurge(false)
            setConfirming(true)
          }}
          disabled={submitting}
        >
          Uninstall
        </button>
        <button onClick={reload} disabled={loading}>
          Refresh
        </button>
      </div>

      {actionError && <div className="alert">{actionError}</div>}
      {task.id && (
        <TaskStrip
          taskId={task.id}
          onDismiss={() => {
            setTask({ id: '', kind: '' })
          }}
          onFinished={(status) => {
            if (status !== 'succeeded') {
              return
            }
            // An update leaves this panel describing an older version, so re-read
            // it; an uninstall removes the subject, so the panel is left alone and
            // the caller decides what happens to the list behind it.
            if (task.kind === 'update') {
              void reload()
              onChanged?.('updated')
              return
            }
            onChanged?.('uninstalled')
          }}
        />
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

      {confirming && (
        <Dialog
          title={`Uninstall ${name}`}
          onClose={() => {
            setConfirming(false)
          }}
        >
          <p>eget stops managing this package, removes the installed binary and forgets the stored version.</p>
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
                void runTask(() => api.submitUninstall({ target: name, purge }), 'uninstall')
              }}
            >
              Uninstall
            </button>
          </div>
        </Dialog>
      )}
    </>
  )
}
