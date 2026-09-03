import { ArrowRight, Bot, Check, CircleAlert, Gauge, Info, Layers3, Play, ShieldCheck, TerminalSquare } from 'lucide-react'
import { MANAGED_TOOLS, joinList, toolDisplayName, toolFullName, type ContextBudgetToolReport, type ManagedTool, type SkillCell, type Snapshot } from '../api'

const TOOL_TONES: Record<ManagedTool, string> = { claude: 'blue', codex: 'orange', muse: 'cyan', grok: 'purple' }
const TOOL_ICON: Record<ManagedTool, typeof Bot> = { claude: Bot, codex: TerminalSquare, muse: Bot, grok: Bot }

function ToolIcon({ tool, size }: { tool: ManagedTool; size?: number }) {
  const Icon = TOOL_ICON[tool]
  return <Icon size={size} />
}

interface DashboardProps {
  snapshot: Snapshot
  busy: boolean
  onBrowseSkills: () => void
  onMeasureContext: () => void
}

export default function Dashboard({ snapshot, busy, onBrowseSkills, onMeasureContext }: DashboardProps) {
  const perTool = MANAGED_TOOLS.map((tool) => ({ tool, counts: effectiveCounts(snapshot, tool) }))
  const all = perTool.reduce((total, item) => ({
    on: total.on + item.counts.on,
    off: total.off + item.counts.off,
    conflict: total.conflict + item.counts.conflict,
    readOnly: total.readOnly + item.counts.readOnly,
  }), { on: 0, off: 0, conflict: 0, readOnly: 0 })
  const total = all.on + all.off + all.conflict + all.readOnly
  const pending = snapshot.pending.length

  return (
    <section className="page dashboard-page" aria-labelledby="dashboard-title">
      <div className="page-heading">
        <div><p className="eyebrow">Operational overview</p><h1 id="dashboard-title">Dashboard</h1><p>Visibility of local agent skills across {joinList(MANAGED_TOOLS.map(toolFullName), 'and')}.</p></div>
        <button className="primary-button" onClick={onBrowseSkills}>Manage skills <ArrowRight size={16} /></button>
      </div>

      <div className="metric-grid">
        <MetricCard icon={<Layers3 />} value={snapshot.stats.managedSkills} label="Managed skills" meta={`${snapshot.groups.length} source groups`} tone="cyan" />
        {perTool.map(({ tool, counts }) => <MetricCard key={tool} icon={<ToolIcon tool={tool} />} value={counts.on} label={`${toolDisplayName(tool)} enabled`} meta={`${counts.off} off · ${pendingFor(snapshot, tool)} pending`} tone={TOOL_TONES[tool]} />)}
      </div>

      <article className="panel context-budget-panel">
        <div className="panel-header">
          <div><p className="eyebrow">Prompt budget</p><h2>Global skill catalog cost</h2></div>
          <div className="context-header-actions"><span className="panel-badge context-token-badge">≈ 1 token / 4 chars</span><button className="secondary-button" onClick={onMeasureContext} disabled={busy}><Play size={13} /> Run provider diagnostics</button></div>
        </div>
        <div className="context-budget-list">
          {MANAGED_TOOLS.map((tool) => <ContextBudgetRow key={tool} report={snapshot.contextBudgets[tool]} icon={<ToolIcon tool={tool} size={17} />} name={toolFullName(tool)} />)}
        </div>
        <div className="context-budget-note"><Info size={13} /><span>Filesystem estimate by default. Diagnostics run local read-only Codex and Claude commands only when requested; Muse and Grok are always filesystem estimates.</span></div>
      </article>

      <div className="dashboard-grid">
        <article className="panel status-panel">
          <div className="panel-header"><div><p className="eyebrow">Current state</p><h2>Skill visibility</h2></div><span className="panel-badge">{total} tool cells</span></div>
          <div className="status-layout">
            <div className="donut" style={{ '--on': `${portion(all.on, total)}deg`, '--off': `${portion(all.on + all.off, total)}deg`, '--conflict': `${portion(all.on + all.off + all.conflict, total)}deg` } as React.CSSProperties}>
              <div><strong>{percent(all.on, total)}%</strong><span>enabled</span></div>
            </div>
            <div className="legend-list">
              <LegendRow color="green" label="Enabled" value={all.on} total={total} />
              <LegendRow color="gray" label="Disabled" value={all.off} total={total} />
              <LegendRow color="orange" label="Conflict" value={all.conflict} total={total} />
              {all.readOnly > 0 && <LegendRow color="blue" label="Read only" value={all.readOnly} total={total} />}
            </div>
          </div>
          <div className="tool-bars">
            {perTool.map(({ tool, counts }) => <ToolBar key={tool} name={toolFullName(tool)} counts={counts} />)}
          </div>
        </article>

        <article className="panel groups-panel">
          <div className="panel-header"><div><p className="eyebrow">Collections</p><h2>Largest groups</h2></div><span className="panel-badge">{snapshot.groups.length}</span></div>
          <div className="group-list">
            {snapshot.groups.slice(0, 6).map((group, index) => (
              <div className="group-row" key={group.group}>
                <span className={`group-icon tone-${index % 3}`}><Layers3 size={15} /></span>
                <div><strong>{group.group}</strong><small>{group.sources.join(' · ')}</small></div>
                <span className="group-count">{group.rows}</span>
              </div>
            ))}
            {snapshot.groups.length === 0 && <p className="empty-copy">No managed source groups found.</p>}
          </div>
        </article>
      </div>

      <article className="panel health-panel">
        <div className="panel-header"><div><p className="eyebrow">Safety</p><h2>Apply readiness</h2></div></div>
        {snapshot.conflicts.length === 0 ? (
          <div className="health-ok"><span><ShieldCheck size={23} /></span><div><strong>No restore conflicts detected</strong><p>Pending changes will still receive a fresh filesystem preflight before Apply.</p></div><Check size={18} /></div>
        ) : (
          <div className="conflict-list">
            {snapshot.conflicts.map((conflict) => (
              <div className="conflict-row" key={`${conflict.tool}:${conflict.skillName}`}>
                <CircleAlert size={18} /><div><strong>{conflict.skillName}</strong><small>{conflict.tool} · {conflict.group}</small></div><code>{conflict.blockerPath}</code>
              </div>
            ))}
          </div>
        )}
      </article>
    </section>
  )
}

