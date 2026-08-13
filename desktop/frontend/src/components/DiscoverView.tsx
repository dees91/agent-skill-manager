import { useEffect, useMemo, useRef, useState } from 'react'
import type { ReactNode } from 'react'
import {
  ArrowRight,
  Check,
  CircleAlert,
  CloudOff,
  Download,
  ExternalLink,
  FlaskConical,
  LoaderCircle,
  RefreshCw,
  Search,
  ShieldQuestion,
  TrendingUp,
  X,
} from 'lucide-react'
import { BrowserOpenURL } from '../../wailsjs/runtime/runtime'
import type { Backend, DiscoverDetail, DiscoverPage, DiscoverSkill, SourceMutationResult, SourceProgress } from '../api'

interface DiscoverViewProps {
  backend: Backend
  pendingCount: number
  busy: boolean
  progress: SourceProgress | null
  includeReadOnly: boolean
  onBusy: (busy: boolean) => void
  onResult: (result: SourceMutationResult) => void
  onAnnounce: (message: string) => void
  onViewInstalled: (skillName: string) => void
}

type CatalogView = 'all-time' | 'trending' | 'hot'

export default function DiscoverView(props: DiscoverViewProps) {
  const [view, setView] = useState<CatalogView>('all-time')
  const [query, setQuery] = useState('')
  const [page, setPage] = useState<DiscoverPage | null>(null)
  const [loading, setLoading] = useState(true)
  const [loadingMore, setLoadingMore] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [selected, setSelected] = useState<DiscoverSkill | null>(null)
  const [detail, setDetail] = useState<DiscoverDetail | null>(null)
  const [detailLoading, setDetailLoading] = useState(false)
  const [installOpen, setInstallOpen] = useState(false)
  const [installError, setInstallError] = useState<string | null>(null)
  const [installedName, setInstalledName] = useState<string | null>(null)
  const [searchRevision, setSearchRevision] = useState(0)
  const requestSequence = useRef(0)
  const detailSequence = useRef(0)

  const loadLeaderboard = async (nextView: CatalogView, force = false) => {
    const sequence = ++requestSequence.current
    setLoading(true)
    setError(null)
    try {
      const loaded = await props.backend.getDiscoverPage(nextView, 0, force)
      if (sequence !== requestSequence.current) return
      setPage(loaded)
    } catch (reason) {
      if (sequence === requestSequence.current) setError(errorMessage(reason))
    } finally {
      if (sequence === requestSequence.current) setLoading(false)
    }
  }

  useEffect(() => {
    detailSequence.current++
    setSelected(null)
    setDetail(null)
    setInstallOpen(false)
    setInstalledName(null)
    setLoadingMore(false)
    const normalized = query.trim()
    if (normalized.length === 0) {
      void loadLeaderboard(view)
      return
    }
    if ([...normalized].length < 2) {
      requestSequence.current++
      setLoading(false)
      setError(null)
      setPage(null)
      return
    }
    const sequence = ++requestSequence.current
    const timeout = window.setTimeout(async () => {
      setLoading(true)
      setError(null)
      try {
        const loaded = await props.backend.searchDiscover(normalized)
        if (sequence === requestSequence.current) setPage(loaded)
      } catch (reason) {
        if (sequence === requestSequence.current) setError(errorMessage(reason))
      } finally {
        if (sequence === requestSequence.current) setLoading(false)
      }
    }, 300)
    return () => window.clearTimeout(timeout)
  }, [props.backend, query, searchRevision, view])

  const selectSkill = async (skill: DiscoverSkill) => {
    const sequence = ++detailSequence.current
    setSelected(skill)
    setDetail(null)
    setInstalledName(null)
    setDetailLoading(true)
    try {
      const loaded = await props.backend.getDiscoverSkill(skill.id, false)
      if (sequence === detailSequence.current) setDetail(loaded)
    } catch (reason) {
      if (sequence === detailSequence.current) props.onAnnounce(errorMessage(reason))
    } finally {
      if (sequence === detailSequence.current) setDetailLoading(false)
    }
  }

  const loadMore = async () => {
    if (!page || query.trim() || !page.hasMore || loadingMore) return
    const sequence = ++requestSequence.current
    setLoadingMore(true)
    try {
      const next = await props.backend.getDiscoverPage(view, page.page + 1, false)
      if (sequence === requestSequence.current) setPage({ ...next, skills: deduplicate([...page.skills, ...next.skills]) } as DiscoverPage)
    } catch (reason) {
      if (sequence === requestSequence.current) props.onAnnounce(errorMessage(reason))
    } finally {
      if (sequence === requestSequence.current) setLoadingMore(false)
    }
  }

  const refresh = () => {
    if (query.trim().length >= 2) {
      setSearchRevision((current) => current + 1)
    } else {
      void loadLeaderboard(view, true)
    }
  }

  const acceptInstall = (result: SourceMutationResult) => {
    props.onResult(result)
    if (result.failure) {
      setInstallError(result.failure.message)
    } else if (selected) {
      setInstalledName(selected.skillId)
      setInstallOpen(false)
      setInstallError(null)
      void loadLeaderboard(view, true)
    }
  }

  const skillRows = page?.skills ?? []
  const modeLabel = query.trim().length >= 2 ? `Search results for “${query.trim()}”` : labelForView(view)

  return (
    <section className="page discover-page" aria-labelledby="discover-title">
      <div className="page-heading compact">
        <div><p className="eyebrow"><FlaskConical size={13} /> Experimental catalog</p><h1 id="discover-title">Discover</h1><p>Find individual skills on skills.sh and install them with Skill Manager ownership.</p></div>
        <button className="secondary-button" onClick={refresh} disabled={loading || props.busy}><RefreshCw size={15} className={loading ? 'spin' : ''} /> Refresh catalog</button>
      </div>

      <div className="discover-search-panel">
        <label className="search-field discover-search" htmlFor="discover-search"><Search size={17} /><input id="discover-search" aria-label="Search the skills.sh catalog" maxLength={200} value={query} onChange={(event) => setQuery(event.target.value)} placeholder="Search skills.sh…" /></label>
        <ConnectionStatus page={page} />
      </div>

      {!query.trim() && <div className="discover-tabs" role="tablist" aria-label="Catalog ranking">
        {(['all-time', 'trending', 'hot'] as CatalogView[]).map((item) => <button key={item} role="tab" aria-selected={view === item} className={view === item ? 'active' : ''} onClick={() => setView(item)}>{labelForView(item)}</button>)}
      </div>}

      <article className="discover-table-panel">
        <div className="table-toolbar"><div><strong>{modeLabel}</strong>{page && <span> · {page.total.toLocaleString()} skills</span>}</div><span className="experimental-note"><FlaskConical size={13} /> Unversioned catalog API</span></div>
        {query.trim().length === 1 && <CatalogMessage icon={<Search />} title="Keep typing" message="Enter at least 2 characters to search skills.sh." />}
        {loading && <CatalogMessage icon={<LoaderCircle className="spin" />} title="Loading catalog" message="Contacting skills.sh…" />}
        {error && !loading && <CatalogMessage icon={<CircleAlert />} title="Could not load the catalog" message={error} action={<button className="secondary-button" onClick={refresh}>Try again</button>} />}
        {!loading && !error && query.trim().length !== 1 && skillRows.length === 0 && <CatalogMessage icon={<Search />} title="No matching skills" message={page?.offline ? 'The offline cache has no matching entries.' : 'Try another name or source.'} />}
        {!loading && skillRows.length > 0 && <div className="table-scroll"><table className="discover-table">
          <thead><tr><th>#</th><th>Skill</th><th>Claude</th><th>Codex</th><th>Activity</th><th>Installs</th></tr></thead>
          <tbody>{skillRows.map((skill, index) => <tr key={skill.id} tabIndex={0} aria-label={`Open details for ${skill.name}`} onClick={() => void selectSkill(skill)} onKeyDown={(event) => { if (event.key === 'Enter' || event.key === ' ') { event.preventDefault(); void selectSkill(skill) } }} className={selected?.id === skill.id ? 'selected' : ''}>
            <td className="discover-rank">{query.trim() ? '—' : index + 1}</td>
            <td><div className="discover-name"><strong>{skill.name}</strong><small>{skill.source}</small>{skill.sourceType !== 'github' && <span className="source-type-badge">well-known</span>}</div></td>
            <td><DiscoverState state={skill.claude.status} /></td><td><DiscoverState state={skill.codex.status} /></td>
            <td><Activity skill={skill} view={view} /></td><td className="install-count">{formatCompact(skill.installs)}</td>
          </tr>)}</tbody>
        </table></div>}
        {!query.trim() && page?.hasMore && <div className="load-more-row"><button className="secondary-button" onClick={() => void loadMore()} disabled={loadingMore}>{loadingMore ? <LoaderCircle size={15} className="spin" /> : <ArrowRight size={15} />} Load more</button></div>}
      </article>

      {selected && <DiscoverDrawer skill={selected} detail={detail} loading={detailLoading} busy={props.busy} offline={Boolean(page?.offline || detail?.offline)} installedName={installedName} onClose={() => { detailSequence.current++; setSelected(null); setDetail(null) }} onInstall={() => { setInstallError(null); setInstallOpen(true) }} onViewInstalled={props.onViewInstalled} />}
      {installOpen && selected && <InstallSkillDialog skill={selected} busy={props.busy} pendingCount={props.pendingCount} progress={props.progress} error={installError} backend={props.backend} includeReadOnly={props.includeReadOnly} onBusy={props.onBusy} onResult={acceptInstall} onError={setInstallError} onClose={() => { setInstallOpen(false); setInstallError(null) }} />}
    </section>
  )
}

