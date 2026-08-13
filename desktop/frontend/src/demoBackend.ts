import type { ActionResult, ApplyResult, Backend, PendingChange, SkillCell, SkillRow, Snapshot } from './api'

const seedRows: SkillRow[] = [
  row('release-checklist', 'Prepare a project for a safe release.', 'example-labs/engineering-skills', 'symlink repo', 'ON', 'ON'),
  row('dependency-review', 'Review dependency and supply-chain changes.', 'example-labs/engineering-skills', 'symlink repo', 'ON', 'ON'),
  row('api-contract-audit', 'Check API changes for compatibility risks.', 'example-labs/engineering-skills', 'symlink repo', 'OFF', 'ON'),
  row('incident-summary', 'Turn incident notes into a concise report.', 'example-labs/engineering-skills', 'symlink repo', 'ON', 'OFF'),
  row('ui-accessibility', 'Audit interface semantics and keyboard access.', 'sample-org/product-skills', 'symlink repo', 'ON', 'ON'),
  row('performance-profile', 'Plan and interpret application profiling.', 'sample-org/product-skills', 'symlink repo', 'ON', 'ON'),
  row('local-notes', 'Maintain a private, link-in-place workflow.', 'local', 'local', 'ON', undefined),
  row('catalog-search', 'Find reusable skills in a catalog.', 'Skills CLI', 'Skills CLI', undefined, 'ON'),
  row('decision-review', 'Stress-test a technical decision.', 'Skills CLI', 'Skills CLI', undefined, 'OFF'),
  readOnlyRow('system-image-tools', 'Generate or edit raster images.', 'Codex system', 'Codex system', 'codex'),
  readOnlyRow('system-docs', 'Consult product documentation.', 'Codex system', 'Codex system', 'codex'),
  readOnlyRow('plugin-runtime', 'Plugin-provided runtime skill.', 'Claude plugin', 'Claude plugin', 'claude'),
]

class DemoBackend implements Backend {
  private rows = seedRows
  private pending: PendingChange[] = []
  private sources = [
    { sourceId: 'git:demo', kind: 'git', group: 'example-labs/engineering-skills', location: 'https://github.com/example-labs/engineering-skills', skillCount: 4, claudeCount: 4, codexCount: 4, installedAt: new Date().toISOString(), commit: 'a7c21f93d1b7', canUpdate: true, updateMode: 'Managed Git', updateHint: 'Use Update to fetch changes.' },
    { sourceId: 'local:demo', kind: 'local', group: 'personal-skills', location: '/Users/example/Developer/personal-skills', skillCount: 2, claudeCount: 2, codexCount: 1, installedAt: new Date().toISOString(), canUpdate: false, updateMode: 'Linked folder', updateHint: 'Changes are read directly; no update needed.' },
  ]

  async getSnapshot(includeReadOnly: boolean) { return this.snapshot(includeReadOnly) }

  async toggleCell(skillName: string, tool: string) {
    const row = this.rows.find((candidate) => candidate.name === skillName)
    const cell = row?.[tool as 'claude' | 'codex']
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
    await this.toggleCell(skillName, 'claude')
    return this.toggleCell(skillName, 'codex')
  }

  async toggleGroup(group: string) {
    return this.toggleGroupScope(group, ['claude', 'codex'])
  }

  async toggleGroupScope(group: string, tools: string[]) {
    return this.toggleSkillScope(this.rows.filter((candidate) => candidate.group === group).map((row) => row.name), tools)
  }

  async toggleVisible(skillNames: string[]) {
    return this.toggleSkillScope(skillNames, ['claude', 'codex'])
  }

