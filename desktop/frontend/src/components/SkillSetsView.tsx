import { useEffect, useMemo, useRef, useState } from 'react'
import {
  AlertTriangle,
  ArrowUpDown,
  BookmarkPlus,
  Check,
  ChevronDown,
  ListChecks,
  Pencil,
  Plus,
  Search,
  Trash2,
  X,
} from 'lucide-react'
import { MANAGED_TOOLS, joinList, toolDisplayName } from '../api'
import type {
  ActionResult,
  Backend,
  ManagedTool,
  SkillRow,
  SkillSet,
  SkillSetMutationResult,
  SkillSetTogglePreview,
  Snapshot,
} from '../api'

export interface SkillSetEditorRequest {
  id: number
  kind: 'pending' | 'skill'
  skillNames: string[]
}

interface SkillSetsViewProps {
  snapshot: Snapshot
  busy: boolean
  backend: Backend
  editorRequest: SkillSetEditorRequest | null
  onEditorRequestHandled: () => void
  onBusy: (busy: boolean) => void
  onMutation: (result: SkillSetMutationResult) => void
  onAction: (result: ActionResult) => void
  onAnnounce: (message: string) => void
}

interface EditorState {
  setID?: string
  name: string
  description: string
  selected: Set<string>
  query: string
}

type ToolChoice = '' | ManagedTool | 'all'

