import { useCallback, useEffect, useRef, useState } from 'react'
import {
  CircleAlert,
  Gauge,
  GitFork,
  Layers3,
  ListChecks,
  RefreshCw,
  Search,
  SlidersHorizontal,
  Sparkles,
} from 'lucide-react'
import Dashboard from './components/Dashboard'
import PendingBar from './components/PendingBar'
import SkillsView from './components/SkillsView'
import SkillSetsView, { type SkillSetEditorRequest } from './components/SkillSetsView'
import SourcesView from './components/SourcesView'
import type { ActionResult, Backend, FavoriteMutationResult, SkillRow, SkillSetMutationResult, Snapshot, SourceMutationResult, SourceProgress } from './api'
import { MANAGED_TOOLS, favoriteEligible, joinList, projectPending, toolDisplayName, wailsBackend } from './api'
import { EventsOn } from '../wailsjs/runtime/runtime'

type View = 'dashboard' | 'skills' | 'skillsets' | 'sources'

interface AppProps {
  backend?: Backend
}

export default function App({ backend = wailsBackend }: AppProps) {
  const [view, setView] = useState<View>('dashboard')
  const [snapshot, setSnapshot] = useState<Snapshot | null>(null)
  const [includeReadOnly, setIncludeReadOnly] = useState(false)
  const [expandedSkillGroups, setExpandedSkillGroups] = useState<string[]>([])
  const [selectedSkill, setSelectedSkill] = useState<string | null>(null)
  const [reviewOpen, setReviewOpen] = useState(false)
  const [skillSetEditorRequest, setSkillSetEditorRequest] = useState<SkillSetEditorRequest | null>(null)
  const [loading, setLoading] = useState(true)
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [toast, setToast] = useState<string | null>(null)
  const [sourceProgress, setSourceProgress] = useState<SourceProgress | null>(null)
  const announcementRef = useRef<HTMLDivElement>(null)
  const skillSetRequestID = useRef(0)

  const announce = useCallback((message: string) => {
    setToast(message)
    if (announcementRef.current) announcementRef.current.textContent = message
  }, [])

  const loadSnapshot = useCallback(async (showReadOnly: boolean, quiet = false) => {
    if (!quiet) setLoading(true)
    setError(null)
    try {
      const next = await backend.getSnapshot(showReadOnly)
      setSnapshot(next)
      if (quiet) announce('Filesystem scan refreshed.')
    } catch (reason) {
      setError(errorMessage(reason))
    } finally {
      setLoading(false)
    }
  }, [announce, backend])

  useEffect(() => {
    void loadSnapshot(false)
  }, [loadSnapshot])

  useEffect(() => {
    if (!(window as Window & { runtime?: unknown }).runtime) return
    return EventsOn('source-operation-progress', (progress: SourceProgress) => setSourceProgress(progress))
  }, [])

  useEffect(() => {
    if (!toast) return
    const timeout = window.setTimeout(() => setToast(null), 3200)
    return () => window.clearTimeout(timeout)
  }, [toast])

  const runAction = useCallback(async (action: () => Promise<ActionResult>) => {
    if (!snapshot || busy) return
    setBusy(true)
    try {
      const result = await action()
      setSnapshot((current) => current ? projectPending(current, result.pending, result.contextBudgets, result.skillSets, result.skillSetsWarning) : current)
      announce(actionMessage(result))
    } catch (reason) {
      announce(errorMessage(reason))
    } finally {
      setBusy(false)
    }
  }, [announce, busy, snapshot])

  const acceptActionResult = useCallback((result: ActionResult) => {
    setSnapshot((current) => current ? projectPending(current, result.pending, result.contextBudgets, result.skillSets, result.skillSetsWarning) : current)
  }, [])

  const acceptSkillSetMutation = useCallback((result: SkillSetMutationResult) => {
    setSnapshot((current) => current ? { ...current, skillSets: result.skillSets, skillSetsWarning: result.warning } as Snapshot : current)
  }, [])

  const setSkillFavorite = useCallback(async (skillName: string, favorite: boolean) => {
    if (!snapshot || busy) return
    setBusy(true)
    try {
      const result: FavoriteMutationResult = await backend.setSkillFavorite(skillName, favorite)
      const favorites = new Set(result.favorites)
      setSnapshot((current) => current ? {
        ...current,
        favoritesWarning: result.warning,
        rows: current.rows.map((row) => ({ ...row, favorite: favoriteEligible(row) && favorites.has(row.name) } as SkillRow)),
      } as Snapshot : current)
      announce(result.message)
    } catch (reason) {
      announce(errorMessage(reason))
    } finally {
      setBusy(false)
    }
  }, [announce, backend, busy, snapshot])

  const openSkillSetEditor = useCallback((kind: SkillSetEditorRequest['kind'], skillNames: string[]) => {
    skillSetRequestID.current += 1
    setSkillSetEditorRequest({ id: skillSetRequestID.current, kind, skillNames })
    setView('skillsets')
  }, [])

  const applyPending = useCallback(async () => {
    if (!snapshot?.pending.length || busy) return
    setBusy(true)
    try {
      const result = await backend.applyPending(includeReadOnly)
      setSnapshot(result.snapshot)
      setReviewOpen(Boolean(result.failure))
      announce(result.failure ? `${result.message} ${result.failure.message}` : result.message)
    } catch (reason) {
      announce(errorMessage(reason))
    } finally {
      setBusy(false)
    }
  }, [announce, backend, busy, includeReadOnly, snapshot])

  const clearPending = useCallback(async () => {
    await runAction(() => backend.clearPending())
    setReviewOpen(false)
  }, [backend, runAction])

  const measureContextBudgets = useCallback(async () => {
	if (!snapshot || busy) return
	setBusy(true)
	try {
		setSnapshot(await backend.measureContextBudgets())
		announce('Provider diagnostics completed.')
	} catch (reason) {
		announce(errorMessage(reason))
	} finally {
		setBusy(false)
	}
  }, [announce, backend, busy, snapshot])

  const showReadOnly = useCallback(async (show: boolean) => {
    setIncludeReadOnly(show)
    await loadSnapshot(show)
  }, [loadSnapshot])

  const acceptSourceResult = useCallback((result: SourceMutationResult) => {
    setSnapshot(result.snapshot)
    setSourceProgress(null)
    announce(result.failure ? `${result.message} ${result.failure.message}` : result.message)
  }, [announce])

  const focusSearch = useCallback(() => {
    setView('skills')
    window.setTimeout(() => document.getElementById('skill-search')?.focus(), 0)
  }, [])

  const viewInstalledSkill = useCallback((skillName: string) => {
    setSelectedSkill(skillName)
    setView('skills')
  }, [])

  useEffect(() => {
    const handleKey = (event: KeyboardEvent) => {
      if (!(event.metaKey || event.ctrlKey)) {
        if (event.key === 'Escape') {
          setReviewOpen(false)
          setSelectedSkill(null)
        }
        return
      }
      if (event.key.toLowerCase() === 'f') {
        event.preventDefault()
        focusSearch()
      }
      if (event.key.toLowerCase() === 'r') {
        event.preventDefault()
        if (!busy) void loadSnapshot(includeReadOnly, true)
      }
      if (event.key === 'Enter') {
        event.preventDefault()
        void applyPending()
      }
    }
    window.addEventListener('keydown', handleKey)
    return () => window.removeEventListener('keydown', handleKey)
  }, [applyPending, busy, focusSearch, includeReadOnly, loadSnapshot])

  const selectedRow = snapshot?.rows.find((row) => row.name === selectedSkill) ?? null

  return (
    <div className="app-shell">
      <div className="titlebar" aria-hidden="true">
        <span className="titlebar-name">Skill Manager</span>
        <span className="titlebar-context">Local workspace</span>
      </div>

      <aside className="sidebar" aria-label="Primary navigation">
        <div className="brand">
          <span className="brand-mark"><SlidersHorizontal size={19} /></span>
          <span><strong>Skill Manager</strong><small>{MANAGED_TOOLS.map(toolDisplayName).join(' + ')}</small></span>
        </div>

        <button className="sidebar-search" onClick={focusSearch} aria-label="Search installed skills">
          <Search size={15} /><span>Search installed</span><kbd>⌘F</kbd>
        </button>

        <nav className="nav-list">
          <button aria-label="Dashboard" className={view === 'dashboard' ? 'nav-item active' : 'nav-item'} onClick={() => setView('dashboard')}>
            <Gauge size={17} /><span>Dashboard</span>
          </button>
          <button aria-label="Skills" className={view === 'skills' ? 'nav-item active' : 'nav-item'} onClick={() => setView('skills')}>
            <Layers3 size={17} /><span>Skills</span>
            {snapshot && <span className="nav-count">{snapshot.stats.managedSkills}</span>}
          </button>
          <button aria-label="Skill Sets" className={view === 'skillsets' ? 'nav-item active' : 'nav-item'} onClick={() => setView('skillsets')}>
            <ListChecks size={17} /><span>Skill Sets</span>
            {snapshot && <span className="nav-count">{snapshot.skillSets.length}</span>}
          </button>
          <button aria-label="Sources" className={view === 'sources' ? 'nav-item active' : 'nav-item'} onClick={() => setView('sources')}>
            <GitFork size={17} /><span>Sources</span>
            {snapshot && <span className="nav-count">{snapshot.managedSources.length}</span>}
          </button>
        </nav>

        <div className="sidebar-spacer" />
        <div className="local-note">
          <span className="status-dot" />
          <div><strong>Local state</strong><small>Git actions use network</small></div>
        </div>
        <div className="scan-meta">
          <span>Last scan</span>
          <time>{snapshot?.scannedAt ? formatScanTime(snapshot.scannedAt) : '—'}</time>
        </div>
      </aside>

      <main className="main-region">
        <header className="utility-bar">
          <div className="breadcrumbs"><span>Skill Manager</span><b>/</b><strong>{view === 'dashboard' ? 'Dashboard' : view === 'skills' ? 'Skills' : view === 'skillsets' ? 'Skill Sets' : 'Sources'}</strong></div>
          <div className="utility-actions">
            {snapshot && snapshot.stats.conflictCells > 0 && (
              <button className="conflict-chip" onClick={() => setView('skills')}>
                <CircleAlert size={14} /> {snapshot.stats.conflictCells} conflict{snapshot.stats.conflictCells === 1 ? '' : 's'}
              </button>
            )}
            <button className="icon-button" onClick={() => void loadSnapshot(includeReadOnly, true)} aria-label="Refresh filesystem scan" disabled={loading || busy}>
              <RefreshCw size={16} className={loading ? 'spin' : ''} />
            </button>
          </div>
        </header>

        <div className={`content-scroll ${snapshot?.pending.length ? 'has-pending' : ''}`}>
          {loading && !snapshot && <LoadingState />}
          {error && !snapshot && <ErrorState message={error} retry={() => void loadSnapshot(includeReadOnly)} />}
          {snapshot && view === 'dashboard' && (
            <Dashboard snapshot={snapshot} busy={busy} onBrowseSkills={() => setView('skills')} onMeasureContext={() => void measureContextBudgets()} />
          )}
          {snapshot && view === 'skills' && (
            <SkillsView
              snapshot={snapshot}
              selectedRow={selectedRow}
              busy={busy}
              includeReadOnly={includeReadOnly}
              expandedGroups={expandedSkillGroups}
              onExpandedGroupsChange={setExpandedSkillGroups}
              onIncludeReadOnly={showReadOnly}
              onSelect={setSelectedSkill}
              onToggleCell={(name, tool) => runAction(() => backend.toggleCell(name, tool))}
              onToggleSkillScope={(names, tools) => runAction(() => backend.toggleSkillScope(names, tools))}
              onToggleGroupScope={(group, tools) => runAction(() => backend.toggleGroupScope(group, tools))}
              onSetFavorite={(name, favorite) => void setSkillFavorite(name, favorite)}
              onAddToSkillSet={(name) => openSkillSetEditor('skill', [name])}
              onCloseDetails={() => setSelectedSkill(null)}
            />
          )}
          {snapshot && view === 'skillsets' && (
            <SkillSetsView
              snapshot={snapshot}
              busy={busy}
              backend={backend}
              editorRequest={skillSetEditorRequest}
              onEditorRequestHandled={() => setSkillSetEditorRequest(null)}
              onBusy={setBusy}
              onMutation={acceptSkillSetMutation}
              onAction={acceptActionResult}
              onAnnounce={announce}
            />
          )}
          {snapshot && view === 'sources' && (
            <SourcesView
              sources={snapshot.managedSources}
              pendingCount={snapshot.pending.length}
              busy={busy}
              progress={sourceProgress}
              backend={backend}
              includeReadOnly={includeReadOnly}
              onBusy={setBusy}
              onResult={acceptSourceResult}
              onAnnounce={announce}
            />
          )}
        </div>

        {snapshot && snapshot.pending.length > 0 && (
          <PendingBar
            pending={snapshot.pending}
            reviewOpen={reviewOpen}
            busy={busy}
            onToggleReview={() => setReviewOpen((open) => !open)}
            onCloseReview={() => setReviewOpen(false)}
            onUndo={(name, tool) => runAction(() => backend.undoCell(name, tool))}
            onClear={() => void clearPending()}
            onApply={() => void applyPending()}
            onSaveAsSkillSet={() => openSkillSetEditor('pending', [...new Set(snapshot.pending.map((change) => change.skillName))])}
          />
        )}
      </main>

      <div className="sr-only" role="status" aria-live="polite" ref={announcementRef} />
      {toast && <div className="toast"><Sparkles size={15} />{toast}</div>}
    </div>
  )
}

function LoadingState() {
  return (
    <div className="center-state" aria-label="Loading skills">
      <span className="loader" /><strong>Scanning local skills…</strong><small>Reading {joinList(MANAGED_TOOLS.map(toolDisplayName), 'and')} user directories</small>
    </div>
  )
}

function ErrorState({ message, retry }: { message: string; retry: () => void }) {
  return (
    <div className="center-state error-state">
      <CircleAlert size={28} /><strong>Could not scan skills</strong><small>{message}</small>
      <button className="secondary-button" onClick={retry}>Try again</button>
    </div>
  )
}

function actionMessage(result: ActionResult) {
  const skipped = result.counts.skippedReadOnly + result.counts.skippedMissing + result.counts.skippedConflict
  return skipped ? `${result.message} ${skipped} cell${skipped === 1 ? '' : 's'} skipped.` : result.message
}

function errorMessage(reason: unknown) {
  return reason instanceof Error ? reason.message : String(reason)
}

function formatScanTime(value: string) {
  const date = new Date(value)
  return Number.isNaN(date.getTime()) ? value : date.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })
}
