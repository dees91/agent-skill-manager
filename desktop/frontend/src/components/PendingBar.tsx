import { Check, ChevronUp, Clock3, RotateCcw, Trash2, X } from 'lucide-react'
import type { PendingChange } from '../api'

interface PendingBarProps {
  pending: PendingChange[]
  reviewOpen: boolean
  busy: boolean
  onToggleReview: () => void
  onCloseReview: () => void
  onUndo: (name: string, tool: string) => void
  onClear: () => void
  onApply: () => void
}

export default function PendingBar({ pending, reviewOpen, busy, onToggleReview, onCloseReview, onUndo, onClear, onApply }: PendingBarProps) {
  const enables = pending.filter((change) => change.operation === 'enable').length
  const disables = pending.length - enables
  return (
    <>
      {reviewOpen && (
        <section className="review-drawer" aria-label="Pending changes review">
          <div className="drawer-header"><div><p className="eyebrow">Review batch</p><h2>Pending changes</h2></div><button className="icon-button" onClick={onCloseReview} aria-label="Close pending review"><X size={17} /></button></div>
          <div className="review-summary"><span><b>{enables}</b> enable</span><span><b>{disables}</b> disable</span><small>Applied disables first, then enables</small></div>
          <div className="pending-list">
            {pending.map((change) => (
              <div className="pending-row" key={`${change.tool}:${change.skillName}`}>
                <span className={`operation-icon ${change.operation}`}><Clock3 size={15} /></span>
                <div><strong>{change.skillName}</strong><small>{change.tool} · {change.operation}</small></div>
                <button className="icon-button subtle" onClick={() => onUndo(change.skillName, change.tool)} aria-label={`Undo ${change.operation} for ${change.skillName} in ${change.tool}`}><RotateCcw size={14} /></button>
              </div>
            ))}
          </div>
        </section>
      )}
      <div className="pending-bar" role="region" aria-label="Pending changes">
        <button className="pending-review-button" onClick={onToggleReview} aria-expanded={reviewOpen}>
          <span className="pending-pulse" /><strong>{pending.length} pending change{pending.length === 1 ? '' : 's'}</strong><span>{enables} enable · {disables} disable</span><ChevronUp size={16} className={reviewOpen ? 'rotated' : ''} />
        </button>
        <div className="pending-actions">
          <button className="ghost-button danger-quiet" onClick={onClear} disabled={busy}><Trash2 size={15} /> Clear</button>
          <button className="primary-button" onClick={onApply} disabled={busy}><Check size={16} /> {busy ? 'Working…' : 'Apply changes'}</button>
        </div>
      </div>
    </>
  )
}
