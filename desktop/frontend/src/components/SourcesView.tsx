import { useEffect, useMemo, useRef, useState } from 'react'
import {
  AlertTriangle,
  Check,
  Download,
  FolderOpen,
  GitBranch,
  HardDrive,
  LoaderCircle,
  PackagePlus,
  RefreshCw,
  Search,
  Trash2,
  X,
} from 'lucide-react'
import type {
  Backend,
  ExtendPreview,
  ExtendPreviewSource,
  InstallCellRequest,
  InstallDraft,
  InstallReview,
  ManagedSource,
  ManagedTool,
  SourceMutationResult,
  SourceProgress,
  UninstallPreview,
} from '../api'
import { MANAGED_TOOLS, toolDisplayName } from '../api'

interface SourcesViewProps {
  sources: ManagedSource[]
  pendingCount: number
  busy: boolean
  progress: SourceProgress | null
  backend: Backend
  includeReadOnly: boolean
  onBusy: (busy: boolean) => void
  onResult: (result: SourceMutationResult) => void
  onAnnounce: (message: string) => void
}

type InstallMode = 'git' | 'local'
type Dialog = 'install' | 'update-all' | 'update-one' | 'uninstall' | 'extend' | null
type ColumnSelectionState = 'ON' | 'OFF' | 'MIXED' | 'N/A'

