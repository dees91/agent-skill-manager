import { useMemo, useState } from 'react'
import {
  ArrowUpDown,
  BookmarkPlus,
  ChevronDown,
  ChevronsUpDown,
  CircleAlert,
  Eye,
  EyeOff,
  Filter,
  FolderGit2,
  Link2,
  ListChecks,
  Search,
  SlidersHorizontal,
  Star,
  X,
} from 'lucide-react'
import { MANAGED_TOOLS, favoriteEligible, toolDisplayName, toolFullName, type ManagedTool, type SkillCell, type SkillRow, type Snapshot } from '../api'

export type SkillToolScope = 'all' | ManagedTool
type SkillStatusScope = 'all' | 'active' | 'available' | 'favorites'
type Placement = 'attention' | 'active' | 'available' | 'readonly' | 'unavailable'

interface SkillsViewProps {
  snapshot: Snapshot
  selectedRow: SkillRow | null
  busy: boolean
  includeReadOnly: boolean
  expandedGroups: string[]
  onExpandedGroupsChange: (groups: string[]) => void
  onIncludeReadOnly: (show: boolean) => void
  onSelect: (name: string) => void
  onToggleCell: (name: string, tool: string) => void
  onToggleSkillScope: (names: string[], tools: string[]) => void
  onToggleGroupScope: (group: string, tools: string[]) => void
  onSetFavorite: (name: string, favorite: boolean) => void
  onAddToSkillSet: (name: string) => void
  onCloseDetails: () => void
}

