import {
  ApplyInstall,
  ApplyPending,
  ChooseLocalInstall,
  ClearPending,
  CreateSkillSet,
  DeleteSkillSet,
  ExtendSources,
  GetSnapshot,
  MeasureContextBudgets,
  PrepareGitInstall,
  PreviewExtend,
  PreviewSkillSetToggle,
  PreviewUninstall,
  ReviewInstall,
  SetSkillFavorite,
  ToggleBoth,
  ToggleCell,
  ToggleGroup,
  ToggleGroupScope,
  ToggleSkillScope,
  ToggleSkillSet,
  ToggleVisible,
  UndoCell,
  UninstallSource,
  UpdateAllSources,
  UpdateSkillSet,
  UpdateSource,
} from '../wailsjs/go/main/App'
import type { contextbudget, gui } from '../wailsjs/go/models'

export type Snapshot = gui.Snapshot
export type SkillRow = gui.SkillRow
export type SkillCell = gui.SkillCell
export type SkillSet = gui.SkillSet
export type SkillSetMutationResult = gui.SkillSetMutationResult
export type SkillSetTogglePreview = gui.SkillSetTogglePreview
export type FavoriteMutationResult = gui.FavoriteMutationResult
export type PendingChange = gui.PendingChange
export type ActionResult = gui.ActionResult
export type ApplyResult = gui.ApplyResult
export type ContextBudgetReports = contextbudget.Reports
export type ContextBudgetToolReport = contextbudget.ToolReport
export type ManagedSource = gui.ManagedSource
export type InstallDraft = gui.InstallDraft
export type InstallReview = gui.InstallReview
export type InstallCellRequest = gui.InstallCellRequest
export type SourceMutationResult = gui.SourceMutationResult
export type UninstallPreview = gui.UninstallPreview
export type ExtendPreview = gui.ExtendPreview
export type ExtendPreviewSource = gui.ExtendPreviewSource
export const MANAGED_TOOLS = ['claude', 'codex', 'muse'] as const
export type ManagedTool = (typeof MANAGED_TOOLS)[number]
export function toolDisplayName(tool: ManagedTool): string {
  return tool === 'claude' ? 'Claude' : tool === 'codex' ? 'Codex' : 'Muse'
}
export function toolFullName(tool: ManagedTool): string {
  return tool === 'claude' ? 'Claude Code' : toolDisplayName(tool)
}
export interface SourceProgress {
  operation: string
  phase: string
  group?: string
  current?: number
  total?: number
  message: string
}

export interface Backend {
  getSnapshot(includeReadOnly: boolean): Promise<Snapshot>
  measureContextBudgets(): Promise<Snapshot>
  toggleCell(skillName: string, tool: string): Promise<ActionResult>
  toggleBoth(skillName: string): Promise<ActionResult>
  toggleGroup(group: string): Promise<ActionResult>
  toggleGroupScope(group: string, tools: string[]): Promise<ActionResult>
  toggleSkillScope(skillNames: string[], tools: string[]): Promise<ActionResult>
  toggleVisible(skillNames: string[]): Promise<ActionResult>
  undoCell(skillName: string, tool: string): Promise<ActionResult>
  clearPending(): Promise<ActionResult>
  applyPending(includeReadOnly: boolean): Promise<ApplyResult>
  createSkillSet(name: string, description: string, skillNames: string[]): Promise<SkillSetMutationResult>
  updateSkillSet(setID: string, name: string, description: string, skillNames: string[]): Promise<SkillSetMutationResult>
  deleteSkillSet(setID: string): Promise<SkillSetMutationResult>
  previewSkillSetToggle(setID: string, tools: string[]): Promise<SkillSetTogglePreview>
  toggleSkillSet(setID: string, tools: string[]): Promise<ActionResult>
  setSkillFavorite(skillName: string, favorite: boolean): Promise<FavoriteMutationResult>
  prepareGitInstall(gitURL: string): Promise<InstallDraft>
  chooseLocalInstall(): Promise<InstallDraft>
  reviewInstall(draftID: string, selections: InstallCellRequest[]): Promise<InstallReview>
  applyInstall(reviewID: string, includeReadOnly: boolean): Promise<SourceMutationResult>
  updateSource(sourceID: string, includeReadOnly: boolean): Promise<SourceMutationResult>
  updateAllSources(includeReadOnly: boolean): Promise<SourceMutationResult>
  previewUninstall(sourceID: string): Promise<UninstallPreview>
  uninstallSource(sourceID: string, confirmation: string, includeReadOnly: boolean): Promise<SourceMutationResult>
  previewExtend(tool: string): Promise<ExtendPreview>
  extendSources(tool: string, includeReadOnly: boolean): Promise<SourceMutationResult>
}