function DiscoverDrawer({ skill, detail, loading, busy, offline, installedName, onClose, onInstall, onViewInstalled }: { skill: DiscoverSkill; detail: DiscoverDetail | null; loading: boolean; busy: boolean; offline: boolean; installedName: string | null; onClose: () => void; onInstall: () => void; onViewInstalled: (name: string) => void }) {
  const available = [skill.claude, skill.codex].some((state) => state.status === 'available')
  const justInstalled = installedName === skill.skillId
  const installed = [skill.claude, skill.codex].some((state) => state.status.startsWith('installed-')) || justInstalled
  return <aside className="details-drawer discover-drawer" aria-label={`Catalog details for ${skill.name}`}>
    <div className="drawer-header"><div><p className="eyebrow">skills.sh detail</p><h2>{skill.name}</h2></div><button className="icon-button" onClick={onClose} aria-label="Close catalog details"><X size={17} /></button></div>
    <p className="details-description">{loading ? 'Loading description…' : detail?.description || 'No description is available from the catalog snapshot.'}</p>
    <dl className="details-summary"><div><dt>Source</dt><dd>{skill.source}</dd></div><div><dt>Installs</dt><dd>{skill.installs.toLocaleString()}</dd></div></dl>
    <div className="discover-agent-details"><AgentDetail label="Claude Code" state={skill.claude.status} message={skill.claude.message} /><AgentDetail label="Codex" state={skill.codex.status} message={skill.codex.message} /></div>
    <div className="audit-external"><ShieldQuestion size={16} /><div><strong>Security audits: External only</strong><p>Anonymous API access does not expose verified audit results. Review them on skills.sh.</p></div></div>
    {skill.sourceType !== 'github' && <div className="catalog-warning"><CircleAlert size={15} /><span>Well-known sources are visible but will be supported in the next installation scope.</span></div>}
    {offline && <div className="catalog-warning"><CloudOff size={15} /><span>Catalog data is offline. New installation is disabled until skills.sh responds.</span></div>}
    <div className="drawer-actions">
      <button className="secondary-button" onClick={() => BrowserOpenURL(skill.url)}><ExternalLink size={14} /> View on skills.sh</button>
      {installed && <button className="secondary-button" onClick={() => onViewInstalled(skill.skillId)}><Check size={14} /> View in Skills</button>}
      <button className="primary-button" onClick={onInstall} disabled={busy || offline || !skill.installable || !available || justInstalled}><Download size={14} /> {available && !justInstalled ? 'Install' : 'Installed'}</button>
    </div>
  </aside>
}

