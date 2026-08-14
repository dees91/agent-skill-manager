export const VIDEO = {
  revision: 1,
  fps: 30,
  width: 1920,
  height: 1080,
  durationInFrames: 600,
} as const

export type SceneID = 'problem' | 'terminal' | 'dashboard' | 'skills' | 'sources' | 'closing'

export type StoryScene = {
  id: SceneID
  start: number
  end: number
  copy: Readonly<Record<string, string | readonly string[]>>
  assets?: readonly string[]
}

export const STORYBOARD: readonly StoryScene[] = [
  {
    id: 'problem',
    start: 0,
    end: 84,
    copy: {
      eyebrow: 'CLAUDE CODE + CODEX',
      headline: 'Agent skills, scattered across tools?',
      claudePath: '~/.claude/skills',
      codexPath: '~/.agents/skills',
      skillNames: ['release-checklist', 'dependency-review', 'ui-accessibility'],
    },
  },
  {
    id: 'terminal',
    start: 78,
    end: 165,
    copy: {
      eyebrow: 'TERMINAL FIRST',
      headline: 'One local inventory.',
      command: '$ skill-manager status',
      output: ['ON: 12', 'OFF: 3', 'CONFLICT: 0', 'RO: 3'],
    },
  },
  {
    id: 'dashboard',
    start: 156,
    end: 258,
    copy: {
      callout: 'See visibility and estimated prompt cost.',
    },
    assets: ['ui/dashboard.png'],
  },
  {
    id: 'skills',
    start: 249,
    end: 420,
    copy: {
      calloutStage: 'Stage first.',
      calloutApply: 'Apply once.',
      calloutRestore: 'Restore anytime.',
      pointerToggle: 'Codex visibility toggle',
      pointerApply: 'Apply changes',
    },
    assets: ['ui/skills-before.png', 'ui/skills-pending.png', 'ui/skills-after.png'],
  },
  {
    id: 'sources',
    start: 411,
    end: 519,
    copy: {
      callout: 'Git repositories and linked folders.',
    },
    assets: ['ui/sources.png'],
  },
  {
    id: 'closing',
    start: 510,
    end: 600,
    copy: {
      product: 'Skill Manager',
      headline: 'Keep every skill. Load only what you need.',
      footer: 'Claude Code + Codex · local · reversible',
    },
    assets: ['appicon.svg'],
  },
] as const

export function getScene(id: SceneID): StoryScene {
  const scene = STORYBOARD.find((candidate) => candidate.id === id)
  if (!scene) throw new Error(`Unknown scene: ${id}`)
  return scene
}

export function sceneDuration(scene: StoryScene): number {
  return scene.end - scene.start
}

export function copyString(scene: StoryScene, key: string): string {
  const value = scene.copy[key]
  if (typeof value !== 'string') throw new Error(`Scene ${scene.id} copy ${key} is not a string`)
  return value
}

export function copyLines(scene: StoryScene, key: string): readonly string[] {
  const value = scene.copy[key]
  if (!Array.isArray(value)) throw new Error(`Scene ${scene.id} copy ${key} is not a list`)
  return value
}
