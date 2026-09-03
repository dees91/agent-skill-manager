import { MANAGED_TOOLS, favoriteEligible, type ActionResult, type ApplyResult, type Backend, type ManagedTool, type PendingChange, type SkillCell, type SkillRow, type Snapshot } from './api'

interface DemoSource extends Record<`${ManagedTool}Count`, number> {
  sourceId: string
  kind: string
  group: string
  location: string
  skillCount: number
  installedAt: string
  commit?: string
  canUpdate: boolean
  updateMode: string
  updateHint: string
}

const seedRows: SkillRow[] = [
  row('release-checklist', 'Prepare a project for a safe release.', 'example-labs/engineering-skills', 'symlink repo', { claude: 'ON', codex: 'ON', muse: 'ON' }),
  row('dependency-review', 'Review dependency and supply-chain changes.', 'example-labs/engineering-skills', 'symlink repo', { claude: 'ON', codex: 'ON', muse: 'OFF' }),
  row('api-contract-audit', 'Check API changes for compatibility risks.', 'example-labs/engineering-skills', 'symlink repo', { claude: 'OFF', codex: 'ON' }),
  row('incident-summary', 'Turn incident notes into a concise report.', 'example-labs/engineering-skills', 'symlink repo', { claude: 'ON', codex: 'OFF' }),
  row('ui-accessibility', 'Audit interface semantics and keyboard access.', 'sample-org/product-skills', 'symlink repo', { claude: 'ON', codex: 'ON' }),
  row('performance-profile', 'Plan and interpret application profiling.', 'sample-org/product-skills', 'symlink repo', { claude: 'ON', codex: 'ON' }),
  row('media-compose', 'Assemble a short product demo from interface captures.', 'sample-org/media-skills', 'symlink repo', { claude: 'OFF', codex: 'OFF' }),
  row('video-encode', 'Encode and optimize video deliverables.', 'sample-org/media-skills', 'symlink repo', { codex: 'OFF' }),
  row('local-notes', 'Maintain a private, link-in-place workflow.', 'local', 'local', { claude: 'ON', muse: 'ON' }),
  row('catalog-search', 'Find reusable skills in a catalog.', 'Skills CLI', 'Skills CLI', { codex: 'ON' }),
  row('decision-review', 'Stress-test a technical decision.', 'Skills CLI', 'Skills CLI', { codex: 'OFF' }),
  readOnlyRow('system-image-tools', 'Generate or edit raster images.', 'Codex system', 'Codex system', 'codex'),
  readOnlyRow('system-docs', 'Consult product documentation.', 'Codex system', 'Codex system', 'codex'),
  readOnlyRow('plugin-runtime', 'Plugin-provided runtime skill.', 'Claude plugin', 'Claude plugin', 'claude'),
]

class DemoBackend implements Backend {
  private rows = seedRows
  private pending: PendingChange[] = []
  private favorites = new Set(['dependency-review', 'media-compose'])
  private diagnosticsMeasured = false
  private skillSets = [
    { setId: 'set:media-demos', name: 'Media demos', description: 'Use when creating an occasional project video.', skills: ['media-compose', 'video-encode'], createdAt: '2026-08-14T09:00:00Z', updatedAt: '2026-08-14T09:00:00Z' },
    { setId: 'set:release-review', name: 'Release review', description: 'Use before publishing a public build.', skills: ['dependency-review', 'release-checklist'], createdAt: '2026-08-13T09:00:00Z', updatedAt: '2026-08-15T09:00:00Z' },
  ]
  private sources: DemoSource[] = [
    { sourceId: 'git:demo', kind: 'git', group: 'example-labs/engineering-skills', location: 'https://github.com/example-labs/engineering-skills', skillCount: 4, claudeCount: 4, codexCount: 4, museCount: 1, grokCount: 0, installedAt: new Date().toISOString(), commit: 'a7c21f93d1b7', canUpdate: true, updateMode: 'Managed Git', updateHint: 'Use Update to fetch changes.' },
    { sourceId: 'local:demo', kind: 'local', group: 'personal-skills', location: '/Users/example/Developer/personal-skills', skillCount: 2, claudeCount: 2, codexCount: 1, museCount: 1, grokCount: 0, installedAt: new Date().toISOString(), canUpdate: false, updateMode: 'Linked folder', updateHint: 'Changes are read directly; no update needed.' },
  ]

