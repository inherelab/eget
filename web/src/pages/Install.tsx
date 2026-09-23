import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { api } from '../api/client'
import type { InstallCandidatesResponse } from '../api/types'

export default function Install() {
  const navigate = useNavigate()
  const [target, setTarget] = useState('')
  const [version, setVersion] = useState('')
  const [asset, setAsset] = useState('')
  const [output, setOutput] = useState('')
  const [file, setFile] = useState('')
  const [extractAll, setExtractAll] = useState(false)
  const [downloadOnly, setDownloadOnly] = useState(false)
  const [addToConfig, setAddToConfig] = useState(false)
  const [silent, setSilent] = useState(false)
  const [candidates, setCandidates] = useState<InstallCandidatesResponse | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [busy, setBusy] = useState(false)

  const checkAssets = async () => {
    const trimmed = target.trim()
    if (!trimmed) {
      setError('Enter a target such as owner/repo')
      return
    }
    setBusy(true)
    setError(null)
    setCandidates(null)
    try {
      setCandidates(await api.installCandidates(trimmed))
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err))
    } finally {
      setBusy(false)
    }
  }

  const submit = async () => {
    const trimmed = target.trim()
    if (!trimmed) {
      setError('Enter a target such as owner/repo')
      return
    }
    setBusy(true)
    setError(null)
    try {
      const accepted = await api.submitInstall({
        target: trimmed,
        version: version.trim(),
        asset: asset.trim(),
        output: output.trim(),
        file: file.trim(),
        extractAll,
        downloadOnly,
        addToConfig,
        silent,
      })
      navigate(`/tasks?task=${encodeURIComponent(accepted.taskId)}`)
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err))
    } finally {
      setBusy(false)
    }
  }

  return (
    <section className="page">
      <div className="page-head">
        <h1>Install</h1>
      </div>

      <div className="notice">
        Installs run as tasks with live logs. The console downloads and extracts only: it never launches a
        GUI installer and never executes a downloaded binary.
      </div>

      {error && <div className="alert">{error}</div>}

      <div className="panel">
        <h2>Target</h2>
        <div className="toolbar">
          <input
            type="text"
            value={target}
            onChange={(event) => setTarget(event.target.value)}
            placeholder="owner/repo, sourceforge:project/file, or a direct URL"
            style={{ minWidth: 360 }}
          />
          <button onClick={() => void checkAssets()} disabled={busy}>
            Check assets
          </button>
        </div>

        {candidates && candidates.candidates.length === 0 && (
          <div className="notice">No ambiguity for this target: the best matching asset is used.</div>
        )}

        {candidates && candidates.candidates.length > 0 && (
          <table>
            <thead>
              <tr>
                <th>Candidate asset</th>
                <th />
              </tr>
            </thead>
            <tbody>
              {candidates.candidates.map((candidate) => (
                <tr key={candidate}>
                  <td className="mono">{candidate}</td>
                  <td>
                    <button onClick={() => setAsset(candidate)}>Use this</button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </div>

      <div className="panel">
        <h2>Options</h2>
        <div className="toolbar">
          <input
            type="text"
            value={asset}
            onChange={(event) => setAsset(event.target.value)}
            placeholder="asset (exact name or keyword)"
          />
          <input
            type="text"
            value={version}
            onChange={(event) => setVersion(event.target.value)}
            placeholder="version / tag"
          />
          <input
            type="text"
            value={file}
            onChange={(event) => setFile(event.target.value)}
            placeholder="extract file inside the archive"
          />
          <input
            type="text"
            value={output}
            onChange={(event) => setOutput(event.target.value)}
            placeholder="output path"
          />
        </div>
        <div className="toolbar">
          <label className="check">
            <input type="checkbox" checked={extractAll} onChange={(event) => setExtractAll(event.target.checked)} />
            extract all
          </label>
          <label className="check">
            <input
              type="checkbox"
              checked={downloadOnly}
              onChange={(event) => setDownloadOnly(event.target.checked)}
            />
            download only
          </label>
          <label className="check">
            <input
              type="checkbox"
              checked={addToConfig}
              onChange={(event) => setAddToConfig(event.target.checked)}
            />
            add to config
          </label>
          <label className="check">
            <input type="checkbox" checked={silent} onChange={(event) => setSilent(event.target.checked)} />
            silent installer (MSI /qn)
          </label>
          <button className="primary" onClick={() => void submit()} disabled={busy}>
            Install
          </button>
        </div>
        <div className="notice">
          When several assets match and you have not picked one, the task fails and lists the candidates
          instead of guessing. Without "silent installer" a GUI installer is only downloaded, never launched.
        </div>
      </div>
    </section>
  )
}
