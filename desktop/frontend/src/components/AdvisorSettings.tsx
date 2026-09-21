import { useEffect, useState } from 'react'
import type { AdvisorSettingsView, Backend } from '../api'

interface AdvisorSettingsProps {
  backend: Backend
}

export default function AdvisorSettings({ backend }: AdvisorSettingsProps) {
  const [view, setView] = useState<AdvisorSettingsView | null>(null)
  const [selectedMode, setSelectedMode] = useState('local')
  const [keyValue, setKeyValue] = useState('')
  const [status, setStatus] = useState('')
  const [error, setError] = useState<string | null>(null)
  const [busy, setBusy] = useState(false)

  useEffect(() => {
    let cancelled = false
    void backend.getAdvisorSettings().then((next) => {
      if (cancelled) return
      setView(next)
      setSelectedMode(next.mode)
    }).catch((reason) => {
      if (!cancelled) setError(reason instanceof Error ? reason.message : String(reason))
    })
    return () => {
      cancelled = true
      setKeyValue('')
    }
  }, [backend])

  const run = async (action: () => Promise<AdvisorSettingsView | void>) => {
    setBusy(true)
    setError(null)
    try {
      const next = await action()
      if (next) {
        setView(next)
        setSelectedMode(next.mode)
      }
    } catch (reason) {
      setError(reason instanceof Error ? reason.message : String(reason))
    } finally {
      setBusy(false)
      setKeyValue('')
    }
  }

  return (
    <section className="advisor-page">
      <header className="page-heading">
        <div>
          <p className="eyebrow">Optional cloud recommendations</p>
          <h1>Advisor</h1>
        </div>
      </header>

      <article className="advisor-panel">
        <h2>Recommendation provider</h2>
        <p className="dialog-description">Local BM25F stays the default. TypeSafe is explicit opt-in with your key and your billing. Skill Manager sends the task brief plus names and bounded descriptions of every toggleable skill for the selected host. It does not send transcripts, project files, or full skill bodies.</p>
        <p className="dialog-description">Pinned model: {view?.model ?? 'jev-1.13.0'}.</p>
        <div className="advisor-choice-row">
          <button type="button" aria-label="Local" className={selectedMode === 'local' ? 'advisor-choice active' : 'advisor-choice'} onClick={() => { setSelectedMode('local'); setKeyValue('') }} disabled={busy}>
            <strong>Local</strong>
            <small>Offline ranked search only</small>
          </button>
          <button type="button" aria-label="TypeSafe" className={selectedMode === 'typesafe' ? 'advisor-choice active' : 'advisor-choice'} onClick={() => setSelectedMode('typesafe')} disabled={busy}>
            <strong>TypeSafe</strong>
            <small>Your key, your billing</small>
          </button>
        </div>
        <button type="button" className="primary-button" disabled={busy || !view} onClick={() => void run(() => backend.saveAdvisorProvider(selectedMode))}>Save</button>
      </article>

      {selectedMode === 'typesafe' && (
        <article className="advisor-panel">
          <h2>API key</h2>
          <label className="dialog-field">
            TypeSafe API key
            <input type="password" autoComplete="off" value={keyValue} onChange={(event) => setKeyValue(event.target.value)} disabled={busy} />
          </label>
          <div className="advisor-key-actions">
            <button type="button" className="primary-button" disabled={busy || keyValue.trim() === ''} onClick={() => void run(() => backend.setAdvisorKey(keyValue))}>Save key</button>
            <button type="button" disabled={busy} onClick={() => setKeyValue('')}>Cancel</button>
            <button type="button" disabled={busy} onClick={() => void (async () => {
              setBusy(true)
              setError(null)
              try {
                const check = await backend.checkAdvisorConnection()
                setStatus(check.ok ? `Connected (${check.keySource})` : `Check failed: ${check.reason ?? 'unknown'}`)
              } catch (reason) {
                setError(reason instanceof Error ? reason.message : String(reason))
              } finally {
                setBusy(false)
              }
            })()}>Check connection</button>
            <button type="button" disabled={busy} onClick={() => void run(() => backend.removeAdvisorKey())}>Remove key</button>
          </div>
          <p className="advisor-status">{status || `Stored key: ${view?.storedKey ?? 'absent'}. Environment key: ${view?.environmentKey ? 'present' : 'absent'}. Store: ${view?.credentialStore ?? 'unknown'}.`}</p>
        </article>
      )}
      {error && <p className="advisor-error" role="alert">{error}</p>}
    </section>
  )
}