function ContextBudgetRow({ report, icon, name }: { report: ContextBudgetToolReport; icon: React.ReactNode; name: string }) {
  const current = report.current
  const projected = report.projected
  const currentWidth = clampPercent(current.usedPercent)
  const projectedWidth = clampPercent(projected.usedPercent)
  const health = report.projectionChanged ? projected.health : current.health
  const contextLabel = report.contextWindowTokens > 0
    ? `${formatCount(report.contextWindowTokens)} context${report.contextWindowAssumed ? ' assumed' : ''}`
    : 'context unknown'
  const pressureLabel = `${formatPercent(current.usedPercent)} of ${report.budgetLabel}`
  const trimming = current.shortenedDescriptions + current.omittedSkills

  return (
    <div className={`context-budget-row context-health-${health}`}>
      <div className="context-tool">
        <span>{icon}</span>
        <div><strong>{name}</strong><small>{report.model} · {contextLabel}</small></div>
      </div>
      <div className="context-usage">
        <div className="context-usage-heading">
          <div><strong>≈{formatCount(current.estimatedTokens)}</strong><span> / {formatCount(report.budgetTokens)} tokens</span></div>
          <b>{formatPercent(current.usedPercent)}</b>
        </div>
        <div className="budget-track" role="progressbar" aria-label={`${name} global skill catalog budget`} aria-valuemin={0} aria-valuemax={100} aria-valuenow={Math.min(100, Math.round(current.usedPercent))}>
          <span className="budget-current" style={{ width: `${currentWidth}%` }} />
          {report.projectionChanged && <span className="budget-projected" style={{ left: `calc(${projectedWidth}% - 1px)` }} />}
        </div>
        <div className="context-usage-meta">
          <span>{current.skillCount} skill{current.skillCount === 1 ? '' : 's'} · {pressureLabel}</span>
          {trimming > 0 && <span>{current.shortenedDescriptions} shortened · {current.omittedSkills} omitted</span>}
        </div>
      </div>
      <div className="context-assurance">
        <span className={`accuracy-badge accuracy-${report.accuracy}`}>{accuracyLabel(report.accuracy)}</span>
        {report.projectionChanged ? (
          <div className="after-apply"><small>After Apply</small><strong>{formatPercent(projected.usedPercent)}</strong><span>≈{formatCount(projected.estimatedTokens)} tokens</span></div>
        ) : (
          <div className="context-status"><Gauge size={14} /><span>{healthLabel(health)}</span></div>
        )}
      </div>
      <p className="context-provider-note" title={report.coverage}>{report.message}</p>
    </div>
  )
}