export default function SkillsView(props: SkillsViewProps) {
  const { snapshot, selectedRow, busy, includeReadOnly, expandedGroups } = props
  const [query, setQuery] = useState('')
  const [group, setGroup] = useState('all')
  const [source, setSource] = useState('all')
  const [statusScope, setStatusScope] = useState<SkillStatusScope>('all')
  const [toolScope, setToolScope] = useState<SkillToolScope>('all')
  const [filtersOpen, setFiltersOpen] = useState(false)
  const tools = toolsForScope(toolScope)

  const matchingRows = useMemo(() => {
    const normalized = query.trim().toLowerCase()
    return snapshot.rows
      .filter((row) => {
        if (group !== 'all' && row.group !== group) return false
        if (source !== 'all' && !rowSources(row).includes(source)) return false
        if (toolScope !== 'all' && !cellForTool(row, toolScope)) return false
        if (!normalized) return true
        return [row.name, row.description, row.group, row.source, ...rowSources(row)]
          .filter(Boolean)
          .some((value) => value.toLowerCase().includes(normalized))
      })
      .sort(compareRows)
  }, [group, query, snapshot.rows, source, toolScope])

  const favoriteRows = matchingRows.filter((row) => row.favorite)
  const scopedRows = statusScope === 'favorites' ? favoriteRows : matchingRows
  const attentionRows = scopedRows.filter((row) => placementFor(row, tools) === 'attention')
  const activeRows = scopedRows.filter((row) => placementFor(row, tools) === 'active')
  const availableRows = scopedRows.filter((row) => placementFor(row, tools) === 'available')
  const readOnlyRows = scopedRows.filter((row) => placementFor(row, tools) === 'readonly')
  const showActive = statusScope !== 'available'
  const showAvailable = statusScope !== 'active'
  const resultRows = scopedRows.filter((row) => {
    const placement = placementFor(row, tools)
    if (placement === 'attention') return true
    if (statusScope === 'active') return placement === 'active'
    if (statusScope === 'available') return placement === 'available'
    return placement !== 'unavailable'
  })
  const targetCount = toggleableCellCount(resultRows, tools)
  const visibleCount = attentionRows.length + (showActive ? activeRows.length : 0) + (showAvailable ? availableRows.length : 0) + (statusScope === 'all' ? readOnlyRows.length : 0)
  const advancedFilterCount = Number(group !== 'all') + Number(source !== 'all') + Number(includeReadOnly)
  const filtersActive = Boolean(query) || group !== 'all' || source !== 'all' || statusScope !== 'all' || toolScope !== 'all' || includeReadOnly

  const resetFilters = () => {
    setQuery('')
    setGroup('all')
    setSource('all')
    setStatusScope('all')
    setToolScope('all')
    if (includeReadOnly) props.onIncludeReadOnly(false)
  }

  const toggleExpanded = (groupName: string) => {
    const current = new Set(expandedGroups)
    if (current.has(groupName)) current.delete(groupName)
    else current.add(groupName)
    props.onExpandedGroupsChange([...current].sort())
  }

  return (
    <section className="page skills-page" aria-labelledby="skills-title">
      <div className="page-heading compact skills-heading">
        <div><p className="eyebrow">Managed filesystem entries</p><h1 id="skills-title">Skills</h1><p>Keep active skills close and open source packs only when you need them.</p></div>
        <button
          className="secondary-button bulk-button"
          onClick={() => props.onToggleSkillScope(resultRows.map((row) => row.name), tools)}
          disabled={busy || targetCount === 0}
          aria-label={`Smart-toggle filtered results: ${targetCount} eligible ${targetCount === 1 ? 'cell' : 'cells'}`}
        >
          <ListChecks size={16} /> Smart-toggle results <span className="bulk-count">{targetCount}</span>
        </button>
      </div>

      <div className="skills-filter-panel">
        <div className="skills-filter-primary">
          <label className="search-field" htmlFor="skill-search"><Search size={16} /><input id="skill-search" aria-label="Search name, description, group, or source" value={query} onChange={(event) => setQuery(event.target.value)} placeholder="Search skills or sources…" /><kbd>⌘F</kbd></label>
          <SegmentedFilter label="Skill availability" value={statusScope} options={[["all", `All ${matchingRows.length}`], ["active", `Active ${matchingRows.filter((row) => placementFor(row, tools) === 'active').length}`], ["available", `Available ${matchingRows.filter((row) => placementFor(row, tools) === 'available').length}`], ["favorites", `Favorites ${favoriteRows.length}`]]} onChange={(value) => setStatusScope(value as SkillStatusScope)} />
          <SegmentedFilter label="Tool scope" value={toolScope} options={[["all", "All tools"], ...MANAGED_TOOLS.map((tool) => [tool, toolDisplayName(tool)] as [string, string])]} onChange={(value) => setToolScope(value as SkillToolScope)} />
          <button className={`secondary-button filters-button ${filtersOpen ? 'active' : ''}`} aria-expanded={filtersOpen} aria-controls="advanced-skill-filters" onClick={() => setFiltersOpen((open) => !open)}>
            <Filter size={14} /> Filters {advancedFilterCount > 0 && <span>{advancedFilterCount}</span>}<ChevronDown size={13} />
          </button>
        </div>
        {filtersOpen && (
          <div className="skills-filter-advanced" id="advanced-skill-filters">
            <SelectFilter label="Group" value={group} onChange={setGroup} options={[["all", "All groups"], ...snapshot.groups.map((item) => [item.group, item.group] as [string, string])]} />
            <SelectFilter label="Source" value={source} onChange={setSource} options={[["all", "All sources"], ...snapshot.sources.map((item) => [item, item] as [string, string])]} />
            <button className={`read-only-switch ${includeReadOnly ? 'on' : ''}`} role="switch" aria-checked={includeReadOnly} onClick={() => props.onIncludeReadOnly(!includeReadOnly)} disabled={busy}>
              {includeReadOnly ? <Eye size={15} /> : <EyeOff size={15} />}<span>Read only</span><i />
            </button>
            {filtersActive && <button className="ghost-button clear-all-filters" onClick={resetFilters}><X size={13} /> Clear filters</button>}
          </div>
        )}
      </div>

      {snapshot.favoritesWarning && <div className="favorites-warning" role="alert"><CircleAlert size={17} /><div><strong>Favorites are unavailable</strong><p>{snapshot.favoritesWarning}</p><small>Normal skill toggles and Apply remain available. Repair or restore favorites.json, then refresh.</small></div></div>}

      <div className="skills-results-summary">
        <span><strong>{visibleCount}</strong> of {snapshot.rows.length} skills shown</span>
        <span>Bulk scope: <strong>{toolScope === 'all' ? MANAGED_TOOLS.map(toolDisplayName).join(' + ') : toolDisplayName(toolScope)}</strong></span>
      </div>

      <div className="skills-workspace">
        {attentionRows.length > 0 && (
          <WorkspaceSection title="Needs attention" count={attentionRows.length} tone="attention" description="Resolve blockers before these skills can be restored.">
            <SkillsTable rows={attentionRows} selectedRow={selectedRow} busy={busy} tools={tools} showGroup favoritesUnavailable={Boolean(snapshot.favoritesWarning)} onSelect={props.onSelect} onToggleCell={props.onToggleCell} onToggleSkillScope={props.onToggleSkillScope} onSetFavorite={props.onSetFavorite} />
          </WorkspaceSection>
        )}

        {showActive && activeRows.length > 0 && (
          <WorkspaceSection title="Active now" count={activeRows.length} tone="active" description="Immediately visible to the selected tool scope.">
            <SkillsTable rows={activeRows} selectedRow={selectedRow} busy={busy} tools={tools} showGroup favoritesUnavailable={Boolean(snapshot.favoritesWarning)} onSelect={props.onSelect} onToggleCell={props.onToggleCell} onToggleSkillScope={props.onToggleSkillScope} onSetFavorite={props.onSetFavorite} />
          </WorkspaceSection>
        )}

        {showAvailable && availableRows.length > 0 && (
          <WorkspaceSection title="Available by source" count={availableRows.length} description="Open a source pack when a task needs one of its skills.">
            <GroupedSkills
              rows={availableRows}
              allRows={snapshot.rows}
              selectedRow={selectedRow}
              busy={busy}
              tools={tools}
              forceExpanded={Boolean(query.trim()) || statusScope === 'favorites'}
              expandedGroups={expandedGroups}
              onToggleExpanded={toggleExpanded}
              onSelect={props.onSelect}
              onToggleCell={props.onToggleCell}
              onToggleSkillScope={props.onToggleSkillScope}
              onToggleGroupScope={props.onToggleGroupScope}
              onSetFavorite={props.onSetFavorite}
              favoritesUnavailable={Boolean(snapshot.favoritesWarning)}
            />
          </WorkspaceSection>
        )}

        {statusScope === 'all' && readOnlyRows.length > 0 && (
          <WorkspaceSection title="Read only" count={readOnlyRows.length} description="System and plugin skills are visible here but cannot be changed.">
            <GroupedSkills
              rows={readOnlyRows}
              allRows={snapshot.rows}
              selectedRow={selectedRow}
              busy={busy}
              tools={tools}
              forceExpanded={Boolean(query.trim())}
              expandedGroups={expandedGroups}
              onToggleExpanded={toggleExpanded}
              onSelect={props.onSelect}
              onToggleCell={props.onToggleCell}
              onToggleSkillScope={props.onToggleSkillScope}
              onToggleGroupScope={props.onToggleGroupScope}
              onSetFavorite={props.onSetFavorite}
              favoritesUnavailable={Boolean(snapshot.favoritesWarning)}
              readOnly
            />
          </WorkspaceSection>
        )}

        {visibleCount === 0 && (
          <div className="empty-table skills-empty">{statusScope === 'favorites' ? <Star size={24} /> : <Filter size={24} />}<strong>{statusScope === 'favorites' ? 'No favorite skills found' : 'No matching skills'}</strong><p>{statusScope === 'favorites' ? 'Star a managed skill or adjust the current filters.' : 'Adjust the search or clear the active filters.'}</p><button className="secondary-button" onClick={resetFilters}>{statusScope === 'favorites' ? 'Show all skills' : 'Clear filters'}</button></div>
        )}
      </div>

      {selectedRow && <DetailsDrawer row={selectedRow} onClose={props.onCloseDetails} onToggleGroup={(groupName) => props.onToggleGroupScope(groupName, tools)} onAddToSkillSet={props.onAddToSkillSet} onSetFavorite={props.onSetFavorite} favoritesUnavailable={Boolean(snapshot.favoritesWarning)} busy={busy} toolScope={toolScope} />}
    </section>
  )
}