const generatedBackend: Backend = {
  getSnapshot: GetSnapshot,
  measureContextBudgets: MeasureContextBudgets,
  toggleCell: ToggleCell,
  toggleBoth: ToggleBoth,
  toggleGroup: ToggleGroup,
  toggleGroupScope: ToggleGroupScope,
  toggleSkillScope: ToggleSkillScope,
  toggleVisible: ToggleVisible,
  undoCell: UndoCell,
  clearPending: ClearPending,
  applyPending: ApplyPending,
  createSkillSet: CreateSkillSet,
  updateSkillSet: UpdateSkillSet,
  deleteSkillSet: DeleteSkillSet,
  previewSkillSetToggle: PreviewSkillSetToggle,
  toggleSkillSet: ToggleSkillSet,
  setSkillFavorite: SetSkillFavorite,
  prepareGitInstall: PrepareGitInstall,
  chooseLocalInstall: ChooseLocalInstall,
  reviewInstall: ReviewInstall,
  applyInstall: ApplyInstall,
  updateSource: UpdateSource,
  updateAllSources: UpdateAllSources,
  previewUninstall: PreviewUninstall,
  uninstallSource: UninstallSource,
  previewExtend: PreviewExtend,
  extendSources: ExtendSources,
}

async function activeBackend(): Promise<Backend> {
  const bridgeAvailable = Boolean((window as Window & { go?: unknown }).go)
  if (bridgeAvailable) return generatedBackend
  if (import.meta.env.DEV) return (await import('./demoBackend')).demoBackend
  throw new Error('The native Skill Manager bridge is unavailable.')
}

export const wailsBackend: Backend = {
  getSnapshot: async (...args) => (await activeBackend()).getSnapshot(...args),
  measureContextBudgets: async (...args) => (await activeBackend()).measureContextBudgets(...args),
  toggleCell: async (...args) => (await activeBackend()).toggleCell(...args),
  toggleBoth: async (...args) => (await activeBackend()).toggleBoth(...args),
  toggleGroup: async (...args) => (await activeBackend()).toggleGroup(...args),
  toggleGroupScope: async (...args) => (await activeBackend()).toggleGroupScope(...args),
  toggleSkillScope: async (...args) => (await activeBackend()).toggleSkillScope(...args),
  toggleVisible: async (...args) => (await activeBackend()).toggleVisible(...args),
  undoCell: async (...args) => (await activeBackend()).undoCell(...args),
  clearPending: async (...args) => (await activeBackend()).clearPending(...args),
  applyPending: async (...args) => (await activeBackend()).applyPending(...args),
  createSkillSet: async (...args) => (await activeBackend()).createSkillSet(...args),
  updateSkillSet: async (...args) => (await activeBackend()).updateSkillSet(...args),
  deleteSkillSet: async (...args) => (await activeBackend()).deleteSkillSet(...args),
  previewSkillSetToggle: async (...args) => (await activeBackend()).previewSkillSetToggle(...args),
  toggleSkillSet: async (...args) => (await activeBackend()).toggleSkillSet(...args),
  setSkillFavorite: async (...args) => (await activeBackend()).setSkillFavorite(...args),
  prepareGitInstall: async (...args) => (await activeBackend()).prepareGitInstall(...args),
  chooseLocalInstall: async (...args) => (await activeBackend()).chooseLocalInstall(...args),
  reviewInstall: async (...args) => (await activeBackend()).reviewInstall(...args),
  applyInstall: async (...args) => (await activeBackend()).applyInstall(...args),
  updateSource: async (...args) => (await activeBackend()).updateSource(...args),
  updateAllSources: async (...args) => (await activeBackend()).updateAllSources(...args),
  previewUninstall: async (...args) => (await activeBackend()).previewUninstall(...args),
  uninstallSource: async (...args) => (await activeBackend()).uninstallSource(...args),
  previewExtend: async (...args) => (await activeBackend()).previewExtend(...args),
  extendSources: async (...args) => (await activeBackend()).extendSources(...args),
}

export function projectPending(snapshot: Snapshot, pending: PendingChange[], contextBudgets: ContextBudgetReports, skillSets = snapshot.skillSets, skillSetsWarning = snapshot.skillSetsWarning): Snapshot {
  const byCell = new Map(pending.map((change) => [`${change.tool}:${change.skillName}`, change.operation]))
  return {
    ...snapshot,
    pending,
    contextBudgets,
    skillSets: skillSets ?? [],
    skillSetsWarning,
    rows: snapshot.rows.map((row) => ({
      ...row,
      ...Object.fromEntries(MANAGED_TOOLS.map((tool) => [tool, projectCell(row[tool], byCell)])),
    })),
  } as Snapshot
}

export function favoriteEligible(row: SkillRow): boolean {
  return MANAGED_TOOLS.some((tool) => {
    const cell = row[tool]
    return Boolean(cell && !cell.readOnly && (cell.conflict || ['ON', 'OFF', 'CONFLICT'].includes(cell.state)))
  })
}

function projectCell(cell: SkillCell | undefined, pending: Map<string, string>): SkillCell | undefined {
  if (!cell) return undefined
  const operation = pending.get(`${cell.tool}:${cell.name}`)
  return {
    ...cell,
    pending: operation,
    effectiveState: operation === 'disable' ? 'OFF' : operation === 'enable' ? 'ON' : cell.state,
  } as SkillCell
}