  async toggleSkillScope(skillNames: string[], tools: string[]) {
    const cells = skillNames.flatMap((name) => {
      const row = this.rows.find((candidate) => candidate.name === name)
      return tools.map((tool) => ({ row, tool, cell: row?.[tool as 'claude' | 'codex'] }))
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
      const cell = row?.[change.tool as 'claude' | 'codex']
      if (cell) cell.state = cell.effectiveState = change.operation === 'enable' ? 'ON' : 'OFF'
    }
    const completed = this.pending.map((change) => ({ ...change }))
    this.pending = []
    return { completed, message: `Applied ${completed.length} change(s).`, snapshot: this.snapshot(includeReadOnly) } as unknown as ApplyResult
  }

  private action(message: string, changed: number, removed = 0) {
    return { message, counts: { changed, removed, skippedReadOnly: 0, skippedMissing: 0, skippedConflict: 0 }, pending: [...this.pending], contextBudgets: demoBudgets(this.pending) } as unknown as ActionResult
  }

  private snapshot(includeReadOnly: boolean) {
    const visible = this.rows.filter((row) => includeReadOnly || ![row.claude, row.codex].some((cell) => cell?.readOnly))
    const rows = visible.map((row) => projectRow(row, this.pending))
    const groups = [...new Set(rows.map((row) => row.group))].map((group) => {
      const grouped = rows.filter((row) => row.group === group)
      return { group, rows: grouped.length, claude: counts(grouped, 'claude'), codex: counts(grouped, 'codex'), sources: [...new Set(grouped.map((row) => row.source))] }
    })
    return {
      rows,
      groups,
      sources: [...new Set(rows.map((row) => row.source))].sort(),
      managedSources: [...this.sources],
      stats: {
        managedSkills: rows.filter((row) => !row.claude?.readOnly && !row.codex?.readOnly).length,
        readOnlySkills: rows.filter((row) => row.claude?.readOnly || row.codex?.readOnly).length,
        claude: counts(rows, 'claude'),
        codex: counts(rows, 'codex'),
        conflictCells: 0,
      },
      conflicts: [],
      contextBudgets: demoBudgets(this.pending),
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
  async getDiscoverPage(view: string) { return demoDiscoverPage(view) as never }
  async searchDiscover() { return demoDiscoverPage('search') as never }
  async getDiscoverSkill(skillID: string) { const skill = demoDiscoverSkills().find((item) => item.id === skillID)!; return { skill, description: 'A catalog skill used to demonstrate safe skills.sh discovery.', fetchedAt: new Date().toISOString(), offline: false, fromCache: false, auditStatus: 'external-only' } as never }
  async installDiscoverSkill() { return this.sourceResult('Installed catalog skill.') }
  async updateSource() { return this.sourceResult('Updated 1 source; 0 already up to date.') }
  async updateAllSources() { return this.sourceResult('Updated 1 source; 0 already up to date.') }
  async previewUninstall(sourceID: string) {
    const source = this.sources.find((item) => item.sourceId === sourceID)!
    return { sourceId: sourceID, kind: source.kind, group: source.group, location: source.location, activeLinks: source.claudeCount + source.codexCount, disabledLinks: 0, removesCheckout: source.kind === 'git', preservesSource: source.kind === 'local' } as never
  }
  async uninstallSource(sourceID: string) {
    this.sources = this.sources.filter((source) => source.sourceId !== sourceID)
    return this.sourceResult('Uninstalled demo source.')
  }

  private sourceResult(message: string) {
    return { message, completed: [], snapshot: this.snapshot(false) } as never
  }
}

export const demoBackend: Backend = new DemoBackend()

function row(name: string, description: string, group: string, source: string, claude?: string, codex?: string) {
  return { name, description, group, source, claude: claude ? cell(name, 'claude', claude, group, source) : undefined, codex: codex ? cell(name, 'codex', codex, group, source) : undefined } as unknown as SkillRow
}

function readOnlyRow(name: string, description: string, group: string, source: string, tool: 'claude' | 'codex') {
  const entry = cell(name, tool, 'RO', group, source)
  entry.readOnly = true
  return { name, description, group, source, [tool]: entry } as unknown as SkillRow
}

function cell(name: string, tool: string, state: string, group: string, source: string) {
  return { tool, name, displayName: name, description: '', state, effectiveState: state, source, group, entryType: 'symlink', activePath: `/Users/example/.${tool}/skills/${name}`, disabledPath: state === 'OFF' ? `/Users/example/.skill-manager/disabled/${tool}/${name}` : '', skillFilePath: `/Users/example/.${tool}/skills/${name}/SKILL.md`, symlinkTarget: `/Users/example/Developer/agent-skills/${name}`, repoOrigin: '', repoCommit: '', readOnly: false } as unknown as SkillCell
}

function projectRow(row: SkillRow, pending: PendingChange[]) {
  return { ...row, claude: projectCell(row.claude, pending), codex: projectCell(row.codex, pending) } as SkillRow
}

function projectCell(cell: SkillCell | undefined, pending: PendingChange[]) {
  if (!cell) return undefined
  const operation = pending.find((change) => change.tool === cell.tool && change.skillName === cell.name)?.operation
  return { ...cell, pending: operation, effectiveState: operation === 'disable' ? 'OFF' : operation === 'enable' ? 'ON' : cell.state } as SkillCell
}

function counts(rows: SkillRow[], tool: 'claude' | 'codex') {
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
    claude: { tool: 'claude', status: 'available', message: '' },
    codex: { tool: 'codex', status: 'available', message: '' },
  }))
}