function WorkspaceSection({ title, count, description, tone = 'default', children }: { title: string; count: number; description: string; tone?: 'default' | 'active' | 'attention'; children: React.ReactNode }) {
  return (
    <section className={`workspace-section workspace-${tone}`} aria-labelledby={`section-${slug(title)}`}>
      <header className="workspace-section-header"><div><h2 id={`section-${slug(title)}`}>{title}</h2><span>{count}</span></div><p>{description}</p></header>
      {children}
    </section>
  )
}

function GroupedSkills(props: {
  rows: SkillRow[]
  allRows: SkillRow[]
  selectedRow: SkillRow | null
  busy: boolean
  tools: ManagedTool[]
  forceExpanded: boolean
  expandedGroups: string[]
  readOnly?: boolean
  onToggleExpanded: (group: string) => void
  onSelect: (name: string) => void
  onToggleCell: (name: string, tool: string) => void
  onToggleSkillScope: (names: string[], tools: string[]) => void
  onToggleGroupScope: (group: string, tools: string[]) => void
  onSetFavorite: (name: string, favorite: boolean) => void
  favoritesUnavailable: boolean
}) {
  const groups = groupRows(props.rows)
  return <div className="skill-groups">{groups.map(([groupName, rows]) => {
    const allGroupRows = props.allRows.filter((row) => row.group === groupName)
    const metrics = groupMetrics(allGroupRows, props.tools)
    const expanded = props.forceExpanded || props.expandedGroups.includes(groupName)
    const panelID = `skill-group-${props.readOnly ? 'readonly-' : ''}${slug(groupName)}`
    const sourceText = unique(allGroupRows.flatMap(rowSources)).join(', ') || 'unknown'
    return (
      <section className={`skill-group ${expanded ? 'expanded' : ''}`} key={groupName}>
        <header className="skill-group-header">
          <button className="skill-group-toggle" aria-label={`${groupName}, ${sourceText}, ${metrics.active} active, ${metrics.available} available${metrics.pending ? `, ${metrics.pending} pending` : ''}`} aria-expanded={expanded} aria-controls={panelID} aria-disabled={props.forceExpanded} title={props.forceExpanded ? 'Filtered results expand matching groups temporarily.' : undefined} onClick={() => { if (!props.forceExpanded) props.onToggleExpanded(groupName) }}>
            <ChevronDown size={15} />
            <span className="group-icon"><FolderGit2 size={14} /></span>
            <span className="skill-group-identity"><strong>{groupName}</strong><small>{sourceText}</small></span>
            <span className="skill-group-metrics"><b>{metrics.active} active</b><b>{metrics.available} available</b>{metrics.pending > 0 && <b className="pending-metric">{metrics.pending} pending</b>}</span>
          </button>
          {!props.readOnly && (
            <button
              className="secondary-button group-toggle-action"
              onClick={() => props.onToggleGroupScope(groupName, props.tools)}
              disabled={props.busy || metrics.targets === 0}
              aria-label={`Smart-toggle entire group ${groupName}: ${metrics.targets} eligible ${metrics.targets === 1 ? 'cell' : 'cells'}`}
            ><ArrowUpDown size={14} /> Toggle group <span>{metrics.targets}</span></button>
          )}
        </header>
        <div id={panelID} hidden={!expanded}>{expanded && <SkillsTable rows={rows} selectedRow={props.selectedRow} busy={props.busy} tools={props.tools} favoritesUnavailable={props.favoritesUnavailable} onSelect={props.onSelect} onToggleCell={props.onToggleCell} onToggleSkillScope={props.onToggleSkillScope} onSetFavorite={props.onSetFavorite} />}</div>
      </section>
    )
  })}</div>
}