  async getSnapshot(includeReadOnly: boolean) { return this.snapshot(includeReadOnly) }

  async toggleCell(skillName: string, tool: string) {
    const row = this.rows.find((candidate) => candidate.name === skillName)
    const cell = row?.[tool as ManagedTool]
    if (!cell || cell.readOnly || cell.conflict) return this.action('Cell cannot be toggled.', 0)
    const key = `${tool}:${skillName}`
    const current = this.pending.find((change) => `${change.tool}:${change.skillName}` === key)
    if (current) {
      this.pending = this.pending.filter((change) => change !== current)
      return this.action('Pending change removed.', 0, 1)
    }
    this.pending = [...this.pending, { tool, skillName, operation: cell.state === 'ON' ? 'disable' : 'enable' } as PendingChange]
    return this.action('Pending change added.', 1)
  }

  async toggleBoth(skillName: string) {
    let last: ActionResult = this.action('Cell cannot be toggled.', 0)
    for (const tool of MANAGED_TOOLS) last = await this.toggleCell(skillName, tool)
    return last
  }

  async toggleGroup(group: string) {
    return this.toggleGroupScope(group, [...MANAGED_TOOLS])
  }

  async toggleGroupScope(group: string, tools: string[]) {
    return this.toggleSkillScope(this.rows.filter((candidate) => candidate.group === group).map((row) => row.name), tools)
  }

  async toggleVisible(skillNames: string[]) {
    return this.toggleSkillScope(skillNames, [...MANAGED_TOOLS])
  }

  async toggleSkillScope(skillNames: string[], tools: string[]) {
    const cells = skillNames.flatMap((name) => {
      const row = this.rows.find((candidate) => candidate.name === name)
      return tools.map((tool) => ({ row, tool, cell: row?.[tool as ManagedTool] }))
    }).filter((item) => item.row && item.cell && !item.cell.readOnly && !item.cell.conflict && ['ON', 'OFF'].includes(item.cell.state)) as Array<{ row: SkillRow; tool: string; cell: SkillCell }>
    const effective = (item: { row: SkillRow; tool: string; cell: SkillCell }) => {
      const pending = this.pending.find((change) => change.skillName === item.row.name && change.tool === item.tool)
      return pending?.operation === 'enable' ? 'ON' : pending?.operation === 'disable' ? 'OFF' : item.cell.state
    }
    const target = cells.every((item) => effective(item) === 'ON') ? 'OFF' : 'ON'
    let changed = 0
    let removed = 0
    for (const item of cells.filter((cell) => effective(cell) === (target === 'ON' ? 'OFF' : 'ON'))) {
      const index = this.pending.findIndex((change) => change.skillName === item.row.name && change.tool === item.tool)
      const desired = item.cell.state === target ? undefined : target === 'ON' ? 'enable' : 'disable'
      if (!desired && index >= 0) {
        this.pending.splice(index, 1)
        removed++
      } else if (desired && (index < 0 || this.pending[index].operation !== desired)) {
        const change = { skillName: item.row.name, tool: item.tool, operation: desired } as PendingChange
        if (index >= 0) this.pending[index] = change
        else this.pending.push(change)
        changed++
      }
    }
    return this.action('Filtered results: pending changes updated.', changed, removed)
  }

  async undoCell(skillName: string, tool: string) {
    this.pending = this.pending.filter((change) => change.skillName !== skillName || change.tool !== tool)
    return this.action('Pending change removed.', 0, 1)
  }

  async clearPending() {
    const removed = this.pending.length
    this.pending = []
    return this.action('All pending changes cleared.', 0, removed)
  }

  async applyPending(includeReadOnly: boolean) {
    for (const change of this.pending) {
      const row = this.rows.find((candidate) => candidate.name === change.skillName)
      const cell = row?.[change.tool as ManagedTool]
      if (cell) cell.state = cell.effectiveState = change.operation === 'enable' ? 'ON' : 'OFF'
    }
    const completed = this.pending.map((change) => ({ ...change }))
    this.pending = []
    return { completed, message: `Applied ${completed.length} change(s).`, snapshot: this.snapshot(includeReadOnly) } as unknown as ApplyResult
  }

