import type { ActionResult, ApplyResult, Backend, ManagedTool, Snapshot } from '../api'
import { DEMO_BUDGET_SPECS } from '../demoBackend'
import { contextbudget, gui } from '../../wailsjs/go/models'

export function fixtureSnapshot(): Snapshot {
  return new gui.Snapshot({
    rows: [
      {
        name: 'alpha-skill',
        description: 'Alpha automation skill',
        source: 'local',
        group: 'local',
        favorite: true,
        claude: cell('alpha-skill', 'claude', 'ON'),
        codex: cell('alpha-skill', 'codex', 'OFF'),
        muse: cell('alpha-skill', 'muse', 'OFF'),
        grok: cell('alpha-skill', 'grok', 'OFF'),
      },
      {
        name: 'codex-helper',
        description: 'Codex-only helper',
        source: 'Skills CLI',
        group: 'Skills CLI',
        favorite: false,
        codex: cell('codex-helper', 'codex', 'ON', 'Skills CLI', 'Skills CLI'),
      },
    ],
    skillSets: [skillSetFixture()],
    skillSetsWarning: '',
    favoritesWarning: '',
    groups: [
      { group: 'local', rows: 1, claude: counts(1, 0), codex: counts(0, 1), muse: counts(0, 1), grok: counts(0, 1), sources: ['local'] },
      { group: 'Skills CLI', rows: 1, claude: counts(0, 0), codex: counts(1, 0), muse: counts(0, 0), grok: counts(0, 0), sources: ['Skills CLI'] },
    ],
    sources: ['Skills CLI', 'local'],
    managedSources: [
      {
        sourceId: 'git:fixture',
        kind: 'git',
        group: 'demo/skills',
        location: 'https://github.com/demo/skills',
        skillCount: 2,
        claudeCount: 2,
        codexCount: 1,
        museCount: 1,
        grokCount: 1,
        installedAt: '2026-08-11T09:00:00Z',
        commit: '1234567890abcdef',
        canUpdate: true,
        updateMode: 'Managed Git',
        updateHint: 'Use Update to fetch changes.',
      },
      {
        sourceId: 'local:fixture',
        kind: 'local',
        group: 'local-skills',
        location: '/Users/example/Projects/local-skills',
        skillCount: 1,
        claudeCount: 1,
        codexCount: 1,
        museCount: 1,
        grokCount: 1,
        installedAt: '2026-08-11T09:00:00Z',
        canUpdate: false,
        updateMode: 'Linked folder',
        updateHint: 'Changes are read directly; no update needed.',
      },
    ],
    stats: {
      managedSkills: 2,
      readOnlySkills: 0,
      claude: counts(1, 0),
      codex: counts(1, 1),
      muse: counts(0, 1),
      grok: counts(0, 1),
      conflictCells: 0,
    },
    conflicts: [],
    contextBudgets: budgetReports(),
    pending: [],
    includeReadOnly: false,
    scannedAt: '2026-08-11T10:30:00Z',
  })
}