function SkillsTable(props: {
  rows: SkillRow[]
  selectedRow: SkillRow | null
  busy: boolean
  tools: ManagedTool[]
  showGroup?: boolean
  favoritesUnavailable: boolean
  onSelect: (name: string) => void
  onToggleCell: (name: string, tool: string) => void
  onToggleSkillScope: (names: string[], tools: string[]) => void
  onSetFavorite: (name: string, favorite: boolean) => void
}) {
  return (
    <div className="skills-table-wrap">
      <table className="skills-table">
        <thead><tr><th>Skill</th>{MANAGED_TOOLS.map((tool) => <th key={tool}>{toolDisplayName(tool)}</th>)}<th><span className="sr-only">Actions</span></th></tr></thead>
        <tbody>{props.rows.map((row) => {
          const targets = toggleableCellCount([row], props.tools)
          return (
            <tr key={row.name} className={props.selectedRow?.name === row.name ? 'selected' : ''} tabIndex={0} onClick={() => props.onSelect(row.name)} onKeyDown={(event) => { if (event.target === event.currentTarget && (event.key === 'Enter' || event.key === ' ')) { event.preventDefault(); props.onSelect(row.name) } }}>
              <td><div className="skill-name"><span className="skill-glyph">{initials(row.name)}</span><div><strong>{row.name}</strong><small>{row.description || 'No description in SKILL.md'}</small>{props.showGroup && <span className="active-group-label"><FolderGit2 size={11} />{row.group}<i>{row.source}</i></span>}</div></div></td>
              {MANAGED_TOOLS.map((tool) => <td key={tool}><ToolCell cell={row[tool]} rowName={row.name} busy={props.busy} onToggle={props.onToggleCell} /></td>)}
              <td><div className="row-actions">{favoriteEligible(row) && <button className={`icon-button subtle favorite-button ${row.favorite ? 'active' : ''}`} title={row.favorite ? 'Remove from favorites' : 'Add to favorites'} aria-label={`${row.favorite ? 'Remove' : 'Add'} ${row.name} ${row.favorite ? 'from' : 'to'} favorites`} aria-pressed={row.favorite} onClick={(event) => { event.stopPropagation(); props.onSetFavorite(row.name, !row.favorite) }} disabled={props.busy || props.favoritesUnavailable}><Star size={15} fill={row.favorite ? 'currentColor' : 'none'} /></button>}<button className="icon-button subtle" title="Smart-toggle this row in the selected tool scope" aria-label={`Smart-toggle ${row.name}: ${targets} eligible ${targets === 1 ? 'cell' : 'cells'}`} onClick={(event) => { event.stopPropagation(); props.onToggleSkillScope([row.name], props.tools) }} disabled={props.busy || targets === 0}><SlidersHorizontal size={15} /></button></div></td>
            </tr>
          )
        })}</tbody>
      </table>
    </div>
  )
}