export default function SourcesView(props: SourcesViewProps) {
  const { sources, pendingCount, busy, progress, backend, includeReadOnly, onBusy, onResult, onAnnounce } = props
  const [dialog, setDialog] = useState<Dialog>(null)
  const [selectedSource, setSelectedSource] = useState<ManagedSource | null>(null)
  const [uninstallPreview, setUninstallPreview] = useState<UninstallPreview | null>(null)
  const [dialogError, setDialogError] = useState<string | null>(null)
  const installButtonRef = useRef<HTMLButtonElement>(null)
  const returnFocusRef = useRef<HTMLElement | null>(null)

  const openDialog = (next: Dialog, source: ManagedSource | null = null) => {
    if (pendingCount > 0) {
      onAnnounce('Apply or clear pending skill changes before managing sources.')
      return
    }
    setSelectedSource(source)
    setUninstallPreview(null)
    setDialogError(null)
    returnFocusRef.current = document.activeElement instanceof HTMLElement ? document.activeElement : null
    setDialog(next)
  }

  const closeDialog = () => {
    if (busy) return
    setDialog(null)
    setSelectedSource(null)
    setUninstallPreview(null)
    setDialogError(null)
    window.setTimeout(() => (returnFocusRef.current ?? installButtonRef.current)?.focus(), 0)
  }

  const runMutation = async (action: () => Promise<SourceMutationResult>) => {
    if (busy) return
    onBusy(true)
    setDialogError(null)
    try {
      const result = await action()
      onResult(result)
      if (result.failure) setDialogError(result.failure.message)
      else closeDialog()
    } catch (reason) {
      setDialogError(errorMessage(reason))
    } finally {
      onBusy(false)
    }
  }

  const prepareUninstall = async (source: ManagedSource) => {
    openDialog('uninstall', source)
    onBusy(true)
    try {
      setUninstallPreview(await backend.previewUninstall(source.sourceId))
    } catch (reason) {
      setDialogError(errorMessage(reason))
    } finally {
      onBusy(false)
    }
  }

  return (
    <section className="page sources-page" aria-labelledby="sources-heading">
      <div className="page-heading compact">
        <div>
          <p className="eyebrow">Managed sources</p>
          <h1 id="sources-heading">Sources</h1>
          <p>Install and maintain repositories and link-in-place folders owned by Skill Manager.</p>
        </div>
        <div className="source-heading-actions">
          <button className="secondary-button" disabled={busy || pendingCount > 0 || !canExtendSources(sources)} onClick={() => openDialog('extend')}>
            <PackagePlus size={14} /> Extend to tool
          </button>
          <button className="secondary-button" disabled={busy || pendingCount > 0 || !sources.some((source) => source.canUpdate)} onClick={() => openDialog('update-all')}>
            <RefreshCw size={14} /> Update all
          </button>
          <button ref={installButtonRef} className="primary-button" disabled={busy || pendingCount > 0} onClick={() => openDialog('install')}>
            <PackagePlus size={14} /> Install source
          </button>
        </div>
      </div>

      {pendingCount > 0 && (
        <div className="source-blocked-note" role="status">
          <AlertTriangle size={15} /> Apply or clear {pendingCount} pending skill change{pendingCount === 1 ? '' : 's'} before managing sources.
        </div>
      )}

      <div className="panel sources-panel">
        <div className="panel-header">
          <div><h2>Installed sources</h2><small>{sources.length} managed source{sources.length === 1 ? '' : 's'}</small></div>
          <span className="panel-badge">{sources.reduce((total, source) => total + source.skillCount, 0)} skills</span>
        </div>
        {sources.length === 0 ? (
          <div className="source-empty">
            <PackagePlus size={28} /><strong>No managed sources</strong>
            <p>Install a Git repository or choose a local skill folder.</p>
            <button className="primary-button" disabled={busy || pendingCount > 0} onClick={() => openDialog('install')}>Install source</button>
          </div>
        ) : (
          <div className="source-table-scroll">
            <table className="source-table">
              <thead><tr><th>Source</th><th>Skills</th><th>Targets</th><th>Update mode</th><th>Actions</th></tr></thead>
              <tbody>{sources.map((source) => (
                <tr key={source.sourceId}>
                  <td><div className="source-identity"><span>{source.kind === 'git' ? <GitBranch size={15} /> : <HardDrive size={15} />}</span><div><strong>{source.group}</strong><code>{source.location}</code></div></div></td>
                  <td><strong className="source-count">{source.skillCount}</strong></td>
                  <td><div className="target-counts"><span>Claude {source.claudeCount}</span><span>Codex {source.codexCount}</span><span>Muse {source.museCount}</span></div></td>
                  <td><div className="source-update-mode"><span className={source.canUpdate ? 'status-dot git' : 'status-dot'} /><div><strong>{source.updateMode}</strong><small>{source.updateHint}</small>{source.commit && <code>Commit {shortCommit(source.commit)}</code>}</div></div></td>
                  <td><div className="source-row-actions">
                    {source.canUpdate && <button aria-label={`Update ${source.group}`} className="secondary-button compact-button" disabled={busy || pendingCount > 0} onClick={() => openDialog('update-one', source)}><RefreshCw size={12} /> Update</button>}
                    <button aria-label={`Uninstall ${source.group}`} className="ghost-button compact-button destructive" disabled={busy || pendingCount > 0} onClick={() => void prepareUninstall(source)}><Trash2 size={12} /> Uninstall</button>
                  </div></td>
                </tr>
              ))}</tbody>
            </table>
          </div>
        )}
      </div>

      {dialog === 'install' && (
        <InstallDialog backend={backend} busy={busy} progress={progress} includeReadOnly={includeReadOnly} error={dialogError} onBusy={onBusy} onResult={onResult} onError={setDialogError} onClose={closeDialog} onAnnounce={onAnnounce} />
      )}
      {(dialog === 'update-all' || dialog === 'update-one') && (
        <ConfirmDialog title={dialog === 'update-all' ? 'Update all repositories?' : `Update ${selectedSource?.group}?`} description={dialog === 'update-all' ? 'Repositories are processed in deterministic order and the batch stops at the first failure.' : 'Skill Manager will fetch origin, validate installed paths, and fast-forward only.'} confirmLabel={dialog === 'update-all' ? 'Update all' : 'Update'} busy={busy} progress={progress} error={dialogError} onClose={closeDialog} onConfirm={() => void runMutation(() => dialog === 'update-all' ? backend.updateAllSources(includeReadOnly) : backend.updateSource(selectedSource!.sourceId, includeReadOnly))} />
      )}
      {dialog === 'uninstall' && selectedSource && (
        <UninstallDialog source={selectedSource} preview={uninstallPreview} busy={busy} progress={progress} error={dialogError} onClose={closeDialog} onConfirm={(confirmation) => void runMutation(() => backend.uninstallSource(selectedSource.sourceId, confirmation, includeReadOnly))} />
      )}
      {dialog === 'extend' && (
        <ExtendDialog sources={sources} backend={backend} busy={busy} progress={progress} includeReadOnly={includeReadOnly} error={dialogError} onBusy={onBusy} onResult={onResult} onError={setDialogError} onClose={closeDialog} />
      )}
    </section>
  )
}