export default function SkillSetsView(props: SkillSetsViewProps) {
  const { snapshot, busy, backend, editorRequest } = props
  const [query, setQuery] = useState('')
  const [expandedSetID, setExpandedSetID] = useState<string | null>(null)
  const [editor, setEditor] = useState<EditorState | null>(null)
  const [addSkillName, setAddSkillName] = useState<string | null>(null)
  const [toggleSet, setToggleSet] = useState<SkillSet | null>(null)
  const [toolChoice, setToolChoice] = useState<ToolChoice>('')
  const [preview, setPreview] = useState<SkillSetTogglePreview | null>(null)
  const [previewBusy, setPreviewBusy] = useState(false)
  const [deleteSet, setDeleteSet] = useState<SkillSet | null>(null)
  const [dialogError, setDialogError] = useState<string | null>(null)

  const openEditor = (set?: SkillSet, seed: string[] = []) => {
    setDialogError(null)
    setEditor({
      setID: set?.setId,
      name: set?.name ?? '',
      description: set?.description ?? '',
      selected: new Set([...(set?.members.map((member) => member.name) ?? []), ...seed]),
      query: '',
    })
  }

  useEffect(() => {
    if (!editorRequest) return
    const names = [...new Set(editorRequest.skillNames)].sort()
    if (editorRequest.kind === 'skill' && snapshot.skillSets.length > 0) setAddSkillName(names[0] ?? null)
    else openEditor(undefined, names)
    props.onEditorRequestHandled()
  // The request ID intentionally owns this one-shot transition.
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [editorRequest?.id])

  const matchingSets = useMemo(() => {
    const normalized = query.trim().toLowerCase()
    return snapshot.skillSets.filter((set) => !normalized || [set.name, set.description, ...set.members.map((member) => member.name)].some((value) => value.toLowerCase().includes(normalized)))
  }, [query, snapshot.skillSets])

  const runMutation = async (action: () => Promise<SkillSetMutationResult>, close: () => void) => {
    props.onBusy(true)
    setDialogError(null)
    try {
      const result = await action()
      props.onMutation(result)
      props.onAnnounce(result.message)
      close()
    } catch (reason) {
      setDialogError(errorMessage(reason))
    } finally {
      props.onBusy(false)
    }
  }

  const saveEditor = () => {
    if (!editor) return
    const skills = [...editor.selected].sort()
    const action = editor.setID
      ? () => backend.updateSkillSet(editor.setID!, editor.name, editor.description, skills)
      : () => backend.createSkillSet(editor.name, editor.description, skills)
    void runMutation(action, () => setEditor(null))
  }

  const selectToolChoice = async (choice: ToolChoice) => {
    if (!toggleSet || !choice) return
    setToolChoice(choice)
    setPreview(null)
    setDialogError(null)
    setPreviewBusy(true)
    try {
      setPreview(await backend.previewSkillSetToggle(toggleSet.setId, toolsForChoice(choice)))
    } catch (reason) {
      setDialogError(errorMessage(reason))
    } finally {
      setPreviewBusy(false)
    }
  }

  const stageToggle = async () => {
    if (!toggleSet || !toolChoice || !preview) return
    props.onBusy(true)
    setDialogError(null)
    try {
      const result = await backend.toggleSkillSet(toggleSet.setId, toolsForChoice(toolChoice))
      props.onAction(result)
      props.onAnnounce(result.message)
      setToggleSet(null)
    } catch (reason) {
      setDialogError(errorMessage(reason))
    } finally {
      props.onBusy(false)
    }
  }

  const openToggle = (set: SkillSet) => {
    setToggleSet(set)
    setToolChoice('')
    setPreview(null)
    setDialogError(null)
  }

  return (
    <section className="page skill-sets-page" aria-labelledby="skill-sets-title">
      <div className="page-heading compact skill-sets-heading">
        <div><p className="eyebrow">Reusable task recipes</p><h1 id="skill-sets-title">Skill Sets</h1><p>Remember useful combinations and stage them for {joinList(MANAGED_TOOLS.map(toolDisplayName), 'or')} when the task returns.</p></div>
        <button className="primary-button" onClick={() => openEditor()} disabled={busy || Boolean(snapshot.skillSetsWarning)}><Plus size={16} /> New Skill Set</button>
      </div>

      {snapshot.skillSetsWarning && <div className="skill-sets-warning" role="alert"><AlertTriangle size={17} /><div><strong>Skill Sets are unavailable</strong><p>{snapshot.skillSetsWarning}</p><small>Skills, Sources, and normal toggles remain available.</small></div></div>}

      <div className="skill-sets-toolbar">
        <label className="search-field" htmlFor="skill-set-search"><Search size={16} /><input id="skill-set-search" value={query} onChange={(event) => setQuery(event.target.value)} placeholder="Search sets or members…" aria-label="Search Skill Sets" /></label>
        <span><strong>{matchingSets.length}</strong> of {snapshot.skillSets.length} sets</span>
      </div>

      {snapshot.skillSets.length === 0 && !snapshot.skillSetsWarning ? (
        <div className="empty-table skill-sets-empty"><BookmarkPlus size={27} /><strong>No Skill Sets yet</strong><p>Create one here, save the current Pending batch, or add a skill from its details.</p><button className="secondary-button" onClick={() => openEditor()}><Plus size={14} /> Create first set</button></div>
      ) : matchingSets.length === 0 ? (
        <div className="empty-table"><Search size={24} /><strong>No matching sets</strong><p>Clear the search to see every saved recipe.</p></div>
      ) : (
        <div className="skill-sets-table-wrap">
          <table className="skill-sets-table">
            <thead><tr><th>Skill Set</th><th>Skills</th>{MANAGED_TOOLS.map((tool) => <th key={tool}>{toolDisplayName(tool)}</th>)}<th><span className="sr-only">Actions</span></th></tr></thead>
            <tbody>{matchingSets.map((set) => {
              const expanded = expandedSetID === set.setId
              return <SetRows key={set.setId} set={set} expanded={expanded} busy={busy} onExpand={() => setExpandedSetID(expanded ? null : set.setId)} onToggle={() => openToggle(set)} onEdit={() => openEditor(set)} onDelete={() => { setDeleteSet(set); setDialogError(null) }} />
            })}</tbody>
          </table>
        </div>
      )}

      {editor && <SkillSetEditor snapshot={snapshot} editor={editor} busy={busy} error={dialogError} onChange={setEditor} onClose={() => setEditor(null)} onSave={saveEditor} />}
      {addSkillName && <AddSkillDialog skillName={addSkillName} sets={snapshot.skillSets} busy={busy} onClose={() => setAddSkillName(null)} onNew={() => { const name = addSkillName; setAddSkillName(null); openEditor(undefined, [name]) }} onExisting={(set) => { const name = addSkillName; setAddSkillName(null); openEditor(set, [name]) }} />}
      {toggleSet && <ToggleDialog set={toggleSet} choice={toolChoice} preview={preview} previewBusy={previewBusy} busy={busy} error={dialogError} onChoice={(choice) => void selectToolChoice(choice)} onClose={() => setToggleSet(null)} onConfirm={() => void stageToggle()} />}
      {deleteSet && <ConfirmDeleteDialog set={deleteSet} busy={busy} error={dialogError} onClose={() => setDeleteSet(null)} onConfirm={() => void runMutation(() => backend.deleteSkillSet(deleteSet.setId), () => setDeleteSet(null))} />}
    </section>
  )
}

function SetRows({ set, expanded, busy, onExpand, onToggle, onEdit, onDelete }: { set: SkillSet; expanded: boolean; busy: boolean; onExpand: () => void; onToggle: () => void; onEdit: () => void; onDelete: () => void }) {
  return <>
    <tr className={expanded ? 'expanded' : ''}>
      <td><button className="set-identity" onClick={onExpand} aria-expanded={expanded}><ChevronDown size={15} /><span><strong>{set.name}</strong><small>{set.description || 'No “When to use” description'}</small></span></button></td>
      <td><span className="set-member-count">{set.members.length}</span>{set.unavailable > 0 && <small className="set-unavailable">{set.unavailable} unavailable</small>}</td>
      {MANAGED_TOOLS.map((tool) => <td key={tool}><ToolSummary summary={set[tool]} /></td>)}
      <td><div className="set-row-actions"><button className="secondary-button compact-button" onClick={onToggle} disabled={busy}><ArrowUpDown size={13} /> Toggle…</button><button className="icon-button subtle" onClick={onEdit} disabled={busy} aria-label={`Edit ${set.name}`}><Pencil size={14} /></button><button className="icon-button subtle destructive" onClick={onDelete} disabled={busy} aria-label={`Delete ${set.name}`}><Trash2 size={14} /></button></div></td>
    </tr>
    {expanded && <tr className="set-members-row"><td colSpan={3 + MANAGED_TOOLS.length}><div className="set-members-list">{set.members.map((member) => <div className="set-member" key={member.name}><span className="skill-glyph">{initials(member.name)}</span><div><strong>{member.name}</strong><small>{member.available ? `${member.group} · ${member.source || 'unknown'}` : 'Unavailable — kept in this recipe'}</small></div>{MANAGED_TOOLS.map((tool) => <MemberState key={tool} label={toolDisplayName(tool)} state={member[tool].effectiveState} pending={member[tool].pending} />)}</div>)}</div></td></tr>}
  </>
}

function ToolSummary({ summary }: { summary: SkillSet[ManagedTool] }) {
  const changed = summary.appliedStatus !== summary.effectiveStatus
  return <div className="set-tool-summary"><span className={`set-status status-${summary.appliedStatus}`}>{statusLabel(summary.appliedStatus)}</span><small>{summary.on}/{summary.eligible} ON{summary.missing ? ` · ${summary.missing} missing` : ''}</small>{changed && <em>After Apply: {statusLabel(summary.effectiveStatus)}</em>}</div>
}

function MemberState({ label, state, pending }: { label: string; state: string; pending?: string }) {
  return <span className={`member-state state-${state.toLowerCase()}`} title={`${label}: ${state}${pending ? `, pending ${pending}` : ''}`}><small>{label}</small><b>{state}</b>{pending && <i>{pending}</i>}</span>
}

function SkillSetEditor({ snapshot, editor, busy, error, onChange, onClose, onSave }: { snapshot: Snapshot; editor: EditorState; busy: boolean; error: string | null; onChange: (editor: EditorState) => void; onClose: () => void; onSave: () => void }) {
  const candidates = useMemo(() => {
    const rows = snapshot.rows.filter(isToggleableRow)
    const allRows = new Map(snapshot.rows.map((row) => [row.name, row]))
    const byName = new Map(rows.map((row) => [row.name, row]))
    for (const name of editor.selected) if (!byName.has(name)) byName.set(name, allRows.get(name) ?? { name, description: '', group: 'unknown', source: 'unknown' } as SkillRow)
    const normalized = editor.query.trim().toLowerCase()
    return [...byName.values()].filter((row) => !normalized || [row.name, row.description, row.group, row.source].some((value) => value.toLowerCase().includes(normalized))).sort((left, right) => left.name.localeCompare(right.name))
  }, [editor.query, editor.selected, snapshot.rows])
  const toggle = (name: string) => {
    const selected = new Set(editor.selected)
    if (selected.has(name)) selected.delete(name)
    else selected.add(name)
    onChange({ ...editor, selected })
  }
  useDialogEscape(onClose, busy)
  return <SetModal title={editor.setID ? 'Edit Skill Set' : 'New Skill Set'} onClose={onClose} busy={busy} wide>
    <div className="set-editor-fields"><label className="dialog-field"><span>Name</span><input autoFocus data-initial-focus value={editor.name} onChange={(event) => onChange({ ...editor, name: event.target.value })} placeholder="Video production" /></label><label className="dialog-field"><span>When to use <small>optional</small></span><textarea value={editor.description} onChange={(event) => onChange({ ...editor, description: event.target.value })} placeholder="Use when creating an occasional project video." rows={2} /></label></div>
    <div className="set-editor-selection"><div className="set-editor-selection-head"><label className="search-field"><Search size={14} /><input value={editor.query} onChange={(event) => onChange({ ...editor, query: event.target.value })} aria-label="Search skills for Skill Set" placeholder="Search available skills…" /></label><span><strong>{editor.selected.size}</strong> selected</span></div><div className="set-editor-list">{candidates.map((row) => { const missing = !isToggleableRow(row); return <label className={`set-editor-row ${missing ? 'missing' : ''}`} key={row.name}><input type="checkbox" checked={editor.selected.has(row.name)} onChange={() => toggle(row.name)} /><span><strong>{row.name}</strong><small>{missing ? 'Unavailable — saved member' : `${row.group} · ${row.description || 'No description'}`}</small></span>{MANAGED_TOOLS.map((tool) => <EditorCell key={tool} state={row[tool]?.state ?? '—'} />)}</label>})}</div></div>
    {error && <DialogError message={error} />}
    <DialogActions onClose={onClose} busy={busy}><button className="primary-button" disabled={busy || !editor.name.trim() || editor.selected.size === 0} onClick={onSave}><Check size={15} /> Save Skill Set</button></DialogActions>
  </SetModal>
}

function EditorCell({ state }: { state: string }) { return <span className={`editor-cell state-${state.toLowerCase()}`}>{state}</span> }

function AddSkillDialog({ skillName, sets, busy, onClose, onNew, onExisting }: { skillName: string; sets: SkillSet[]; busy: boolean; onClose: () => void; onNew: () => void; onExisting: (set: SkillSet) => void }) {
  useDialogEscape(onClose, busy)
  return <SetModal title={`Add ${skillName} to a Skill Set`} onClose={onClose} busy={busy}><p className="dialog-description">Choose a saved recipe to edit, or start a new one with this skill selected.</p><div className="add-to-set-list">{sets.map((set) => <button className="set-picker-row" key={set.setId} onClick={() => onExisting(set)}><ListChecks size={15} /><span><strong>{set.name}</strong><small>{set.members.length} skills</small></span></button>)}</div><DialogActions onClose={onClose} busy={busy}><button className="primary-button" onClick={onNew}><Plus size={15} /> New Skill Set</button></DialogActions></SetModal>
}

function ToggleDialog({ set, choice, preview, previewBusy, busy, error, onChoice, onClose, onConfirm }: { set: SkillSet; choice: ToolChoice; preview: SkillSetTogglePreview | null; previewBusy: boolean; busy: boolean; error: string | null; onChoice: (choice: ToolChoice) => void; onClose: () => void; onConfirm: () => void }) {
  useDialogEscape(onClose, busy || previewBusy)
  const totalSkipped = preview ? preview.counts.skippedMissing + preview.counts.skippedReadOnly + preview.counts.skippedConflict : 0
  return <SetModal title={`Toggle ${set.name}`} onClose={onClose} busy={busy || previewBusy}><p className="dialog-description">Choose the tool scope for this use. The result is staged in Pending and will not touch files until Apply.</p><div className="set-tool-choice" role="group" aria-label="Tool scope for Skill Set">{([...MANAGED_TOOLS, 'all'] as ToolChoice[]).map((value) => <button key={value} className={choice === value ? 'active' : ''} aria-pressed={choice === value} onClick={() => onChoice(value)} disabled={busy || previewBusy}>{value === 'all' ? 'All' : value ? toolDisplayName(value) : ''}</button>)}</div>{previewBusy && <div className="set-preview-loading" role="status">Calculating smart-toggle preview…</div>}{preview && <div className={`set-toggle-preview direction-${preview.direction}`}><ArrowUpDown size={19} /><div><strong>{preview.direction === 'none' ? 'No eligible cells' : `${titleCase(preview.direction)} ${preview.counts.changed + preview.counts.removed} pending change${preview.counts.changed + preview.counts.removed === 1 ? '' : 's'}`}</strong><p>{preview.eligible} eligible cell{preview.eligible === 1 ? '' : 's'} in {preview.tools.map(titleCase).join(' + ')}.</p>{totalSkipped > 0 && <small>{totalSkipped} skipped · {preview.counts.skippedMissing} missing · {preview.counts.skippedReadOnly} read-only · {preview.counts.skippedConflict} conflict</small>}</div></div>}{error && <DialogError message={error} />}<DialogActions onClose={onClose} busy={busy || previewBusy}><button className="primary-button" disabled={busy || previewBusy || !preview || preview.direction === 'none' || preview.counts.changed + preview.counts.removed === 0} onClick={onConfirm}><ArrowUpDown size={15} /> Stage {preview?.direction === 'enable' ? 'enable' : preview?.direction === 'disable' ? 'disable' : 'changes'}</button></DialogActions></SetModal>
}

function ConfirmDeleteDialog({ set, busy, error, onClose, onConfirm }: { set: SkillSet; busy: boolean; error: string | null; onClose: () => void; onConfirm: () => void }) {
  useDialogEscape(onClose, busy)
  return <SetModal title={`Delete ${set.name}?`} onClose={onClose} busy={busy}><div className="delete-set-message"><Trash2 size={20} /><div><strong>This deletes only the saved recipe</strong><p>Skill state and existing Pending changes will not be modified. A private metadata backup is created first.</p></div></div>{error && <DialogError message={error} />}<DialogActions onClose={onClose} busy={busy}><button className="danger-button" disabled={busy} onClick={onConfirm}>Delete Skill Set</button></DialogActions></SetModal>
}

function SetModal({ title, onClose, busy, wide = false, children }: { title: string; onClose: () => void; busy: boolean; wide?: boolean; children: React.ReactNode }) {
  const ref = useRef<HTMLElement>(null)
  useEffect(() => {
    const modal = ref.current
    if (!modal) return
    const selector = 'button:not(:disabled), input:not(:disabled), textarea:not(:disabled), [tabindex]:not([tabindex="-1"])'
    const trapFocus = (event: KeyboardEvent) => {
      if (event.key !== 'Tab') return
      const focusable = [...modal.querySelectorAll<HTMLElement>(selector)]
      if (focusable.length === 0) return
      const first = focusable[0]
      const last = focusable[focusable.length - 1]
      if (!modal.contains(document.activeElement)) { event.preventDefault(); (event.shiftKey ? last : first).focus() }
      else if (event.shiftKey && document.activeElement === first) { event.preventDefault(); last.focus() }
      else if (!event.shiftKey && document.activeElement === last) { event.preventDefault(); first.focus() }
    }
    modal.addEventListener('keydown', trapFocus)
    const preferred = modal.querySelector<HTMLElement>('[data-initial-focus]')
    const first = modal.querySelector<HTMLElement>(selector)
    ;(preferred ?? first ?? modal).focus()
    return () => modal.removeEventListener('keydown', trapFocus)
  }, [])
  return <div className="modal-backdrop" role="presentation"><section ref={ref} tabIndex={-1} className={`source-modal skill-set-modal ${wide ? 'wide' : ''}`} role="dialog" aria-modal="true" aria-labelledby="skill-set-modal-title"><header><h2 id="skill-set-modal-title">{title}</h2><button className="icon-button subtle" aria-label="Close dialog" disabled={busy} onClick={onClose}><X size={15} /></button></header><div className="source-modal-body">{children}</div></section></div>
}

function DialogActions({ onClose, busy, children }: { onClose: () => void; busy: boolean; children: React.ReactNode }) { return <div className="dialog-actions"><button className="secondary-button" disabled={busy} onClick={onClose}>Cancel</button>{children}</div> }
function DialogError({ message }: { message: string }) { return <div className="dialog-error" role="alert"><AlertTriangle size={14} /><span>{message}</span></div> }
function useDialogEscape(onClose: () => void, busy: boolean) { useEffect(() => { const handler = (event: KeyboardEvent) => { if (event.key === 'Escape' && !busy) onClose() }; window.addEventListener('keydown', handler); return () => window.removeEventListener('keydown', handler) }, [busy, onClose]) }
function toolsForChoice(choice: ToolChoice): ManagedTool[] { return choice === 'all' ? [...MANAGED_TOOLS] : choice ? [choice] : [] }
function titleCase(value: string) { return value ? value[0].toUpperCase() + value.slice(1) : value }
function statusLabel(value: string) { return value.split('-').map(titleCase).join(' ') }
function errorMessage(reason: unknown) { return reason instanceof Error ? reason.message : String(reason) }
function initials(value: string) { return value.split(/[-_\s]+/).filter(Boolean).slice(0, 2).map((part) => part[0]?.toUpperCase()).join('') || 'SK' }
function isToggleableRow(row: SkillRow) { return MANAGED_TOOLS.some((tool) => { const cell = row[tool]; return Boolean(cell && !cell.readOnly && ['ON', 'OFF'].includes(cell.state)) }) }