function ToolCell({ cell, rowName, busy, onToggle }: { cell?: SkillCell; rowName: string; busy: boolean; onToggle: (name: string, tool: string) => void }) {
  if (!cell) return <span className="missing-state">—</span>
  const state = cell.conflict ? 'CONFLICT' : cell.readOnly ? 'RO' : cell.effectiveState
  const disabled = busy || cell.readOnly || Boolean(cell.conflict) || !['ON', 'OFF'].includes(cell.state)
  return (
    <button
      className={`state-control state-${state.toLowerCase()} ${cell.pending ? 'pending' : ''}`}
      disabled={disabled}
      onClick={(event) => { event.stopPropagation(); onToggle(rowName, cell.tool) }}
      aria-label={`${cell.tool} ${rowName}: ${state}${cell.pending ? `, pending ${cell.pending}` : ''}`}
      title={cell.conflict?.message || (cell.readOnly ? 'Read-only source' : `Stage ${cell.effectiveState === 'ON' ? 'disable' : 'enable'}`)}
    >
      <span className="state-dot" /><b>{state}</b>{cell.pending && <small>{cell.state} → {cell.effectiveState}</small>}
    </button>
  )
}

function SegmentedFilter({ label, value, options, onChange }: { label: string; value: string; options: [string, string][]; onChange: (value: string) => void }) {
  return <div className="segmented-filter" role="group" aria-label={label}>{options.map(([key, text]) => <button key={key} aria-pressed={value === key} className={value === key ? 'active' : ''} onClick={() => onChange(key)}>{text}</button>)}</div>
}