  async createSkillSet(name: string, description: string, skillNames: string[]) {
    const now = new Date().toISOString()
    this.skillSets.push({ setId: `set:demo-${this.skillSets.length + 1}`, name: name.trim(), description: description.trim(), skills: [...new Set(skillNames)].sort(), createdAt: now, updatedAt: now })
    this.skillSets.sort((left, right) => left.name.localeCompare(right.name))
    return this.skillSetMutation(`Created Skill Set ${name.trim()}.`)
  }

  async updateSkillSet(setID: string, name: string, description: string, skillNames: string[]) {
    const set = this.skillSets.find((candidate) => candidate.setId === setID)
    if (!set) throw new Error('Skill Set not found')
    set.name = name.trim()
    set.description = description.trim()
    set.skills = [...new Set(skillNames)].sort()
    set.updatedAt = new Date().toISOString()
    this.skillSets.sort((left, right) => left.name.localeCompare(right.name))
    return this.skillSetMutation(`Updated Skill Set ${set.name}.`)
  }

  async deleteSkillSet(setID: string) {
    const set = this.skillSets.find((candidate) => candidate.setId === setID)
    this.skillSets = this.skillSets.filter((candidate) => candidate.setId !== setID)
    return this.skillSetMutation(`Deleted Skill Set ${set?.name ?? ''}. Pending skill changes were not modified.`)
  }

  async previewSkillSetToggle(setID: string, tools: string[]) {
    const set = this.skillSets.find((candidate) => candidate.setId === setID)
    if (!set) throw new Error('Skill Set not found')
    const items = set.skills.flatMap((name) => tools.map((tool) => {
      const skillRow = this.rows.find((candidate) => candidate.name === name)
      const skillCell = skillRow?.[tool as ManagedTool]
      const pending = this.pending.find((change) => change.skillName === name && change.tool === tool)
      const effective = pending?.operation === 'enable' ? 'ON' : pending?.operation === 'disable' ? 'OFF' : skillCell?.state
      return { skillCell, pending, effective }
    }))
    const eligible = items.filter((item) => item.skillCell && !item.skillCell.readOnly && !item.skillCell.conflict && ['ON', 'OFF'].includes(item.skillCell.state))
    const direction = eligible.length === 0 ? 'none' : eligible.every((item) => item.effective === 'ON') ? 'disable' : 'enable'
    const target = direction === 'disable' ? 'ON' : 'OFF'
    const affected = eligible.filter((item) => item.effective === target)
    return { setId: set.setId, name: set.name, tools, direction, eligible: eligible.length, counts: { changed: affected.length, removed: 0, skippedReadOnly: 0, skippedMissing: items.length - eligible.length, skippedConflict: 0 } } as never
  }

  async toggleSkillSet(setID: string, tools: string[]) {
    const set = this.skillSets.find((candidate) => candidate.setId === setID)
    if (!set) throw new Error('Skill Set not found')
    return this.toggleSkillScope(set.skills, tools)
  }

  async setSkillFavorite(skillName: string, favorite: boolean) {
    const skillRow = this.rows.find((candidate) => candidate.name === skillName)
    const eligible = Boolean(skillRow && favoriteEligible(skillRow))
    if (favorite && !eligible) throw new Error(`Skill ${skillName} is not a managed user skill.`)
    if (favorite) this.favorites.add(skillName)
    else this.favorites.delete(skillName)
    return { message: `${favorite ? 'Added' : 'Removed'} ${skillName} ${favorite ? 'to' : 'from'} favorites.`, favorites: [...this.favorites].sort(), warning: '' } as never
  }

  private action(message: string, changed: number, removed = 0) {
    return { message, counts: { changed, removed, skippedReadOnly: 0, skippedMissing: 0, skippedConflict: 0 }, pending: [...this.pending], contextBudgets: demoBudgets(this.pending, this.diagnosticsMeasured), skillSets: this.projectSkillSets(), skillSetsWarning: '' } as unknown as ActionResult
  }

