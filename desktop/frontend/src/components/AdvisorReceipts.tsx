import { useCallback, useEffect, useRef, useState } from 'react'
import { AlertTriangle, Eraser, X } from 'lucide-react'
import type { AdvisorReceipt, AdvisorReceiptResult, Backend } from '../api'

interface AdvisorReceiptsProps {
  backend: Backend
  includeReadOnly: boolean
  pendingCount: number
  busy: boolean
  onBusy: (busy: boolean) => void
  onResult: (result: AdvisorReceiptResult) => void
}

type Confirm = { kind: 'cleanup'; receipt: AdvisorReceipt } | { kind: 'forget'; receipt: AdvisorReceipt } | { kind: 'all' }

const TOOL_LABELS: Record<string, string> = { claude: 'Claude', codex: 'Codex', muse: 'Muse', grok: 'Grok' }
const ACTION_LABELS: Record<string, string> = { disable: 'turns off', release: 'stays on, shared', already_off: 'already off' }

// AdvisorReceipts lists recorded advisor receipts and releases them on request
// (Iteration 28). A receipt is cleaned or forgotten only on a user action.
export default function AdvisorReceipts({ backend, includeReadOnly, pendingCount, busy, onBusy, onResult }: AdvisorReceiptsProps) {
  const [receipts, setReceipts] = useState<AdvisorReceipt[] | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [confirm, setConfirm] = useState<Confirm | null>(null)
  const [confirmError, setConfirmError] = useState<string | null>(null)

  const load = useCallback(async () => {
    try {
      setReceipts(await backend.listAdvisorReceipts())
      setError(null)
    } catch (reason) {
      setError(errorMessage(reason))
    }
  }, [backend])

  useEffect(() => { void load() }, [load])

  const blocked = (receipts ?? []).filter((receipt) => receipt.blocked)
  const releasable = (receipts ?? []).filter((receipt) => !receipt.blocked)
  const locked = busy || pendingCount > 0

  const run = async (action: () => Promise<AdvisorReceiptResult>) => {
    onBusy(true)
    setConfirmError(null)
    try {
      const result = await action()
      setReceipts(result.receipts)
      onResult(result)
      if (result.failure) setConfirmError(result.failure.message)
      else setConfirm(null)
    } catch (reason) {
      setConfirmError(errorMessage(reason))
    } finally {
      onBusy(false)
    }
  }

  const confirmAction = () => {
    if (!confirm) return
    if (confirm.kind === 'all') void run(() => backend.cleanupAllAdvisorReceipts(includeReadOnly))
    else if (confirm.kind === 'cleanup') void run(() => backend.cleanupAdvisorReceipt(confirm.receipt.receiptId, includeReadOnly))
    else void run(() => backend.forgetAdvisorReceipt(confirm.receipt.receiptId, includeReadOnly))
  }

  return (
    <article className="advisor-panel">
      <div className="receipts-heading">
        <div>
          <h2>Receipts</h2>
          <p className="dialog-description">Skills that the advisor turned on for an agent session. A session that ends early leaves its receipt here.</p>
        </div>
        <button type="button" className="secondary-button" disabled={locked || releasable.length === 0} onClick={() => { setConfirmError(null); setConfirm({ kind: 'all' }) }}><Eraser size={14} /> Clean up all</button>
      </div>
      {pendingCount > 0 && <p className="advisor-status">Apply or clear pending skill changes before you clean up receipts.</p>}
      {error && <p className="advisor-error" role="alert">{error}</p>}
      {receipts && receipts.length === 0 && <p className="advisor-status">No receipts. Every temporary activation was cleaned up.</p>}
      {receipts && receipts.length > 0 && (
        <ul className="receipt-list">{receipts.map((receipt) => (
          <li key={receipt.receiptId} aria-label={`${toolLabel(receipt.tool)} receipt ${shortID(receipt.receiptId)}`} className={receipt.blocked ? 'receipt-row blocked' : 'receipt-row'}>
            <div className="receipt-meta">
              <strong>{toolLabel(receipt.tool)}</strong>
              <time dateTime={receipt.createdAt} title={new Date(receipt.createdAt).toLocaleString()}>{relativeTime(receipt.createdAt)}</time>
              <code>{shortID(receipt.receiptId)}</code>
            </div>
            <ul className="receipt-skills">{receipt.skills.map((skill) => (
              <li key={skill.name}><span>{skill.name}</span>{skill.action && <small>{ACTION_LABELS[skill.action] ?? skill.action}</small>}</li>
            ))}</ul>
            {receipt.blocked && <p className="receipt-cause"><AlertTriangle size={13} />{receipt.cause}</p>}
            <div className="receipt-actions">
              {receipt.blocked
                ? <button type="button" className="secondary-button compact-button destructive" disabled={locked} onClick={() => { setConfirmError(null); setConfirm({ kind: 'forget', receipt }) }}>Forget</button>
                : <button type="button" className="secondary-button compact-button" disabled={locked} onClick={() => { setConfirmError(null); setConfirm({ kind: 'cleanup', receipt }) }}>Clean up</button>}
            </div>
          </li>
        ))}</ul>
      )}
      {confirm && (
        <ReceiptDialog
          title={confirm.kind === 'all' ? 'Clean up all receipts?' : confirm.kind === 'cleanup' ? 'Clean up receipt?' : 'Forget blocked receipt?'}
          confirmLabel={confirm.kind === 'all' ? `Clean up ${releasable.length} receipt${releasable.length === 1 ? '' : 's'}` : confirm.kind === 'cleanup' ? 'Clean up' : 'Forget receipt'}
          danger={confirm.kind === 'forget'}
          busy={busy}
          error={confirmError}
          onClose={() => { if (!busy) setConfirm(null) }}
          onConfirm={confirmAction}
        >
          {confirm.kind === 'forget' ? <>
            <p className="dialog-description">{confirm.receipt.cause}</p>
            <p className="dialog-description">Skill Manager removes only the receipt record. It will not turn these skills off: {confirm.receipt.skills.map((skill) => skill.name).join(', ')}. Toggle them in Skills if necessary.</p>
          </> : <>
            {confirm.kind === 'all'
              ? <p className="dialog-description">{releasable.length} receipt{releasable.length === 1 ? '' : 's'} will be released in list order. The batch stops at the first failure.{blocked.length > 0 ? ` ${blocked.length} blocked receipt${blocked.length === 1 ? ' is' : 's are'} skipped.` : ''}</p>
              : <p className="dialog-description">{confirm.receipt.skills.map((skill) => `${skill.name} ${ACTION_LABELS[skill.action ?? ''] ?? ''}`.trim()).join(', ')}.</p>}
            <p className="dialog-description">An open agent session can still use these skills. Cleanup turns them off for that session.</p>
          </>}
        </ReceiptDialog>
      )}
    </article>
  )
}