function SelectFilter({ label, value, onChange, options }: { label: string; value: string; onChange: (value: string) => void; options: [string, string][] }) {
  return <label className="select-filter"><span className="sr-only">{label}</span><select aria-label={label} value={value} onChange={(event) => onChange(event.target.value)}>{options.map(([key, text]) => <option value={key} key={key}>{text}</option>)}</select><ChevronsUpDown size={13} /></label>
}

function DetailsDrawer({ row, onClose, onToggleGroup, onAddToSkillSet, onSetFavorite, favoritesUnavailable, busy, toolScope }: { row: SkillRow; onClose: () => void; onToggleGroup: (group: string) => void; onAddToSkillSet: (name: string) => void; onSetFavorite: (name: string, favorite: boolean) => void; favoritesUnavailable: boolean; busy: boolean; toolScope: SkillToolScope }) {
  const canSaveToSet = MANAGED_TOOLS.some((tool) => {
    const cell = row[tool]
    return Boolean(cell && !cell.readOnly && ['ON', 'OFF'].includes(cell.state))
  })
  return (
    <aside className="details-drawer" aria-label={`Details for ${row.name}`}>
      <div className="drawer-header"><div><p className="eyebrow">Skill details</p><h2>{row.name}</h2></div><button className="icon-button" onClick={onClose} aria-label="Close skill details"><X size={17} /></button></div>
      <p className="details-description">{row.description || 'No description was parsed from SKILL.md.'}</p>
      <dl className="details-summary"><div><dt>Group</dt><dd>{row.group}</dd></div><div><dt>Source</dt><dd>{row.source}</dd></div></dl>
      {favoriteEligible(row) && <button className={`secondary-button details-group-action details-favorite-action ${row.favorite ? 'active' : ''}`} onClick={() => onSetFavorite(row.name, !row.favorite)} disabled={busy || favoritesUnavailable}><Star size={15} fill={row.favorite ? 'currentColor' : 'none'} /> {row.favorite ? 'Remove from favorites' : 'Add to favorites'}</button>}
      <button className="secondary-button details-group-action" onClick={() => onAddToSkillSet(row.name)} disabled={busy || !canSaveToSet} title={canSaveToSet ? undefined : 'Only toggleable user skills can be added'}><BookmarkPlus size={15} /> Add to Skill Set…</button>
      <button className="secondary-button details-group-action" onClick={() => onToggleGroup(row.group)} disabled={busy}><ArrowUpDown size={15} /> Smart-toggle group · {toolScope === 'all' ? 'All tools' : toolDisplayName(toolScope)}</button>
      <div className="detail-cells">
        {MANAGED_TOOLS.map((tool) => {
          const cell = row[tool]
          return cell ? <CellDetails key={tool} title={toolFullName(tool)} cell={cell} /> : null
        })}
      </div>
    </aside>
  )
}

function CellDetails({ title, cell }: { title: string; cell: SkillCell }) {
  return (
    <section className="cell-details">
      <div className="cell-details-title"><strong>{title}</strong><span className={`mini-state state-${(cell.conflict ? 'conflict' : cell.readOnly ? 'ro' : cell.effectiveState).toLowerCase()}`}>{cell.conflict ? 'CONFLICT' : cell.readOnly ? 'RO' : cell.effectiveState}</span></div>
      <Detail label="Source" value={cell.source} />
      <Detail label="Entry type" value={cell.entryType || 'unknown'} />
      <Detail label="Active path" value={cell.activePath} mono />
      {cell.disabledPath && <Detail label="Disabled path" value={cell.disabledPath} mono />}
      {cell.symlinkTarget && <Detail label="Symlink target" value={cell.symlinkTarget} mono icon />}
      {cell.repoOrigin && <Detail label="Repository" value={cell.repoOrigin} mono />}
      {cell.repoCommit && <Detail label="Commit" value={cell.repoCommit.slice(0, 12)} mono />}
      {cell.conflict && <div className="detail-conflict"><CircleAlert size={16} /><div><strong>Restore blocked</strong><p>{cell.conflict.message}</p><code>{cell.conflict.blockerPath}</code></div></div>}
    </section>
  )
}