function InstallDialog({ backend, busy, progress, includeReadOnly, error, onBusy, onResult, onError, onClose, onAnnounce }: {
  backend: Backend; busy: boolean; progress: SourceProgress | null; includeReadOnly: boolean; error: string | null
  onBusy: (busy: boolean) => void; onResult: (result: SourceMutationResult) => void; onError: (error: string | null) => void; onClose: () => void; onAnnounce: (message: string) => void
}) {
  const [mode, setMode] = useState<InstallMode>('git')
  const [gitURL, setGitURL] = useState('')
  const [draft, setDraft] = useState<InstallDraft | null>(null)
  const [review, setReview] = useState<InstallReview | null>(null)
  const [selections, setSelections] = useState<Set<string>>(new Set())
  const initialFocus = useRef<HTMLInputElement>(null)

  useEffect(() => { initialFocus.current?.focus() }, [])
  useDialogEscape(onClose, busy)

  const inspect = async () => {
    onBusy(true); onError(null)
    try {
      const next = mode === 'git' ? await backend.prepareGitInstall(gitURL) : await backend.chooseLocalInstall()
      if (next.cancelled) return
      setDraft(next)
      setReview(null)
      setSelections(new Set(next.candidates.flatMap((candidate) => MANAGED_TOOLS.map((tool) => candidate[tool]).filter((cell) => cell.status !== 'conflict').map((cell) => key(candidate.name, cell.tool)))))
      if (next.cloned) onAnnounce('Repository cloned for inspection. The checkout will be retained if you cancel.')
    } catch (reason) { onError(errorMessage(reason)) } finally { onBusy(false) }
  }

  const reviewSelection = async () => {
    if (!draft) return
    onBusy(true); onError(null)
    try {
      const next = await backend.reviewInstall(draft.draftId, selectedRequests(selections))
      setReview(next)
    } catch (reason) { onError(errorMessage(reason)) } finally { onBusy(false) }
  }

  const apply = async () => {
    if (!review?.ready) return
    onBusy(true); onError(null)
    try {
      const result = await backend.applyInstall(review.reviewId!, includeReadOnly)
      onResult(result)
      if (!result.failure) onClose()
      else onError(result.failure.message)
    } catch (reason) { onError(errorMessage(reason)) } finally { onBusy(false) }
  }

  const toggle = (skill: string, tool: string) => {
    setReview(null)
    setSelections((current) => { const next = new Set(current); const cell = key(skill, tool); next.has(cell) ? next.delete(cell) : next.add(cell); return next })
  }

  const setToolSelection = (tool: ManagedTool, selected: boolean) => {
    if (!draft) return
    setReview(null)
    setSelections((current) => {
      const next = new Set(current)
      draft.candidates.forEach((candidate) => {
        if (candidate[tool].status === 'conflict') return
        const cell = key(candidate.name, tool)
        selected ? next.add(cell) : next.delete(cell)
      })
      return next
    })
  }

  return (
    <Modal title="Install source" onClose={onClose} busy={busy} wide>
      {!draft ? <>
        <div className="source-mode-tabs" role="tablist" aria-label="Source type">
          <button role="tab" aria-selected={mode === 'git'} className={mode === 'git' ? 'active' : ''} onClick={() => setMode('git')}><GitBranch size={14} /> Git repository</button>
          <button role="tab" aria-selected={mode === 'local'} className={mode === 'local' ? 'active' : ''} onClick={() => setMode('local')}><FolderOpen size={14} /> Local folder</button>
        </div>
        {mode === 'git' ? <label className="dialog-field"><span>HTTPS or SSH Git URL</span><input ref={initialFocus} value={gitURL} onChange={(event) => setGitURL(event.target.value)} placeholder="https://github.com/owner/repo" /></label> : <div className="local-picker-copy"><FolderOpen size={24} /><strong>Choose a folder containing skills</strong><p>The source remains in place and Skill Manager creates only managed links.</p></div>}
        <div className="dialog-note"><Download size={14} /><p>A missing Git repository is cloned before discovery. A fresh checkout is retained if you close this dialog, so retry stays cheap.</p></div>
        {error && <DialogError message={error} />}
        {busy && <ProgressState progress={progress} />}
        <DialogActions onClose={onClose} busy={busy}><button className="primary-button" disabled={busy || (mode === 'git' && !gitURL.trim())} onClick={() => void inspect()}>{mode === 'git' ? 'Clone & inspect' : 'Choose folder'}</button></DialogActions>
      </> : <>
        <div className="draft-summary"><div><span>{draft.kind === 'git' ? <GitBranch size={14} /> : <HardDrive size={14} />}</span><div><strong>{draft.group}</strong><code>{draft.location}</code></div></div><small>{draft.candidates.length} skill{draft.candidates.length === 1 ? '' : 's'} discovered</small></div>
        <InstallMatrix draft={draft} selections={selections} busy={busy} onToggle={toggle} onSetToolSelection={setToolSelection} />
        {review && <ReviewSummary review={review} />}
        {error && <DialogError message={error} />}
        {busy && <ProgressState progress={progress} />}
        <DialogActions onClose={onClose} busy={busy} secondaryLabel="Cancel">
          {!review?.ready ? <button className="primary-button" disabled={busy || selections.size === 0} onClick={() => void reviewSelection()}>Review {selections.size} target{selections.size === 1 ? '' : 's'}</button> : <button className="primary-button" disabled={busy} onClick={() => void apply()}>Install {review.createCount} link{review.createCount === 1 ? '' : 's'}</button>}
        </DialogActions>
      </>}
    </Modal>
  )
}