  private snapshot(includeReadOnly: boolean) {
    const visible = this.rows.filter((row) => includeReadOnly || !MANAGED_TOOLS.some((tool) => row[tool]?.readOnly))
    const rows = visible.map((row) => ({ ...projectRow(row, this.pending), favorite: favoriteEligible(row) && this.favorites.has(row.name) } as SkillRow))
    const groups = [...new Set(rows.map((row) => row.group))].map((group) => {
      const grouped = rows.filter((row) => row.group === group)
      return { group, rows: grouped.length, ...Object.fromEntries(MANAGED_TOOLS.map((tool) => [tool, counts(grouped, tool)])), sources: [...new Set(grouped.map((row) => row.source))] }
    })
    return {
      rows,
      skillSets: this.projectSkillSets(),
      skillSetsWarning: '',
      favoritesWarning: '',
      groups,
      sources: [...new Set(rows.map((row) => row.source))].sort(),
      managedSources: [...this.sources],
      stats: {
        managedSkills: rows.filter((row) => MANAGED_TOOLS.every((tool) => !row[tool]?.readOnly)).length,
        readOnlySkills: rows.filter((row) => MANAGED_TOOLS.some((tool) => row[tool]?.readOnly)).length,
        ...Object.fromEntries(MANAGED_TOOLS.map((tool) => [tool, counts(rows, tool)])),
        conflictCells: 0,
      },
      conflicts: [],
      contextBudgets: demoBudgets(this.pending, this.diagnosticsMeasured),
      pending: [...this.pending],
      includeReadOnly,
      scannedAt: new Date().toISOString(),
    } as unknown as Snapshot
  }

  async prepareGitInstall() {
    return { draftId: 'draft:demo', kind: 'git', group: 'example-labs/new-skills', location: 'https://github.com/example-labs/new-skills', candidates: demoCandidates(), cloned: true, reused: false, retainedClone: true, cancelled: false } as never
  }
  async chooseLocalInstall() {
    return { draftId: 'draft:local', kind: 'local', group: 'local-pack', location: '/Users/example/Developer/local-pack', candidates: demoCandidates(), cloned: false, reused: false, retainedClone: false, cancelled: false } as never
  }
  async reviewInstall(draftID: string, selections: never[]) {
    return { reviewId: 'review:demo', draftId: draftID, group: 'example-labs/new-skills', selections, createCount: selections.length, alreadyOnCount: 0, alreadyOffCount: 0, conflicts: [], ready: true } as never
  }
  async applyInstall() { return this.sourceResult('Installed demo source.') }
  async measureContextBudgets() {
    this.diagnosticsMeasured = true
    return this.snapshot(false)
  }
  async updateSource() { return this.sourceResult('Updated 1 source; 0 already up to date.') }
  async updateAllSources() { return this.sourceResult('Updated 1 source; 0 already up to date.') }
  async previewUninstall(sourceID: string) {
    const source = this.sources.find((item) => item.sourceId === sourceID)!
    const affectedFavorites = this.rows.filter((row) => row.group === source.group && this.favorites.has(row.name)).map((row) => row.name).sort()
    return { sourceId: sourceID, kind: source.kind, group: source.group, location: source.location, activeLinks: MANAGED_TOOLS.reduce((total, tool) => total + source[`${tool}Count`], 0), disabledLinks: 0, removesCheckout: source.kind === 'git', preservesSource: source.kind === 'local', affectedSkillSets: [], skillSetImpactWarning: '', affectedFavorites, favoriteImpactWarning: '' } as never
  }
  async uninstallSource(sourceID: string) {
    this.sources = this.sources.filter((source) => source.sourceId !== sourceID)
    return this.sourceResult('Uninstalled demo source.')
  }
  async previewExtend(tool: string) {
    const sources = this.sources.map((source) => {
      const names = this.rows.filter((row) => row.group === source.group).map((row) => row.name)
      return { kind: source.kind, group: source.group, skillNames: names, skillCount: names.length, created: names.length, alreadyInstalled: 0, disabledAfter: 0, status: 'ready', reason: '', skipped: [], conflicts: [] }
    })
    return { tool, sources, createCount: sources.reduce((total, source) => total + source.created, 0), blockedCount: 0 } as never
  }
  async extendSources(tool: string) {
    return this.sourceResult(`Extended ${this.sources.length} source(s) to ${tool}: 0 created, 0 already installed.`)
  }

  private sourceResult(message: string) {
    return { message, completed: [], snapshot: this.snapshot(false) } as never
  }

  private skillSetMutation(message: string) {
    return { message, skillSets: this.projectSkillSets(), warning: '' } as never
  }

