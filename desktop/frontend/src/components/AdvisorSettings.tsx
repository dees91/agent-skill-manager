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

  const run = async (kind: 'provider' | 'saveKey' | 'removeKey', action: () => Promise<AdvisorSettingsView>) => {
    setBusy(true)
    setError(null)
    setStatus('')
    try {
      const next = await action()
      setView(next)
      // Saving a key never changes the saved provider, so keep an unsaved
      // TypeSafe selection; saving the provider and removing the key do.
      if (kind !== 'saveKey') setSelectedMode(next.mode)
    } catch (reason) {
      setError(reason instanceof Error ? reason.message : String(reason))
      if (kind === 'removeKey') {
        // A refused removal still resets the saved provider to local.
        try {
          const current = await backend.getAdvisorSettings()
          setView(current)
          setSelectedMode(current.mode)
        } catch {
          // Keep the reported error.
        }
      }
    } finally {
      setBusy(false)
      if (kind !== 'provider') setKeyValue('')
    }
  }

  // The key panel belongs to the TypeSafe choice. A key that exists while
  // Local is selected is still surfaced below the cards so it can be removed.
  const showKeyPanel = selectedMode === 'typesafe'
  const storedKeyPresent = view?.storedKey === 'present'
  const keyPresent = storedKeyPresent || Boolean(view?.environmentKey)
  const keyStatus = `Stored key: ${view?.storedKey ?? 'absent'}. Environment key: ${view?.environmentKey ? 'present' : 'absent'}. Store: ${view?.credentialStore ?? 'unknown'}.`

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
          <button type="button" aria-label="Local" aria-pressed={selectedMode === 'local'} className={selectedMode === 'local' ? 'advisor-choice active' : 'advisor-choice'} onClick={() => { setSelectedMode('local'); setKeyValue(''); setStatus('') }} disabled={busy}>
            <strong>Local</strong>
            <small>Offline ranked search only</small>
          </button>
          <button type="button" aria-label="TypeSafe" aria-pressed={selectedMode === 'typesafe'} className={selectedMode === 'typesafe' ? 'advisor-choice active' : 'advisor-choice'} onClick={() => { setSelectedMode('typesafe'); setStatus('') }} disabled={busy}>
            <strong>TypeSafe</strong>
            <small>Your key, your billing</small>
          </button>
        </div>
        <button type="button" className="primary-button" disabled={busy || !view} onClick={() => void run('provider', () => backend.saveAdvisorProvider(selectedMode))}>Save</button>
        {!showKeyPanel && keyPresent && (
          <div className="advisor-key-note">
            <p className="advisor-status">{keyStatus}</p>
            {storedKeyPresent && <button type="button" className="danger-button" disabled={busy} onClick={() => void run('removeKey', () => backend.removeAdvisorKey())}>Remove key</button>}
          </div>
        )}
      </article>

      {showKeyPanel && (
        <article className="advisor-panel">
          <h2>API key</h2>
          <label className="dialog-field">
            TypeSafe API key
            <input type="password" autoComplete="off" value={keyValue} onChange={(event) => setKeyValue(event.target.value)} disabled={busy} />
          </label>
          <div className="advisor-key-actions">
            <button type="button" className="primary-button" disabled={busy || keyValue.trim() === ''} onClick={() => void run('saveKey', () => backend.setAdvisorKey(keyValue))}>Save key</button>
            <button type="button" className="secondary-button" disabled={busy} onClick={() => setKeyValue('')}>Cancel</button>
            <button type="button" className="secondary-button" disabled={busy} onClick={() => void (async () => {
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
            <button type="button" className="danger-button" disabled={busy} onClick={() => void run('removeKey', () => backend.removeAdvisorKey())}>Remove key</button>
          </div>
          <p className="advisor-status">{status || keyStatus}</p>
        </article>
      )}
      {error && <p className="advisor-error" role="alert">{error}</p>}
    </section>
  )
}