function InstallMatrix({ draft, selections, busy, onToggle, onSetToolSelection }: {
  draft: InstallDraft
  selections: Set<string>
  busy: boolean
  onToggle: (skill: string, tool: string) => void
  onSetToolSelection: (tool: ManagedTool, selected: boolean) => void
}) {
  const [query, setQuery] = useState('')
  const visible = useMemo(() => draft.candidates.filter((candidate) => candidate.name.toLowerCase().includes(query.toLowerCase()) || candidate.relativePath.toLowerCase().includes(query.toLowerCase())), [draft, query])
  return <div className="install-matrix-wrap">
    <label className="search-field install-search"><Search size={14} /><input aria-label="Filter discovered skills" value={query} disabled={busy} onChange={(event) => setQuery(event.target.value)} placeholder="Filter discovered skills…" /></label>
    <div className="install-matrix-scroll"><table className="install-matrix"><thead><tr><th scope="col">Skill</th>{MANAGED_TOOLS.map((tool) => <InstallColumnHeader key={tool} tool={tool} draft={draft} selections={selections} busy={busy} onSetToolSelection={onSetToolSelection} />)}</tr></thead><tbody>{visible.map((candidate) => <tr key={candidate.name}><td><strong>{candidate.name}</strong><code>{candidate.relativePath}</code></td>{MANAGED_TOOLS.map((tool) => { const cell = candidate[tool]; const checked = selections.has(key(candidate.name, tool)); return <td key={tool}><label className={`matrix-cell status-${cell.status}`} title={cell.message}><input type="checkbox" aria-label={`${candidate.name} ${tool}`} checked={checked} disabled={busy || cell.status === 'conflict'} onChange={() => onToggle(candidate.name, tool)} /><span>{checked && <Check size={11} />}</span><small>{cell.status.replace('-', ' ')}</small></label></td> })}</tr>)}</tbody></table></div>
  </div>
}

function InstallColumnHeader({ tool, draft, selections, busy, onSetToolSelection }: {
  tool: ManagedTool
  draft: InstallDraft
  selections: Set<string>
  busy: boolean
  onSetToolSelection: (tool: ManagedTool, selected: boolean) => void
}) {
  const applicable = draft.candidates.filter((candidate) => candidate[tool].status !== 'conflict')
  const selectedCount = applicable.filter((candidate) => selections.has(key(candidate.name, tool))).length
  const state: ColumnSelectionState = applicable.length === 0 ? 'N/A' : selectedCount === 0 ? 'OFF' : selectedCount === applicable.length ? 'ON' : 'MIXED'
  const name = toolDisplayName(tool)
  const count = applicable.length === 0 ? 'no available targets' : `${selectedCount} of ${applicable.length} selected`
  const selectAll = state !== 'ON'
  return <th scope="col">
    <button
      className="install-column-toggle"
      data-state={state.toLowerCase().replace('/', '-')}
      aria-label={`${name} all targets: ${state} (${count})`}
      aria-pressed={state === 'MIXED' ? 'mixed' : state === 'ON'}
      title={selectAll ? `Select all ${name} targets` : `Clear all ${name} targets`}
      disabled={busy || state === 'N/A'}
      onClick={() => onSetToolSelection(tool, selectAll)}
    >
      <span>{name}</span><small>{state}</small>
    </button>
  </th>
}