  private projectSkillSets() {
    return this.skillSets.map((set) => {
      const members = set.skills.map((name) => {
        const skillRow = this.rows.find((candidate) => candidate.name === name)
        const projected = skillRow ? projectRow(skillRow, this.pending) : undefined
        const cells = Object.fromEntries(MANAGED_TOOLS.map((tool) => [tool, demoSetMemberCell(tool, projected?.[tool], this.pending)])) as Record<ManagedTool, ReturnType<typeof demoSetMemberCell>>
        return { name, description: projected?.description ?? '', group: projected?.group ?? 'unknown', source: projected?.source ?? 'unknown', available: MANAGED_TOOLS.some((tool) => cells[tool].eligible), ...cells }
      })
      const summaries = Object.fromEntries(MANAGED_TOOLS.map((tool) => [tool, demoSetSummary(tool, members.map((member) => member[tool]))])) as Record<ManagedTool, ReturnType<typeof demoSetSummary>>
      return { setId: set.setId, name: set.name, description: set.description, members, ...summaries, unavailable: members.filter((member) => !member.available).length, pending: MANAGED_TOOLS.reduce((total, tool) => total + summaries[tool].pending, 0), createdAt: set.createdAt, updatedAt: set.updatedAt }
    })
  }
}

export const demoBackend: Backend = new DemoBackend()

function row(name: string, description: string, group: string, source: string, states: Partial<Record<ManagedTool, string>>) {
  return { name, description, group, source, ...Object.fromEntries(MANAGED_TOOLS.map((tool) => {
    const state = states[tool]
    return [tool, state ? cell(name, tool, state, group, source) : undefined]
  })) } as unknown as SkillRow
}

function readOnlyRow(name: string, description: string, group: string, source: string, tool: ManagedTool) {
  const entry = cell(name, tool, 'RO', group, source)
  entry.readOnly = true
  return { name, description, group, source, [tool]: entry } as unknown as SkillRow
}

function cell(name: string, tool: string, state: string, group: string, source: string) {
  return { tool, name, displayName: name, description: '', state, effectiveState: state, source, group, entryType: 'symlink', activePath: `/Users/example/.${tool}/skills/${name}`, disabledPath: state === 'OFF' ? `/Users/example/.skill-manager/disabled/${tool}/${name}` : '', skillFilePath: `/Users/example/.${tool}/skills/${name}/SKILL.md`, symlinkTarget: `/Users/example/Developer/agent-skills/${name}`, repoOrigin: '', repoCommit: '', readOnly: false } as unknown as SkillCell
}

function projectRow(row: SkillRow, pending: PendingChange[]) {
  return { ...row, favorite: false, ...Object.fromEntries(MANAGED_TOOLS.map((tool) => [tool, projectCell(row[tool], pending)])) } as SkillRow
}

function projectCell(cell: SkillCell | undefined, pending: PendingChange[]) {
  if (!cell) return undefined
  const operation = pending.find((change) => change.tool === cell.tool && change.skillName === cell.name)?.operation
  return { ...cell, pending: operation, effectiveState: operation === 'disable' ? 'OFF' : operation === 'enable' ? 'ON' : cell.state } as SkillCell
}

function counts(rows: SkillRow[], tool: ManagedTool) {
  const result = { on: 0, off: 0, conflict: 0, readOnly: 0 }
  for (const row of rows) {
    const cell = row[tool]
    if (!cell) continue
    if (cell.readOnly) result.readOnly++
    else if (cell.conflict) result.conflict++
    else if (cell.effectiveState === 'ON') result.on++
    else if (cell.effectiveState === 'OFF') result.off++
  }
  return result
}

function demoCandidates() {
  return ['release-notes', 'test-strategy', 'docs-review'].map((name) => ({
    name,
    relativePath: `skills/${name}`,
    ...Object.fromEntries(MANAGED_TOOLS.map((tool) => [tool, { tool, status: 'available', message: '' }])),
  }))
}

interface DemoBudgetSpec {
  model: string
  tokens: number
  budgetTokens: number
  budgetLabel: string
  accuracy: (measured: boolean) => string
  message: (measured: boolean) => string
}