function InstallSkillDialog({ skill, busy, pendingCount, progress, error, backend, includeReadOnly, onBusy, onResult, onError, onClose }: { skill: DiscoverSkill; busy: boolean; pendingCount: number; progress: SourceProgress | null; error: string | null; backend: Backend; includeReadOnly: boolean; onBusy: (busy: boolean) => void; onResult: (result: SourceMutationResult) => void; onError: (error: string | null) => void; onClose: () => void }) {
  const available = useMemo(() => [skill.claude, skill.codex].filter((state) => state.status === 'available').map((state) => state.tool), [skill])
  const [tools, setTools] = useState(() => new Set(available))
  const dialogRef = useRef<HTMLDivElement>(null)

  useEffect(() => {
    const previous = document.activeElement as HTMLElement | null
    dialogRef.current?.querySelector<HTMLElement>('button, input')?.focus()
    const key = (event: KeyboardEvent) => {
      if (event.key === 'Escape' && !busy) onClose()
      if (event.key !== 'Tab' || !dialogRef.current) return
      const controls = [...dialogRef.current.querySelectorAll<HTMLElement>('button:not(:disabled), input:not(:disabled)')]
      if (!controls.length) return
      const first = controls[0], last = controls[controls.length - 1]
      if (event.shiftKey && document.activeElement === first) { event.preventDefault(); last.focus() }
      else if (!event.shiftKey && document.activeElement === last) { event.preventDefault(); first.focus() }
    }
    window.addEventListener('keydown', key)
    return () => { window.removeEventListener('keydown', key); previous?.focus() }
  }, [busy, onClose])

  const install = async () => {
    onBusy(true)
    onError(null)
    try { onResult(await backend.installDiscoverSkill(skill.id, [...tools], includeReadOnly)) }
    catch (reason) { onError(errorMessage(reason)) }
    finally { onBusy(false) }
  }

  return <div className="modal-backdrop"><div className="modal discover-install-modal" role="dialog" aria-modal="true" aria-labelledby="discover-install-title" ref={dialogRef}>
    <div className="modal-header"><div><p className="eyebrow">Install catalog skill</p><h2 id="discover-install-title">{skill.name}</h2></div><button className="icon-button" onClick={onClose} disabled={busy} aria-label="Close install confirmation"><X size={17} /></button></div>
    <div className="modal-content">
      <dl className="install-source-summary"><div><dt>Repository</dt><dd>{skill.source}</dd></div><div><dt>Catalog ID</dt><dd>{skill.id}</dd></div></dl>
      <div className="external-safety-warning"><ShieldQuestion size={20} /><div><strong>Review third-party skills before using them</strong><p>This repository can contain external instructions and supporting code that affect agent behavior. Skill Manager will clone the repository and link only this selected skill.</p></div></div>
      <fieldset className="agent-selection"><legend>Install for</legend>{[skill.claude, skill.codex].map((state) => <label key={state.tool} className={state.status !== 'available' ? 'unavailable' : ''}><input type="checkbox" checked={tools.has(state.tool)} disabled={busy || state.status !== 'available'} onChange={() => setTools((current) => { const next = new Set(current); next.has(state.tool) ? next.delete(state.tool) : next.add(state.tool); return next })} /><span><strong>{state.tool === 'claude' ? 'Claude Code' : 'Codex'}</strong><small>{stateLabel(state.status)}{state.message ? ` · ${state.message}` : ''}</small></span></label>)}</fieldset>
      {pendingCount > 0 && <div className="catalog-warning"><CircleAlert size={15} /><span>Apply or clear {pendingCount} pending visibility change{pendingCount === 1 ? '' : 's'} before installing.</span></div>}
      {error && <div className="catalog-warning" role="alert"><CircleAlert size={15} /><span>{error}</span></div>}
      {busy && progress && <div className="operation-progress" role="status"><LoaderCircle size={16} className="spin" /><div><strong>{progress.message}</strong><small>{progress.group || 'skills.sh catalog'}</small></div></div>}
    </div>
    <div className="modal-actions"><button className="secondary-button" onClick={onClose} disabled={busy}>Cancel</button><button className="primary-button" onClick={() => void install()} disabled={busy || pendingCount > 0 || tools.size === 0}>{busy ? <LoaderCircle size={15} className="spin" /> : <Download size={15} />} Install for {tools.size} agent{tools.size === 1 ? '' : 's'}</button></div>
  </div></div>
}