function ReceiptDialog({ title, confirmLabel, danger, busy, error, onClose, onConfirm, children }: { title: string; confirmLabel: string; danger: boolean; busy: boolean; error: string | null; onClose: () => void; onConfirm: () => void; children: React.ReactNode }) {
  const ref = useRef<HTMLElement>(null)
  const closeRef = useRef(onClose)
  closeRef.current = onClose
  // Contain focus like the source dialogs, and return it to the trigger on close.
  useEffect(() => {
    const modal = ref.current
    if (!modal) return
    const trigger = document.activeElement instanceof HTMLElement ? document.activeElement : null
    const handler = (event: KeyboardEvent) => {
      if (event.key === 'Escape') { closeRef.current(); return }
      if (event.key !== 'Tab') return
      const focusable = [...modal.querySelectorAll<HTMLElement>('button:not(:disabled)')]
      if (focusable.length === 0) { event.preventDefault(); modal.focus(); return }
      const first = focusable[0]
      const last = focusable[focusable.length - 1]
      if (!modal.contains(document.activeElement) || document.activeElement === modal) { event.preventDefault(); (event.shiftKey ? last : first).focus() }
      else if (event.shiftKey && document.activeElement === first) { event.preventDefault(); last.focus() }
      else if (!event.shiftKey && document.activeElement === last) { event.preventDefault(); first.focus() }
    }
    window.addEventListener('keydown', handler)
    ;(modal.querySelector<HTMLElement>('button:not(:disabled)') ?? modal).focus()
    return () => {
      window.removeEventListener('keydown', handler)
      if (trigger?.isConnected) trigger.focus()
    }
  }, [])
  return <div className="modal-backdrop" role="presentation"><section ref={ref} tabIndex={-1} className="source-modal" role="dialog" aria-modal="true" aria-labelledby="receipt-modal-title">
    <header><h2 id="receipt-modal-title">{title}</h2><button className="icon-button subtle" aria-label="Close dialog" disabled={busy} onClick={onClose}><X size={15} /></button></header>
    <div className="source-modal-body">
      {children}
      {error && <div className="dialog-error" role="alert"><AlertTriangle size={14} /><span>{error}</span></div>}
      <div className="dialog-actions"><button className="secondary-button" disabled={busy} onClick={onClose}>Cancel</button><button className={danger ? 'danger-button' : 'primary-button'} disabled={busy} onClick={onConfirm}>{confirmLabel}</button></div>
    </div>
  </section></div>
}

function toolLabel(tool: string) { return TOOL_LABELS[tool] ?? tool }
function shortID(id: string) { return id.slice(0, 8) }
function errorMessage(reason: unknown) { return reason instanceof Error ? reason.message : String(reason) }

// relativeTime renders an ISO time as "just now", "N minutes ago", and so on.
function relativeTime(iso: string, now = Date.now()) {
  const seconds = Math.max(0, Math.round((now - new Date(iso).getTime()) / 1000))
  const units: Array<[number, string]> = [[86_400, 'day'], [3_600, 'hour'], [60, 'minute']]
  for (const [size, unit] of units) {
    const count = Math.floor(seconds / size)
    if (count >= 1) return `${count} ${unit}${count === 1 ? '' : 's'} ago`
  }
  return 'just now'
}