export function mockBackend(snapshot = fixtureSnapshot()): Backend {
  const favorites = new Set(snapshot.rows.filter((row) => row.favorite).map((row) => row.name))
  const action = (pending = snapshot.pending): ActionResult => new gui.ActionResult({
    message: 'Pending change added.',
    counts: { changed: 1, removed: 0, skippedReadOnly: 0, skippedMissing: 0, skippedConflict: 0 },
    pending,
    contextBudgets: budgetReports(pending),
    skillSets: snapshot.skillSets,
    skillSetsWarning: snapshot.skillSetsWarning,
  })
  return {
    getSnapshot: vi.fn(async () => snapshot),
    measureContextBudgets: vi.fn(async () => snapshot),
    toggleCell: vi.fn(async (name, tool) => action([{ skillName: name, tool, operation: 'disable' }] as never)),
    toggleBoth: vi.fn(async () => action()),
    toggleGroup: vi.fn(async () => action()),
    toggleGroupScope: vi.fn(async () => action()),
    toggleSkillScope: vi.fn(async () => action()),
    toggleVisible: vi.fn(async () => action()),
    undoCell: vi.fn(async () => action([])),
    clearPending: vi.fn(async () => action([])),
    applyPending: vi.fn(async (): Promise<ApplyResult> => new gui.ApplyResult({ completed: [], message: 'Applied 1 change(s).', snapshot })),
    createSkillSet: vi.fn(async () => new gui.SkillSetMutationResult({ message: 'Created Skill Set.', skillSets: snapshot.skillSets, warning: '' })),
    updateSkillSet: vi.fn(async () => new gui.SkillSetMutationResult({ message: 'Updated Skill Set.', skillSets: snapshot.skillSets, warning: '' })),
    deleteSkillSet: vi.fn(async () => new gui.SkillSetMutationResult({ message: 'Deleted Skill Set.', skillSets: [], warning: '' })),
    previewSkillSetToggle: vi.fn(async (setId, tools) => new gui.SkillSetTogglePreview({ setId, name: 'Review support', tools, direction: 'disable', eligible: tools.length, counts: { changed: tools.length, removed: 0, skippedReadOnly: 0, skippedMissing: 0, skippedConflict: 0 } })),
    toggleSkillSet: vi.fn(async () => action()),
    setSkillFavorite: vi.fn(async (skillName, favorite) => {
      if (favorite) favorites.add(skillName)
      else favorites.delete(skillName)
      return new gui.FavoriteMutationResult({ message: `${favorite ? 'Added' : 'Removed'} ${skillName} ${favorite ? 'to' : 'from'} favorites.`, favorites: [...favorites].sort(), warning: '' })
    }),
    prepareGitInstall: vi.fn(async () => new gui.InstallDraft({ draftId: 'draft:1', kind: 'git', group: 'demo/skills', location: 'https://github.com/demo/skills', candidates: [candidate('alpha')], cloned: true, reused: false, retainedClone: true, cancelled: false })),
    chooseLocalInstall: vi.fn(async () => new gui.InstallDraft({ draftId: 'draft:2', kind: 'local', group: 'local-skills', location: '/Users/example/Projects/local-skills', candidates: [candidate('local-alpha')], cloned: false, reused: false, retainedClone: false, cancelled: false })),
    reviewInstall: vi.fn(async (draftId, selections) => new gui.InstallReview({ reviewId: 'review:1', draftId, group: 'demo/skills', selections, createCount: selections.length, alreadyOnCount: 0, alreadyOffCount: 0, conflicts: [], ready: true })),
    applyInstall: vi.fn(async () => new gui.SourceMutationResult({ message: 'Installed 2 links.', completed: [], createdLinks: 2, alreadyInstalled: 0, snapshot })),
    updateSource: vi.fn(async () => new gui.SourceMutationResult({ message: 'Updated 1 source(s); 0 already up to date.', completed: [], snapshot })),
    updateAllSources: vi.fn(async () => new gui.SourceMutationResult({ message: 'Updated 1 source(s); 0 already up to date.', completed: [], snapshot })),
    previewUninstall: vi.fn(async (sourceId) => new gui.UninstallPreview({ sourceId, kind: 'git', group: 'demo/skills', location: '/tmp/demo', activeLinks: 2, disabledLinks: 0, removesCheckout: true, preservesSource: false, affectedSkillSets: [], skillSetImpactWarning: '', affectedFavorites: [], favoriteImpactWarning: '' })),
    uninstallSource: vi.fn(async () => new gui.SourceMutationResult({ message: 'Uninstalled source.', completed: [], removedActive: 2, removedDisabled: 0, snapshot })),
    previewExtend: vi.fn(async (tool) => new gui.ExtendPreview({ tool, sources: [new gui.ExtendPreviewSource({ kind: 'git', group: 'demo/skills', skillNames: ['alpha'], skillCount: 1, created: 1, alreadyInstalled: 0, disabledAfter: 0, status: 'ready', reason: '', skipped: [], conflicts: [] })], createCount: 1, blockedCount: 0 })),
    extendSources: vi.fn(async (tool) => new gui.SourceMutationResult({ message: `1 source(s) extended to ${tool}: 1 created, 0 already installed.`, completed: [], createdLinks: 1, alreadyInstalled: 0, snapshot })),
  }
}