function ConnectionStatus({ page }: { page: DiscoverPage | null }) {
  if (!page) return <span className="catalog-connection muted">Waiting for catalog</span>
  const status = page.offline ? 'Offline cache' : page.fromCache ? 'Recent cache' : 'Connected'
  return <span className={`catalog-connection ${page.offline ? 'offline' : page.fromCache ? 'cached' : 'connected'}`} title={page.warning || undefined}>{page.offline ? <CloudOff size={13} /> : <span className="status-dot" />}{status} · updated {formatTime(page.fetchedAt)}</span>
}

function DiscoverState({ state }: { state: string }) { return <span className={`discover-state discover-state-${state}`}>{stateLabel(state)}</span> }
function AgentDetail({ label, state, message }: { label: string; state: string; message?: string }) { return <div><span>{label}</span><DiscoverState state={state} />{message && <small>{message}</small>}</div> }

function Activity({ skill, view }: { skill: DiscoverSkill; view: CatalogView }) {
  const change = skill.change ?? 0
  if (view === 'hot') return <span className={change >= 0 ? 'activity-positive' : 'activity-negative'}>{change >= 0 ? '+' : ''}{change}</span>
  if (view === 'trending') return <TrendingUp size={16} className="activity-positive" />
  if (!skill.weeklyInstalls?.length) return <span className="muted">—</span>
  const weekly = skill.weeklyInstalls
  const maximum = Math.max(...weekly, 1)
  const points = weekly.map((value, index) => `${index * (72 / Math.max(1, weekly.length - 1))},${22 - value / maximum * 18}`).join(' ')
  return <svg className="activity-sparkline" viewBox="0 0 72 24" aria-label="8 week install activity"><polyline points={points} /></svg>
}

function CatalogMessage({ icon, title, message, action }: { icon: ReactNode; title: string; message: string; action?: ReactNode }) { return <div className="catalog-message">{icon}<strong>{title}</strong><p>{message}</p>{action}</div> }
function stateLabel(state: string) { return ({ available: 'Available', 'installed-on': 'Installed · On', 'installed-off': 'Installed · Off', conflict: 'Conflict' } as Record<string, string>)[state] || state }
function labelForView(view: CatalogView) { return view === 'all-time' ? 'All Time' : view === 'trending' ? 'Trending (24h)' : 'Hot' }
function formatCompact(value: number) { return Intl.NumberFormat(undefined, { notation: 'compact', maximumFractionDigits: 1 }).format(value) }
function formatTime(value: string) { const date = new Date(value); return Number.isNaN(date.getTime()) ? value : date.toLocaleString([], { month: 'short', day: 'numeric', hour: '2-digit', minute: '2-digit' }) }
function errorMessage(reason: unknown) { return reason instanceof Error ? reason.message : String(reason) }
function deduplicate(skills: DiscoverSkill[]) { return [...new Map(skills.map((skill) => [skill.id, skill])).values()] }