function ReviewSummary({ review }: { review: InstallReview }) {
  return <div className={review.ready ? 'install-review ready' : 'install-review conflict'}>
    <strong>{review.ready ? 'Ready to install' : `${review.conflicts.length} conflict${review.conflicts.length === 1 ? '' : 's'}`}</strong>
    {review.ready ? <p>{review.createCount} new links · {review.alreadyOnCount} already ON · {review.alreadyOffCount} already OFF</p> : review.conflicts.map((conflict) => <p key={`${conflict.skillName}:${conflict.tool}`}>{conflict.skillName} {conflict.tool}: {conflict.reason}</p>)}
  </div>
}

const TOOL_SOURCE_COUNT: Record<ManagedTool, (source: ManagedSource) => number> = {
  claude: (source) => source.claudeCount,
  codex: (source) => source.codexCount,
  muse: (source) => source.museCount,
  grok: (source) => source.grokCount,
}

function toolSourceCount(source: ManagedSource, tool: ManagedTool): number {
  return TOOL_SOURCE_COUNT[tool](source)
}

function canExtendSources(sources: ManagedSource[]): boolean {
  return sources.length > 0 && sources.some((source) => MANAGED_TOOLS.some((tool) => toolSourceCount(source, tool) < source.skillCount))
}

function defaultExtendTool(sources: ManagedSource[]): ManagedTool {
  for (const tool of MANAGED_TOOLS) {
    if (sources.some((source) => toolSourceCount(source, tool) < source.skillCount)) return tool
  }
  return 'muse'
}

function ExtendDialog({ sources, backend, busy, progress, includeReadOnly, error, onBusy, onResult, onError, onClose }: {
  sources: ManagedSource[]; backend: Backend; busy: boolean; progress: SourceProgress | null; includeReadOnly: boolean; error: string | null
  onBusy: (busy: boolean) => void; onResult: (result: SourceMutationResult) => void; onError: (error: string | null) => void; onClose: () => void
}) {
  const fallbackTool = useMemo(() => defaultExtendTool(sources), [sources])
  const [tool, setTool] = useState<ManagedTool>(fallbackTool)
  const [preview, setPreview] = useState<ExtendPreview | null>(null)
  useDialogEscape(onClose, busy)

  const loadPreview = async (next: ManagedTool) => {
    onBusy(true); onError(null)
    try {
      setPreview(await backend.previewExtend(next))
    } catch (reason) { setPreview(null); onError(errorMessage(reason)) } finally { onBusy(false) }
  }

  useEffect(() => { void loadPreview(fallbackTool) }, []) // eslint-disable-line react-hooks/exhaustive-deps

  const selectTool = (next: ManagedTool) => {
    if (next === tool) return
    setTool(next)
    setPreview(null)
    void loadPreview(next)
  }

  const resetTool = () => {
    onError(null)
    setPreview(null)
    setTool(fallbackTool)
    void loadPreview(fallbackTool)
  }

  const confirm = async () => {
    if (!preview || busy || !canConfirmExtend(preview)) return
    onBusy(true); onError(null)
    try {
      const result = await backend.extendSources(tool, includeReadOnly)
      onResult(result)
      if (!result.failure) onClose()
      else onError(result.failure.message)
    } catch (reason) { onError(errorMessage(reason)) } finally { onBusy(false) }
  }

  const created = preview?.sources.reduce((total, source) => total + source.created, 0) ?? 0
  const already = preview?.sources.reduce((total, source) => total + source.alreadyInstalled, 0) ?? 0
  const disabled = preview?.sources.reduce((total, source) => total + source.disabledAfter, 0) ?? 0
  const showReset = error !== null && /unknown tool/i.test(error)
  const confirmBlocked = !preview || !canConfirmExtend(preview)

  return <Modal title={`Extend sources to ${toolDisplayName(tool)}?`} onClose={onClose} busy={busy}>
    <p className="dialog-description">Missing {toolDisplayName(tool)} links are created for every managed source and mirrored OFF where the skill is OFF everywhere else. The batch stops at the first failure.</p>
    <div className="extend-tool-radio" role="radiogroup" aria-label="Target tool">
      {MANAGED_TOOLS.map((option) => (
        <label key={option} className={tool === option ? 'active' : ''}><input type="radio" name="extend-tool" checked={tool === option} disabled={busy} onChange={() => selectTool(option)} /> {toolDisplayName(option)}</label>
      ))}
    </div>
    {preview && <div className={preview.blockedCount > 0 ? 'install-review conflict' : 'install-review ready'}>
      <strong>{preview.blockedCount > 0 ? `${preview.blockedCount} blocked source${preview.blockedCount === 1 ? '' : 's'}` : 'Ready to extend'}</strong>
      <p>{created} new link{created === 1 ? '' : 's'} · {already} already installed · {disabled} disabled after</p>
      {preview.sources.map((source) => <p key={`${source.kind}:${source.group}`}>{source.group} ({source.kind}): {source.status === 'ready' ? `${source.created} new · ${source.alreadyInstalled} already · ${source.disabledAfter} OFF after` : extendSourceBlockage(source)}</p>)}
    </div>}
    {error && <DialogError message={error} />}
    {showReset && <div className="dialog-actions"><button className="secondary-button" disabled={busy} onClick={resetTool}>Reset tool</button></div>}
    {busy && <ProgressState progress={progress} />}
    <DialogActions onClose={onClose} busy={busy}><button className="primary-button" disabled={busy || confirmBlocked} onClick={() => void confirm()}>Extend to {toolDisplayName(tool)}</button></DialogActions>
  </Modal>
}