function skillSetFixture() {
  return {
    setId: 'set:review-support',
    name: 'Review support',
    description: 'Use when reviewing a cross-tool change.',
    members: [
      {
        name: 'alpha-skill', description: 'Alpha automation skill', group: 'local', source: 'local', available: true,
        claude: setMemberCell('claude', 'ON'), codex: setMemberCell('codex', 'OFF'), muse: setMemberCell('muse', 'OFF'), grok: setMemberCell('grok', 'OFF'),
      },
    ],
    claude: setSummary('claude', 'enabled', 1, 1, 0),
    codex: setSummary('codex', 'disabled', 1, 0, 1),
    muse: setSummary('muse', 'disabled', 1, 0, 1),
    grok: setSummary('grok', 'disabled', 1, 0, 1),
    unavailable: 0,
    pending: 0,
    createdAt: '2026-08-11T09:00:00Z',
    updatedAt: '2026-08-11T09:00:00Z',
  }
}

function setMemberCell(tool: string, state: string) {
  return { tool, state, effectiveState: state, pending: '', eligible: true, reason: '' }
}

function setSummary(tool: string, status: string, eligible: number, on: number, off: number) {
  return { tool, appliedStatus: status, effectiveStatus: status, eligible, on, off, effectiveOn: on, effectiveOff: off, pending: 0, missing: 0, readOnly: 0, conflict: 0 }
}

function candidate(name: string) {
  return {
    name,
    relativePath: `skills/${name}`,
    claude: { tool: 'claude', status: 'available', message: '' },
    codex: { tool: 'codex', status: 'available', message: '' },
    muse: { tool: 'muse', status: 'available', message: '' },
    grok: { tool: 'grok', status: 'available', message: '' },
  }
}

function budgetReports(pending: Array<{ tool: string }> = []) {
  return new contextbudget.Reports({
    claude: budgetReport('claude', 640, pending.filter((change) => change.tool === 'claude').length, false),
    codex: budgetReport('codex', 930, pending.filter((change) => change.tool === 'codex').length, true),
    muse: budgetReport('muse', 640, pending.filter((change) => change.tool === 'muse').length, false),
    grok: budgetReport('grok', 640, pending.filter((change) => change.tool === 'grok').length, false),
  })
}

function budgetReport(tool: ManagedTool, tokens: number, pendingCount: number, measured: boolean) {
  const spec = DEMO_BUDGET_SPECS[tool]
  const currentPercent = Math.round(tokens / spec.budgetTokens * 1000) / 10
  const projectedTokens = Math.max(0, tokens - pendingCount * 35)
  const projectedPercent = Math.round(projectedTokens / spec.budgetTokens * 1000) / 10
  const usage = (estimatedTokens: number, usedPercent: number) => ({
    skillCount: 2,
    requestedCharacters: estimatedTokens * 4,
    renderedCharacters: estimatedTokens * 4,
    estimatedTokens,
    renderedTokens: estimatedTokens,
    usedPercent,
    shortenedDescriptions: 0,
    omittedSkills: 0,
    health: usedPercent >= 100 ? 'over-budget' : usedPercent >= 80 ? 'near-limit' : 'ok',
  })
  return {
    tool,
    model: spec.model,
    contextWindowTokens: spec.contextWindowTokens,
    contextWindowAssumed: spec.contextWindowAssumed,
    budgetFraction: spec.budgetFraction,
    budgetCharacters: spec.budgetTokens * 4,
    budgetTokens: spec.budgetTokens,
    budgetLabel: spec.budgetLabel,
    accuracy: spec.accuracy(measured),
    coverage: 'Global test catalog.',
    message: spec.message(measured),
    current: usage(tokens, currentPercent),
    projected: usage(projectedTokens, projectedPercent),
    projectionChanged: pendingCount > 0,
  }
}

function cell(name: string, tool: string, state: string, source = 'local', group = 'local') {
  return {
    tool,
    name,
    displayName: name,
    description: '',
    state,
    effectiveState: state,
    source,
    group,
    entryType: 'dir',
    activePath: `/tmp/${tool}/${name}`,
    disabledPath: '',
    skillFilePath: `/tmp/${tool}/${name}/SKILL.md`,
    symlinkTarget: '',
    repoOrigin: '',
    repoCommit: '',
    readOnly: false,
  }
}

function counts(on: number, off: number) {
  return { on, off, conflict: 0, readOnly: 0 }
}
