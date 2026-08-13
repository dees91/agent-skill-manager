import type { ActionResult, ApplyResult, Backend, Snapshot } from '../api'
import { contextbudget, gui } from '../../wailsjs/go/models'

export function fixtureSnapshot(): Snapshot {
  return new gui.Snapshot({
    rows: [
      {
        name: 'alpha-skill',
        description: 'Alpha automation skill',
        source: 'local',
        group: 'local',
        claude: cell('alpha-skill', 'claude', 'ON'),
        codex: cell('alpha-skill', 'codex', 'OFF'),
      },
      {
        name: 'codex-helper',
        description: 'Codex-only helper',
        source: 'Skills CLI',
        group: 'Skills CLI',
        codex: cell('codex-helper', 'codex', 'ON', 'Skills CLI', 'Skills CLI'),
      },
    ],
    groups: [
      { group: 'local', rows: 1, claude: counts(1, 0), codex: counts(0, 1), sources: ['local'] },
      { group: 'Skills CLI', rows: 1, claude: counts(0, 0), codex: counts(1, 0), sources: ['Skills CLI'] },
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
  const action = (pending = snapshot.pending): ActionResult => new gui.ActionResult({
    message: 'Pending change added.',
    counts: { changed: 1, removed: 0, skippedReadOnly: 0, skippedMissing: 0, skippedConflict: 0 },
    pending,
    contextBudgets: budgetReports(pending),
  })
  return {
    getSnapshot: vi.fn(async () => snapshot),
    toggleCell: vi.fn(async (name, tool) => action([{ skillName: name, tool, operation: 'disable' }] as never)),
    toggleBoth: vi.fn(async () => action()),
    toggleGroup: vi.fn(async () => action()),
    toggleGroupScope: vi.fn(async () => action()),
    toggleSkillScope: vi.fn(async () => action()),
    toggleVisible: vi.fn(async () => action()),
    undoCell: vi.fn(async () => action([])),
    clearPending: vi.fn(async () => action([])),
    applyPending: vi.fn(async (): Promise<ApplyResult> => new gui.ApplyResult({ completed: [], message: 'Applied 1 change(s).', snapshot })),
    prepareGitInstall: vi.fn(async () => new gui.InstallDraft({ draftId: 'draft:1', kind: 'git', group: 'demo/skills', location: 'https://github.com/demo/skills', candidates: [candidate('alpha')], cloned: true, reused: false, retainedClone: true, cancelled: false })),
    chooseLocalInstall: vi.fn(async () => new gui.InstallDraft({ draftId: 'draft:2', kind: 'local', group: 'local-skills', location: '/Users/example/Projects/local-skills', candidates: [candidate('local-alpha')], cloned: false, reused: false, retainedClone: false, cancelled: false })),
    reviewInstall: vi.fn(async (draftId, selections) => new gui.InstallReview({ reviewId: 'review:1', draftId, group: 'demo/skills', selections, createCount: selections.length, alreadyOnCount: 0, alreadyOffCount: 0, conflicts: [], ready: true })),
    applyInstall: vi.fn(async () => new gui.SourceMutationResult({ message: 'Installed 2 links.', completed: [], createdLinks: 2, alreadyInstalled: 0, snapshot })),
    getDiscoverPage: vi.fn(async (view) => discoverPage(view)),
    searchDiscover: vi.fn(async () => discoverPage('search')),
    getDiscoverSkill: vi.fn(async (skillId) => new gui.DiscoverDetail({ skill: discoverSkills().find((skill) => skill.id === skillId)!, description: 'A catalog skill description.', fetchedAt: '2026-08-12T10:00:00Z', offline: false, fromCache: false, auditStatus: 'external-only' })),
    installDiscoverSkill: vi.fn(async () => new gui.SourceMutationResult({ message: 'Installed catalog skill.', completed: [], createdLinks: 1, alreadyInstalled: 0, snapshot })),
    updateSource: vi.fn(async () => new gui.SourceMutationResult({ message: 'Updated 1 source(s); 0 already up to date.', completed: [], snapshot })),
    updateAllSources: vi.fn(async () => new gui.SourceMutationResult({ message: 'Updated 1 source(s); 0 already up to date.', completed: [], snapshot })),
    previewUninstall: vi.fn(async (sourceId) => new gui.UninstallPreview({ sourceId, kind: 'git', group: 'demo/skills', location: '/tmp/demo', activeLinks: 2, disabledLinks: 0, removesCheckout: true, preservesSource: false })),
    uninstallSource: vi.fn(async () => new gui.SourceMutationResult({ message: 'Uninstalled source.', completed: [], removedActive: 2, removedDisabled: 0, snapshot })),
  }
}

export function discoverSkills() {
  return [
    new gui.DiscoverSkill({ id: 'demo/skills/alpha', skillId: 'alpha', name: 'alpha', source: 'demo/skills', installs: 42000, weeklyInstalls: [2, 3, 4, 5, 7, 8, 9, 11], sourceType: 'github', url: 'https://www.skills.sh/demo/skills/alpha', installable: true, claude: { tool: 'claude', status: 'available' }, codex: { tool: 'codex', status: 'installed-on' } }),
    new gui.DiscoverSkill({ id: 'example.com/provider-skill', skillId: 'provider-skill', name: 'provider-skill', source: 'example.com', installs: 3000, sourceType: 'well-known', url: 'https://www.skills.sh/site/example.com/provider-skill', installable: false, claude: { tool: 'claude', status: 'available' }, codex: { tool: 'codex', status: 'available' } }),
  ]
}

function discoverPage(view: string) {
  return new gui.DiscoverPage({ view, page: 0, total: 2, hasMore: false, skills: discoverSkills(), fetchedAt: '2026-08-12T10:00:00Z', offline: false, fromCache: false })
}

function candidate(name: string) {
  return {
    name,
    relativePath: `skills/${name}`,
    claude: { tool: 'claude', status: 'available', message: '' },
    codex: { tool: 'codex', status: 'available', message: '' },
  }
}

function budgetReports(pending: Array<{ tool: string }> = []) {
  return new contextbudget.Reports({
    claude: budgetReport('claude', 'Claude default', 640, 2000, pending.filter((change) => change.tool === 'claude').length),
    codex: budgetReport('codex', 'gpt-5.6-sol', 930, 5440, pending.filter((change) => change.tool === 'codex').length),
  })
}

function budgetReport(tool: string, model: string, tokens: number, budgetTokens: number, pendingCount: number) {
  const currentPercent = Math.round(tokens / budgetTokens * 1000) / 10
  const projectedTokens = Math.max(0, tokens - pendingCount * 35)
  const projectedPercent = Math.round(projectedTokens / budgetTokens * 1000) / 10
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
    model,
    contextWindowTokens: tool === 'codex' ? 272000 : 200000,
    contextWindowAssumed: tool === 'claude',
    budgetFraction: tool === 'codex' ? .02 : .01,
    budgetCharacters: budgetTokens * 4,
    budgetTokens,
    budgetLabel: tool === 'codex' ? '2% of model context' : '1.0% of model context',
    accuracy: tool === 'codex' ? 'measured' : 'partial',
    coverage: 'Global test catalog.',
    message: tool === 'codex' ? "Measured from Codex's model-visible global catalog." : 'Labeled local estimate.',
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