function canConfirmExtend(preview: ExtendPreview): boolean {
  return preview.blockedCount === 0 && preview.createCount > 0
}

function extendSourceBlockage(source: ExtendPreviewSource): string {
  const details = source.conflicts.map((conflict) => `${conflict.skillName}: ${conflict.reason}`)
  for (const skipped of source.skipped) details.push(`${skipped.skillName}: ${skipped.reason}`)
  if (source.reason) details.unshift(source.reason)
  return `${source.status}${details.length > 0 ? ` — ${details.join('; ')}` : ''}`
}

function ConfirmDialog({ title, description, confirmLabel, busy, progress, error, onClose, onConfirm }: { title: string; description: string; confirmLabel: string; busy: boolean; progress: SourceProgress | null; error: string | null; onClose: () => void; onConfirm: () => void }) {
  useDialogEscape(onClose, busy)
  return <Modal title={title} onClose={onClose} busy={busy}><p className="dialog-description">{description}</p>{error && <DialogError message={error} />}{busy && <ProgressState progress={progress} />}<DialogActions onClose={onClose} busy={busy}><button className="primary-button" disabled={busy} onClick={onConfirm}>{confirmLabel}</button></DialogActions></Modal>
}

function UninstallDialog({ source, preview, busy, progress, error, onClose, onConfirm }: { source: ManagedSource; preview: UninstallPreview | null; busy: boolean; progress: SourceProgress | null; error: string | null; onClose: () => void; onConfirm: (confirmation: string) => void }) {
  const [confirmation, setConfirmation] = useState('')
  useDialogEscape(onClose, busy)
  return <Modal title={`Uninstall ${source.group}?`} onClose={onClose} busy={busy}>
    {!preview && busy ? <ProgressState progress={progress} /> : preview && <><div className="uninstall-impact"><Trash2 size={21} /><div><strong>This removes the complete managed source</strong><p>{preview.activeLinks} active and {preview.disabledLinks} disabled link{preview.activeLinks + preview.disabledLinks === 1 ? '' : 's'}.</p><p>{preview.removesCheckout ? 'The managed Git checkout will be deleted.' : 'The local source folder will be preserved.'}</p></div></div>{preview.affectedSkillSets?.length > 0 && <div className="skill-set-uninstall-impact"><AlertTriangle size={16} /><div><strong>{preview.affectedSkillSets.length} Skill Set{preview.affectedSkillSets.length === 1 ? '' : 's'} {preview.affectedSkillSets.length === 1 ? 'contains' : 'contain'} skills installed by this source</strong>{preview.affectedSkillSets.map((set) => <p key={set.setId}>{set.name}: {set.skills.join(', ')}</p>)}<p>Recipes are kept; removed tool cells may become unavailable.</p></div></div>}{preview.affectedFavorites?.length > 0 && <div className="skill-set-uninstall-impact favorite-uninstall-impact"><AlertTriangle size={16} /><div><strong>{preview.affectedFavorites.length} favorite skill{preview.affectedFavorites.length === 1 ? '' : 's'} {preview.affectedFavorites.length === 1 ? 'is' : 'are'} installed by this source</strong><p>{preview.affectedFavorites.join(', ')}</p><p>Favorites are remembered; removed skills may become unavailable until reinstalled.</p></div></div>}{preview.skillSetImpactWarning && <div className="dialog-error"><AlertTriangle size={14} /><span>Skill Set impact could not be checked: {preview.skillSetImpactWarning}</span></div>}{preview.favoriteImpactWarning && <div className="dialog-error"><AlertTriangle size={14} /><span>Favorite impact could not be checked: {preview.favoriteImpactWarning}</span></div>}<label className="dialog-field"><span>Type <strong>{source.group}</strong> to confirm</span><input autoFocus value={confirmation} onChange={(event) => setConfirmation(event.target.value)} /></label></>}
    {error && <DialogError message={error} />}{busy && preview && <ProgressState progress={progress} />}
    <DialogActions onClose={onClose} busy={busy}><button className="danger-button" disabled={busy || !preview || confirmation !== source.group} onClick={() => onConfirm(confirmation)}>Uninstall source</button></DialogActions>
  </Modal>
}