function Detail({ label, value, mono = false, icon = false }: { label: string; value: string; mono?: boolean; icon?: boolean }) {
  return <div className="detail-line"><span>{label}</span><p className={mono ? 'mono' : ''}>{icon && <Link2 size={12} />}{value || '—'}</p></div>
}

function placementFor(row: SkillRow, tools: ManagedTool[]): Placement {
  const cells = tools.map((tool) => cellForTool(row, tool)).filter((cell): cell is SkillCell => Boolean(cell))
  if (cells.some((cell) => Boolean(cell.conflict) || cell.state === 'CONFLICT')) return 'attention'
  if (cells.some((cell) => !cell.readOnly && cell.state === 'ON')) return 'active'
  if (cells.some((cell) => !cell.readOnly && cell.state === 'OFF')) return 'available'
  if (cells.some((cell) => cell.readOnly || cell.state === 'RO')) return 'readonly'
  return 'unavailable'
}

function groupMetrics(rows: SkillRow[], tools: ManagedTool[]) {
  return {
    active: rows.filter((row) => placementFor(row, tools) === 'active').length,
    available: rows.filter((row) => placementFor(row, tools) === 'available').length,
    pending: rows.reduce((total, row) => total + tools.filter((tool) => Boolean(cellForTool(row, tool)?.pending)).length, 0),
    targets: toggleableCellCount(rows, tools),
  }
}

function toggleableCellCount(rows: SkillRow[], tools: ManagedTool[]) {
  return rows.reduce((total, row) => total + tools.filter((tool) => {
    const cell = cellForTool(row, tool)
    return Boolean(cell && !cell.readOnly && !cell.conflict && ['ON', 'OFF'].includes(cell.state))
  }).length, 0)
}

function toolsForScope(scope: SkillToolScope): ManagedTool[] {
  return scope === 'all' ? [...MANAGED_TOOLS] : [scope]
}

function cellForTool(row: SkillRow, tool: ManagedTool) {
  return row[tool]
}

function rowSources(row: SkillRow) {
  return unique([row.source, ...MANAGED_TOOLS.map((tool) => row[tool]?.source)].filter((value): value is string => Boolean(value)))
}

function groupRows(rows: SkillRow[]): [string, SkillRow[]][] {
  const grouped = new Map<string, SkillRow[]>()
  rows.forEach((row) => grouped.set(row.group, [...(grouped.get(row.group) ?? []), row]))
  return [...grouped.entries()].sort(([leftName, leftRows], [rightName, rightRows]) => Number(rightRows.some((row) => row.favorite)) - Number(leftRows.some((row) => row.favorite)) || leftName.localeCompare(rightName)).map(([name, items]) => [name, items.sort(compareRows)])
}

function compareRows(left: SkillRow, right: SkillRow) {
  const byFavorite = Number(right.favorite) - Number(left.favorite)
  if (byFavorite) return byFavorite
  const byGroup = left.group.localeCompare(right.group)
  return byGroup || left.name.localeCompare(right.name)
}

function unique(values: string[]) {
  return [...new Set(values)]
}

function initials(name: string) {
  const parts = name.split(/[-_\s]+/).filter(Boolean)
  return (parts.length > 1 ? parts.slice(0, 2).map((part) => part[0]).join('') : name.slice(0, 2)).toUpperCase()
}

function slug(value: string) {
  return value.toLowerCase().replace(/[^a-z0-9]+/g, '-').replace(/^-|-$/g, '')
}

function titleCase(value: string) {
  return value.charAt(0).toUpperCase() + value.slice(1)
}