const DEMO_BUDGET_SPECS: Record<ManagedTool, DemoBudgetSpec> = {
  claude: { model: 'Claude default', tokens: 1680, budgetTokens: 2000, budgetLabel: '1.0% of model context', accuracy: () => 'partial', message: () => 'Filesystem estimate. Run provider diagnostics for model-visible evidence.' },
  codex: { model: 'gpt-5.6-sol', tokens: 1951, budgetTokens: 5440, budgetLabel: '2% of model context', accuracy: (measured) => measured ? 'measured' : 'partial', message: (measured) => measured ? "Measured from Codex's model-visible global catalog." : 'Filesystem estimate. Run provider diagnostics for model-visible evidence.' },
  muse: { model: 'Muse default', tokens: 1680, budgetTokens: 2000, budgetLabel: '1% of assumed 200,000-token context', accuracy: () => 'estimated', message: () => 'Filesystem estimate. Muse exposes no supported catalog diagnostic.' },
  grok: { model: 'Grok default', tokens: 1680, budgetTokens: 2000, budgetLabel: '1% of assumed 200,000-token context', accuracy: () => 'estimated', message: () => 'Filesystem estimate. Grok exposes no supported catalog diagnostic.' },
}

function demoBudgets(pending: PendingChange[], measured: boolean) {
  return Object.fromEntries(MANAGED_TOOLS.map((tool) => {
    const spec = DEMO_BUDGET_SPECS[tool]
    return [tool, demoBudget(tool, spec, pending.filter((change) => change.tool === tool).length, measured)]
  }))
}

function demoSetMemberCell(tool: string, skillCell: SkillCell | undefined, pending: PendingChange[]) {
  const operation = pending.find((change) => change.skillName === skillCell?.name && change.tool === tool)?.operation ?? ''
  const state = skillCell?.state ?? '-'
  return { tool, state, effectiveState: operation === 'enable' ? 'ON' : operation === 'disable' ? 'OFF' : state, pending: operation, eligible: Boolean(skillCell && !skillCell.readOnly && !skillCell.conflict && ['ON', 'OFF'].includes(skillCell.state)), reason: skillCell ? '' : 'Not installed for this tool.' }
}

function demoSetSummary(tool: string, cells: ReturnType<typeof demoSetMemberCell>[]) {
  const eligible = cells.filter((cell) => cell.eligible)
  const on = eligible.filter((cell) => cell.state === 'ON').length
  const off = eligible.filter((cell) => cell.state === 'OFF').length
  const effectiveOn = eligible.filter((cell) => cell.effectiveState === 'ON').length
  const effectiveOff = eligible.filter((cell) => cell.effectiveState === 'OFF').length
  const status = (onCount: number, offCount: number) => eligible.length === 0 ? 'unavailable' : onCount === eligible.length ? 'enabled' : offCount === eligible.length ? 'disabled' : 'mixed'
  return { tool, appliedStatus: status(on, off), effectiveStatus: status(effectiveOn, effectiveOff), eligible: eligible.length, on, off, effectiveOn, effectiveOff, pending: cells.filter((cell) => cell.pending).length, missing: cells.filter((cell) => cell.state === '-').length, readOnly: cells.filter((cell) => cell.state === 'RO').length, conflict: cells.filter((cell) => cell.state === 'CONFLICT').length }
}

function demoBudget(tool: string, spec: DemoBudgetSpec, pendingCount: number, measured: boolean) {
  const projectedTokens = Math.max(0, spec.tokens - pendingCount * 42)
  const usage = (value: number) => ({ skillCount: 9, requestedCharacters: value * 4, renderedCharacters: Math.min(value, spec.budgetTokens) * 4, estimatedTokens: value, renderedTokens: Math.min(value, spec.budgetTokens), usedPercent: Math.round(value / spec.budgetTokens * 1000) / 10, shortenedDescriptions: 0, omittedSkills: 0, health: value > spec.budgetTokens ? 'over-budget' : value / spec.budgetTokens >= .8 ? 'near-limit' : 'ok' })
  return {
    tool,
    model: spec.model,
    contextWindowTokens: tool === 'codex' ? 272000 : 200000,
    contextWindowAssumed: tool !== 'codex',
    budgetFraction: tool === 'codex' ? .02 : .01,
    budgetCharacters: spec.budgetTokens * 4,
    budgetTokens: spec.budgetTokens,
    budgetLabel: spec.budgetLabel,
    accuracy: spec.accuracy(measured),
    coverage: 'Global personal and provider catalogs.',
    message: spec.message(measured),
    current: usage(spec.tokens),
    projected: usage(projectedTokens),
    projectionChanged: pendingCount > 0,
  }
}