function MetricCard({ icon, value, label, meta, tone }: { icon: React.ReactNode; value: number; label: string; meta: string; tone: string }) {
  return <article className={`metric-card tone-${tone}`}><span className="metric-icon">{icon}</span><div><strong>{value}</strong><span>{label}</span><small>{meta}</small></div></article>
}

function LegendRow({ color, label, value, total }: { color: string; label: string; value: number; total: number }) {
  return <div className="legend-row"><span className={`legend-dot ${color}`} /><span>{label}</span><strong>{value}</strong><small>{percent(value, total)}%</small></div>
}

function ToolBar({ name, counts }: { name: string; counts: Counts }) {
  const total = counts.on + counts.off + counts.conflict + counts.readOnly
  return (
    <div className="tool-bar-row">
      <div><strong>{name}</strong><small>{counts.on} on · {counts.off} off</small></div>
      <div className="stacked-bar" role="img" aria-label={`${name}: ${counts.on} enabled, ${counts.off} disabled`}>
        <span className="bar-on" style={{ width: `${percent(counts.on, total)}%` }} />
        <span className="bar-off" style={{ width: `${percent(counts.off, total)}%` }} />
        <span className="bar-conflict" style={{ width: `${percent(counts.conflict, total)}%` }} />
      </div>
    </div>
  )
}

type Counts = { on: number; off: number; conflict: number; readOnly: number }

function effectiveCounts(snapshot: Snapshot, tool: ManagedTool): Counts {
  const result = { on: 0, off: 0, conflict: 0, readOnly: 0 }
  for (const row of snapshot.rows) {
    const cell: SkillCell | undefined = row[tool]
    if (!cell) continue
    if (cell.effectiveState === 'ON') result.on++
    else if (cell.effectiveState === 'OFF') result.off++
    else if (cell.state === 'CONFLICT') result.conflict++
    else if (cell.state === 'RO') result.readOnly++
  }
  return result
}

function pendingFor(snapshot: Snapshot, tool: string) {
  return snapshot.pending.filter((change) => change.tool === tool).length
}

function percent(value: number, total: number) {
  return total === 0 ? 0 : Math.round(value / total * 100)
}

function portion(value: number, total: number) {
  return total === 0 ? 0 : Math.round(value / total * 360)
}

function clampPercent(value: number) {
  return Math.max(0, Math.min(100, value))
}

function formatPercent(value: number) {
  return `${value.toFixed(1)}%`
}

function formatCount(value: number) {
  return new Intl.NumberFormat('en-US', { maximumFractionDigits: 0 }).format(value)
}

function accuracyLabel(accuracy: string) {
  if (accuracy === 'measured') return 'Measured'
  if (accuracy === 'estimated') return 'Estimated'
  return 'Partial estimate'
}

function healthLabel(health: string) {
  if (health === 'over-budget') return 'Over budget'
  if (health === 'near-limit') return 'Near limit'
  if (health === 'unavailable') return 'Unavailable'
  return 'Within budget'
}