function Modal({ title, onClose, busy, wide = false, children }: { title: string; onClose: () => void; busy: boolean; wide?: boolean; children: React.ReactNode }) {
  const modalRef = useRef<HTMLElement>(null)
  useEffect(() => {
    const modal = modalRef.current
    if (!modal) return
    const trapFocus = (event: KeyboardEvent) => {
      if (event.key !== 'Tab') return
      const focusable = [...modal.querySelectorAll<HTMLElement>('button:not(:disabled), input:not(:disabled), select:not(:disabled), [tabindex]:not([tabindex="-1"])')]
      if (focusable.length === 0) return
      const first = focusable[0]
      const last = focusable[focusable.length - 1]
      if (!modal.contains(document.activeElement)) { event.preventDefault(); (event.shiftKey ? last : first).focus() }
      else if (event.shiftKey && document.activeElement === first) { event.preventDefault(); last.focus() }
      else if (!event.shiftKey && document.activeElement === last) { event.preventDefault(); first.focus() }
    }
    modal.addEventListener('keydown', trapFocus)
    if (!modal.contains(document.activeElement)) {
      const first = modal.querySelector<HTMLElement>('button:not(:disabled), input:not(:disabled), select:not(:disabled), [tabindex]:not([tabindex="-1"])')
      ;(first ?? modal).focus()
    }
    return () => modal.removeEventListener('keydown', trapFocus)
  }, [])
  return <div className="modal-backdrop" role="presentation"><section ref={modalRef} tabIndex={-1} className={`source-modal ${wide ? 'wide' : ''}`} role="dialog" aria-modal="true" aria-labelledby="source-modal-title"><header><h2 id="source-modal-title">{title}</h2><button className="icon-button subtle" aria-label="Close dialog" disabled={busy} onClick={onClose}><X size={15} /></button></header><div className="source-modal-body">{children}</div></section></div>
}

function DialogActions({ onClose, busy, secondaryLabel = 'Cancel', children }: { onClose: () => void; busy: boolean; secondaryLabel?: string; children: React.ReactNode }) {
  return <div className="dialog-actions"><button className="secondary-button" disabled={busy} onClick={onClose}>{secondaryLabel}</button>{children}</div>
}

function DialogError({ message }: { message: string }) { return <div className="dialog-error" role="alert"><AlertTriangle size={14} /><span>{message}</span></div> }
function ProgressState({ progress }: { progress: SourceProgress | null }) { return <div className="source-progress" role="status"><LoaderCircle size={17} className="spin" /><div><strong>{progress?.message ?? 'Working…'}</strong>{progress?.group && <small>{progress.group}{progress.total ? ` · ${progress.current}/${progress.total}` : ''}</small>}</div></div> }
function useDialogEscape(onClose: () => void, busy: boolean) { useEffect(() => { const handler = (event: KeyboardEvent) => { if (event.key === 'Escape' && !busy) onClose() }; window.addEventListener('keydown', handler); return () => window.removeEventListener('keydown', handler) }, [busy, onClose]) }
function selectedRequests(selections: Set<string>): InstallCellRequest[] { return [...selections].map((item) => { const [tool, ...name] = item.split(':'); return { tool, skillName: name.join(':') } as InstallCellRequest }) }
function key(skill: string, tool: string) { return `${tool}:${skill}` }
function shortCommit(commit: string) { return commit.slice(0, 8) }
function errorMessage(reason: unknown) { return reason instanceof Error ? reason.message : String(reason) }