function demoDiscoverSkills() {
  return [
    { id: 'example-labs/skills/release-notes', skillId: 'release-notes', name: 'release-notes', source: 'example-labs/skills', installs: 2919799, weeklyInstalls: [120, 140, 135, 170, 210, 230, 260, 290], sourceType: 'github', url: 'https://skills.sh/example-labs/skills/release-notes', installable: true, claude: { tool: 'claude', status: 'available' }, codex: { tool: 'codex', status: 'conflict', message: 'Installed by another manager.' } },
    { id: 'sample-org/skills/test-strategy', skillId: 'test-strategy', name: 'test-strategy', source: 'sample-org/skills', installs: 834100, weeklyInstalls: [80, 95, 105, 130, 150, 160, 190, 230], sourceType: 'github', url: 'https://skills.sh/sample-org/skills/test-strategy', installable: true, claude: { tool: 'claude', status: 'available' }, codex: { tool: 'codex', status: 'installed-off', message: '' } },
    { id: 'catalog.example/docs-review', skillId: 'docs-review', name: 'docs-review', source: 'catalog.example', installs: 562800, sourceType: 'well-known', url: 'https://catalog.example/skills/docs-review', installable: false, claude: { tool: 'claude', status: 'available' }, codex: { tool: 'codex', status: 'available' } },
  ]
}

function demoDiscoverPage(view: string) { return { view, page: 0, total: demoDiscoverSkills().length, hasMore: false, skills: demoDiscoverSkills(), fetchedAt: new Date().toISOString(), offline: false, fromCache: false } }

function demoBudgets(pending: PendingChange[]) {
  return {
    claude: demoBudget('claude', 'Claude default', 1680, 2000, pending.filter((change) => change.tool === 'claude').length),
    codex: demoBudget('codex', 'gpt-5.6-sol', 1951, 5440, pending.filter((change) => change.tool === 'codex').length),
  }
}

function demoBudget(tool: string, model: string, tokens: number, budgetTokens: number, pendingCount: number) {
  const projectedTokens = Math.max(0, tokens - pendingCount * 42)
  const usage = (value: number) => ({ skillCount: 9, requestedCharacters: value * 4, renderedCharacters: Math.min(value, budgetTokens) * 4, estimatedTokens: value, renderedTokens: Math.min(value, budgetTokens), usedPercent: Math.round(value / budgetTokens * 1000) / 10, shortenedDescriptions: 0, omittedSkills: 0, health: value > budgetTokens ? 'over-budget' : value / budgetTokens >= .8 ? 'near-limit' : 'ok' })
  return {
    tool,
    model,
    contextWindowTokens: tool === 'codex' ? 272000 : 200000,
    contextWindowAssumed: tool === 'claude',
    budgetFraction: tool === 'codex' ? .02 : .01,
    budgetCharacters: budgetTokens * 4,
    budgetTokens,
    budgetLabel: tool === 'codex' ? '2% of model context' : '1.0% of model context',
    accuracy: tool === 'codex' ? 'measured' : 'partial',
    coverage: 'Global personal and provider catalogs.',
    message: tool === 'codex' ? "Measured from Codex's model-visible global catalog." : 'Claude local catalog estimate; provider-only skills may be unavailable.',
    current: usage(tokens),
    projected: usage(projectedTokens),
    projectionChanged: pendingCount > 0,
  }
}
