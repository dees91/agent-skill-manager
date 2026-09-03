import { render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import App from './App'
import { fixtureSnapshot, mockBackend } from './test/fixtures'
import { gui } from '../wailsjs/go/models'

describe('Skill Manager desktop app', () => {
  it('loads the dashboard from the backend snapshot', async () => {
    const backend = mockBackend()
    render(<App backend={backend} />)

    expect(await screen.findByRole('heading', { name: 'Dashboard' })).toBeInTheDocument()
    expect(screen.getByText('Managed skills')).toBeInTheDocument()
    expect(screen.getByText('Claude enabled')).toBeInTheDocument()
    expect(screen.getByText('Muse enabled')).toBeInTheDocument()
    expect(screen.getByText('Grok enabled')).toBeInTheDocument()
    expect(screen.getByText('Global skill catalog cost')).toBeInTheDocument()
    expect(screen.getByText('Measured')).toBeInTheDocument()
    expect(screen.getAllByText('Partial estimate')).toHaveLength(1)
    expect(screen.getAllByText('Estimated')).toHaveLength(2)
    expect(backend.getSnapshot).toHaveBeenCalledWith(false)
    expect(screen.queryByRole('button', { name: 'Discover' })).not.toBeInTheDocument()
  })

  it('runs provider diagnostics only after explicit confirmation', async () => {
    const user = userEvent.setup()
    const backend = mockBackend()
    render(<App backend={backend} />)

    await screen.findByRole('heading', { name: 'Dashboard' })
    expect(backend.measureContextBudgets).not.toHaveBeenCalled()
    await user.click(screen.getByRole('button', { name: 'Run provider diagnostics' }))
    await waitFor(() => expect(backend.measureContextBudgets).toHaveBeenCalledTimes(1))
  })

  it('filters the skills table without another filesystem scan', async () => {
    const user = userEvent.setup()
    const backend = mockBackend()
    render(<App backend={backend} />)
    await screen.findByRole('heading', { name: 'Dashboard' })

    await user.click(screen.getByRole('button', { name: /Skills/ }))
    const search = screen.getByRole('textbox', { name: /Search name/ })
    await user.type(search, 'codex-helper')

    const table = screen.getByRole('table')
    expect(within(table).getByText('codex-helper')).toBeInTheDocument()
    expect(within(table).queryByText('alpha-skill')).not.toBeInTheDocument()
    expect(backend.getSnapshot).toHaveBeenCalledTimes(1)
  })

  it('stages a cell toggle and applies only after explicit confirmation', async () => {
    const user = userEvent.setup()
    const backend = mockBackend()
    render(<App backend={backend} />)
    await screen.findByRole('heading', { name: 'Dashboard' })
    await user.click(screen.getByRole('button', { name: /Skills/ }))

    await user.click(screen.getByRole('button', { name: 'claude alpha-skill: ON' }))
    expect(backend.toggleCell).toHaveBeenCalledWith('alpha-skill', 'claude')
    expect(backend.applyPending).not.toHaveBeenCalled()
    expect(screen.getByText('1 pending change')).toBeInTheDocument()

    await user.click(screen.getByRole('button', { name: 'Dashboard' }))
    expect(screen.getAllByText('After Apply')).toHaveLength(1)

    await user.click(screen.getByRole('button', { name: /Apply changes/ }))
    await waitFor(() => expect(backend.applyPending).toHaveBeenCalledWith(false))
  })

  it('requests a read-only scan only when the user enables that view', async () => {
    const user = userEvent.setup()
    const snapshot = fixtureSnapshot()
    const backend = mockBackend(snapshot)
    render(<App backend={backend} />)
    await screen.findByRole('heading', { name: 'Dashboard' })
    await user.click(screen.getByRole('button', { name: /Skills/ }))

    await user.click(screen.getByRole('button', { name: 'Filters' }))
    await user.click(screen.getByRole('switch', { name: 'Read only' }))
    await waitFor(() => expect(backend.getSnapshot).toHaveBeenLastCalledWith(true))
  })

  it('keeps active skills prominent and collapses available skills by source', async () => {
    const user = userEvent.setup()
    const snapshot = withAvailableSkills()
    const backend = mockBackend(snapshot)
    render(<App backend={backend} />)
    await screen.findByRole('heading', { name: 'Dashboard' })
    await user.click(screen.getByRole('button', { name: /Skills/ }))

    expect(screen.getByRole('heading', { name: 'Active now' })).toBeInTheDocument()
    expect(screen.getByText('alpha-skill')).toBeInTheDocument()
    expect(screen.getByRole('heading', { name: 'Available by source' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /android\/skills.*symlink repo/ })).toHaveAttribute('aria-expanded', 'false')
    expect(screen.queryByText('camera-helper')).not.toBeInTheDocument()

    await user.click(screen.getByRole('button', { name: /android\/skills.*symlink repo/ }))
    expect(screen.getByText('camera-helper')).toBeInTheDocument()
    expect(screen.getAllByText('camera-helper')).toHaveLength(1)
  })

  it('preserves expanded source groups while navigating within the app session', async () => {
    const user = userEvent.setup()
    const backend = mockBackend(withAvailableSkills())
    render(<App backend={backend} />)
    await screen.findByRole('heading', { name: 'Dashboard' })
    await user.click(screen.getByRole('button', { name: /Skills/ }))
    await user.click(screen.getByRole('button', { name: /android\/skills.*symlink repo/ }))
    expect(screen.getByText('camera-helper')).toBeInTheDocument()

    await user.click(screen.getByRole('button', { name: 'Dashboard' }))
    await user.click(screen.getByRole('button', { name: /Skills/ }))
    expect(screen.getByRole('button', { name: /android\/skills.*symlink repo/ })).toHaveAttribute('aria-expanded', 'true')
    expect(screen.getByText('camera-helper')).toBeInTheDocument()
  })

  it('uses tool chips for classification and scoped bulk actions while keeping all three columns', async () => {
    const user = userEvent.setup()
    const backend = mockBackend(withAvailableSkills())
    render(<App backend={backend} />)
    await screen.findByRole('heading', { name: 'Dashboard' })
    await user.click(screen.getByRole('button', { name: /Skills/ }))
    await user.click(within(screen.getByRole('group', { name: 'Tool scope' })).getByRole('button', { name: 'Claude' }))

    expect(screen.getAllByRole('columnheader', { name: 'Claude' }).length).toBeGreaterThan(0)
    expect(screen.getAllByRole('columnheader', { name: 'Codex' }).length).toBeGreaterThan(0)
    expect(screen.getAllByRole('columnheader', { name: 'Muse' }).length).toBeGreaterThan(0)
    await user.click(screen.getByRole('button', { name: /Smart-toggle filtered results/ }))
    expect(backend.toggleSkillScope).toHaveBeenLastCalledWith(expect.arrayContaining(['alpha-skill', 'camera-helper', 'layout-helper']), ['claude'])
  })

  it('search expands matching source groups and group bulk still targets the entire source', async () => {
    const user = userEvent.setup()
    const backend = mockBackend(withAvailableSkills())
    render(<App backend={backend} />)
    await screen.findByRole('heading', { name: 'Dashboard' })
    await user.click(screen.getByRole('button', { name: /Skills/ }))

    const search = screen.getByRole('textbox', { name: /Search name/ })
    await user.type(search, 'camera')
    expect(screen.getByText('camera-helper')).toBeInTheDocument()
    const groupToggle = screen.getByRole('button', { name: /android\/skills.*symlink repo/ })
    expect(groupToggle).toHaveAttribute('aria-expanded', 'true')
    await user.click(screen.getByRole('button', { name: /Smart-toggle entire group android\/skills/ }))
    expect(backend.toggleGroupScope).toHaveBeenCalledWith('android/skills', ['claude', 'codex', 'muse', 'grok'])

    await user.clear(search)
    expect(screen.getByRole('button', { name: /android\/skills.*symlink repo/ })).toHaveAttribute('aria-expanded', 'false')
    expect(screen.queryByText('camera-helper')).not.toBeInTheDocument()
  })

  it('filters to favorites, expands their available groups, and removes a favorite without touching Pending', async () => {
    const user = userEvent.setup()
    const snapshot = withAvailableSkills()
    const camera = snapshot.rows.find((row) => row.name === 'camera-helper')!
    camera.favorite = true
    const system = skillRow('system-only')
    system.favorite = false
    system.claude!.state = 'RO'
    system.claude!.effectiveState = 'RO'
    system.claude!.readOnly = true
    system.codex!.state = 'RO'
    system.codex!.effectiveState = 'RO'
    system.codex!.readOnly = true
    system.muse!.state = 'RO'
    system.muse!.effectiveState = 'RO'
    system.muse!.readOnly = true
    snapshot.rows.push(system)
    snapshot.pending = [{ skillName: 'alpha-skill', tool: 'claude', operation: 'disable' }] as never
    const backend = mockBackend(snapshot)
    backend.setSkillFavorite = vi.fn(async (skillName, favorite) => new gui.FavoriteMutationResult({
      message: `${favorite ? 'Added' : 'Removed'} ${skillName} ${favorite ? 'to' : 'from'} favorites.`,
      favorites: ['camera-helper', 'system-only'],
      warning: '',
    }))
    render(<App backend={backend} />)
    await screen.findByRole('heading', { name: 'Dashboard' })
    await user.click(screen.getByRole('button', { name: /Skills/ }))
    await user.click(within(screen.getByRole('group', { name: 'Skill availability' })).getByRole('button', { name: /Favorites 2/ }))

    expect(screen.getByText('alpha-skill')).toBeInTheDocument()
    expect(screen.getByText('camera-helper')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /android\/skills.*symlink repo/ })).toHaveAttribute('aria-expanded', 'true')

    await user.click(screen.getByRole('button', { name: 'Remove alpha-skill from favorites' }))
    await waitFor(() => expect(backend.setSkillFavorite).toHaveBeenCalledWith('alpha-skill', false))
    expect(screen.queryByText('alpha-skill')).not.toBeInTheDocument()
    expect(screen.getByText('camera-helper')).toBeInTheDocument()
    expect(screen.queryByText('system-only')).not.toBeInTheDocument()
    expect(screen.getByText('1 pending change')).toBeInTheDocument()
  })

  it('updates a favorite from skill details and keeps favorite controls isolated on metadata errors', async () => {
    const user = userEvent.setup()
    const snapshot = fixtureSnapshot()
    snapshot.favoritesWarning = 'decode favorites failed'
    const backend = mockBackend(snapshot)
    const { unmount } = render(<App backend={backend} />)
    await screen.findByRole('heading', { name: 'Dashboard' })
    await user.click(screen.getByRole('button', { name: /Skills/ }))

    expect(screen.getByRole('alert')).toHaveTextContent('Favorites are unavailable')
    expect(screen.getByRole('button', { name: 'Remove alpha-skill from favorites' })).toBeDisabled()
    expect(screen.getByRole('button', { name: 'claude alpha-skill: ON' })).toBeEnabled()

    unmount()
    snapshot.favoritesWarning = ''
    const healthyBackend = mockBackend(snapshot)
    render(<App backend={healthyBackend} />)
    await screen.findByRole('heading', { name: 'Dashboard' })
    await user.click(screen.getByRole('button', { name: /Skills/ }))
    await user.click(screen.getByText('alpha-skill'))
    await user.click(screen.getByRole('button', { name: 'Remove from favorites' }))
    await waitFor(() => expect(healthyBackend.setSkillFavorite).toHaveBeenCalledWith('alpha-skill', false))
  })

  it('keeps a pending disable in Active now until Apply', async () => {
    const user = userEvent.setup()
    const backend = mockBackend(withAvailableSkills())
    render(<App backend={backend} />)
    await screen.findByRole('heading', { name: 'Dashboard' })
    await user.click(screen.getByRole('button', { name: /Skills/ }))

    await user.click(screen.getByRole('button', { name: 'claude alpha-skill: ON' }))
    expect(screen.getByRole('heading', { name: 'Active now' })).toBeInTheDocument()
    expect(screen.getByText('alpha-skill')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /claude alpha-skill: OFF, pending disable/ })).toBeInTheDocument()
  })

  it('keeps conflicts in Needs attention even when Available is selected', async () => {
    const user = userEvent.setup()
    const snapshot = withAvailableSkills()
    const conflict = skillRow('blocked-helper')
    conflict.claude!.state = 'CONFLICT'
    conflict.claude!.effectiveState = 'CONFLICT'
    conflict.claude!.conflict = new gui.Conflict({ originalPath: '/tmp/claude/blocked-helper', disabledPath: '/tmp/disabled/claude/blocked-helper', blockerPath: '/tmp/claude/blocked-helper', message: 'Restore is blocked.' })
    snapshot.rows.push(conflict)
    snapshot.stats.conflictCells = 1
    const backend = mockBackend(snapshot)
    render(<App backend={backend} />)
    await screen.findByRole('heading', { name: 'Dashboard' })
    await user.click(screen.getByRole('button', { name: /Skills/ }))
    await user.click(within(screen.getByRole('group', { name: 'Skill availability' })).getByRole('button', { name: /Available/ }))

    expect(screen.getByRole('heading', { name: 'Needs attention' })).toBeInTheDocument()
    expect(screen.getByText('blocked-helper')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /claude blocked-helper: CONFLICT/ })).toBeDisabled()
  })

  it('shows saved Skill Sets and requires an explicit tool scope before staging', async () => {
    const user = userEvent.setup()
    const backend = mockBackend()
    render(<App backend={backend} />)
    await screen.findByRole('heading', { name: 'Dashboard' })

    await user.click(screen.getByRole('button', { name: 'Skill Sets' }))
    expect(screen.getByRole('heading', { name: 'Skill Sets' })).toBeInTheDocument()
    expect(screen.getByText('Review support')).toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: 'Toggle…' }))
    const dialog = screen.getByRole('dialog', { name: 'Toggle Review support' })
    expect(backend.previewSkillSetToggle).not.toHaveBeenCalled()
    expect(within(dialog).getByRole('button', { name: /Stage changes/ })).toBeDisabled()

    await user.click(within(dialog).getByRole('button', { name: 'Codex' }))
    await waitFor(() => expect(backend.previewSkillSetToggle).toHaveBeenCalledWith('set:review-support', ['codex']))
    await user.click(await within(dialog).findByRole('button', { name: 'Stage disable' }))
    await waitFor(() => expect(backend.toggleSkillSet).toHaveBeenCalledWith('set:review-support', ['codex']))
  })

  it('creates a Skill Set from unique names in Pending', async () => {
    const user = userEvent.setup()
    const backend = mockBackend()
    render(<App backend={backend} />)
    await screen.findByRole('heading', { name: 'Dashboard' })
    await user.click(screen.getByRole('button', { name: /Skills/ }))
    await user.click(screen.getByRole('button', { name: 'claude alpha-skill: ON' }))
    await user.click(screen.getByRole('button', { name: 'Save as set' }))

    const dialog = screen.getByRole('dialog', { name: 'New Skill Set' })
    expect(within(dialog).getByRole('checkbox', { name: /alpha-skill/ })).toBeChecked()
    await user.type(within(dialog).getByRole('textbox', { name: 'Name' }), 'Change review')
    await user.type(within(dialog).getByRole('textbox', { name: /When to use/ }), 'Use for risky cross-tool changes.')
    await user.click(within(dialog).getByRole('button', { name: 'Save Skill Set' }))
    await waitFor(() => expect(backend.createSkillSet).toHaveBeenCalledWith('Change review', 'Use for risky cross-tool changes.', ['alpha-skill']))
  })

  it('adds a skill to an existing Skill Set through skill details', async () => {
    const user = userEvent.setup()
    const backend = mockBackend()
    render(<App backend={backend} />)
    await screen.findByRole('heading', { name: 'Dashboard' })
    await user.click(screen.getByRole('button', { name: /Skills/ }))
    await user.click(screen.getByText('alpha-skill'))
    await user.click(screen.getByRole('button', { name: 'Add to Skill Set…' }))

    const picker = screen.getByRole('dialog', { name: 'Add alpha-skill to a Skill Set' })
    await user.click(within(picker).getByRole('button', { name: /Review support/ }))
    const editor = screen.getByRole('dialog', { name: 'Edit Skill Set' })
    expect(within(editor).getByRole('checkbox', { name: /alpha-skill/ })).toBeChecked()
    await user.click(within(editor).getByRole('button', { name: 'Save Skill Set' }))
    await waitFor(() => expect(backend.updateSkillSet).toHaveBeenCalledWith('set:review-support', 'Review support', 'Use when reviewing a cross-tool change.', ['alpha-skill']))
  })

  it('shows managed sources and confirms a repository update', async () => {
    const user = userEvent.setup()
    const backend = mockBackend()
    render(<App backend={backend} />)
    await screen.findByRole('heading', { name: 'Dashboard' })

    await user.click(screen.getByRole('button', { name: /Sources/ }))
    expect(screen.getByRole('heading', { name: 'Sources' })).toBeInTheDocument()
    expect(screen.getByText('demo/skills')).toBeInTheDocument()
    expect(screen.getByText('local-skills')).toBeInTheDocument()
    expect(screen.getByRole('columnheader', { name: 'Update mode' })).toBeInTheDocument()
    expect(screen.getByText('Managed Git')).toBeInTheDocument()
    expect(screen.getByText('Use Update to fetch changes.')).toBeInTheDocument()
    expect(screen.getByText('Linked folder')).toBeInTheDocument()
    expect(screen.getByText('Changes are read directly; no update needed.')).toBeInTheDocument()

    await user.click(screen.getByRole('button', { name: 'Update demo/skills' }))
    await user.click(within(screen.getByRole('dialog')).getByRole('button', { name: /^Update$/ }))
    await waitFor(() => expect(backend.updateSource).toHaveBeenCalledWith('git:fixture', false))
  })

  it('inspects Git and reviews an exact install matrix', async () => {
    const user = userEvent.setup()
    const backend = mockBackend()
    render(<App backend={backend} />)
    await screen.findByRole('heading', { name: 'Dashboard' })
    await user.click(screen.getByRole('button', { name: /Sources/ }))
    await user.click(screen.getByRole('button', { name: /^Install source$/ }))

    await user.type(screen.getByRole('textbox', { name: 'HTTPS or SSH Git URL' }), 'https://github.com/demo/skills')
    await user.click(screen.getByRole('button', { name: 'Clone & inspect' }))
    expect(await screen.findByText('alpha')).toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: 'Review 4 targets' }))
    await waitFor(() => expect(backend.reviewInstall).toHaveBeenCalled())
    expect(await screen.findByText('Ready to install')).toBeInTheDocument()
  })

  it('bulk-selects complete install columns independently of the text filter', async () => {
    const user = userEvent.setup()
    const backend = mockBackend()
    backend.prepareGitInstall = vi.fn(async () => new gui.InstallDraft({
      draftId: 'draft:bulk',
      kind: 'git',
      group: 'demo/skills',
      location: 'https://github.com/demo/skills',
      candidates: [
        installCandidate('alpha'),
        installCandidate('beta', 'available', 'conflict'),
        installCandidate('gamma', 'conflict', 'available'),
      ],
      cloned: false,
      reused: true,
      retainedClone: false,
      cancelled: false,
    }))
    let finishReview: (() => void) | undefined
    backend.reviewInstall = vi.fn((draftId, selections) => new Promise<gui.InstallReview>((resolve) => {
      finishReview = () => resolve(new gui.InstallReview({ reviewId: 'review:bulk', draftId, group: 'demo/skills', selections, createCount: selections.length, alreadyOnCount: 0, alreadyOffCount: 0, conflicts: [], ready: true }))
    }))

    render(<App backend={backend} />)
    await screen.findByRole('heading', { name: 'Dashboard' })
    await user.click(screen.getByRole('button', { name: /Sources/ }))
    await user.click(screen.getByRole('button', { name: /^Install source$/ }))
    await user.type(screen.getByRole('textbox', { name: 'HTTPS or SSH Git URL' }), 'https://github.com/demo/skills')
    await user.click(screen.getByRole('button', { name: 'Clone & inspect' }))

    expect(await screen.findByRole('button', { name: 'Claude all targets: ON (2 of 2 selected)' })).toHaveAttribute('aria-pressed', 'true')
    expect(screen.getByRole('button', { name: 'Codex all targets: ON (2 of 2 selected)' })).toHaveAttribute('aria-pressed', 'true')
    expect(screen.getByRole('button', { name: 'Muse all targets: ON (3 of 3 selected)' })).toHaveAttribute('aria-pressed', 'true')
    expect(screen.getByRole('button', { name: 'Grok all targets: ON (3 of 3 selected)' })).toHaveAttribute('aria-pressed', 'true')

    await user.click(screen.getByRole('checkbox', { name: 'alpha claude' }))
    expect(screen.getByRole('button', { name: 'Claude all targets: MIXED (1 of 2 selected)' })).toHaveAttribute('aria-pressed', 'mixed')

    await user.type(screen.getByRole('textbox', { name: 'Filter discovered skills' }), 'alpha')
    expect(screen.queryByText('beta')).not.toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: 'Claude all targets: MIXED (1 of 2 selected)' }))
    await user.click(screen.getByRole('button', { name: 'Claude all targets: ON (2 of 2 selected)' }))
    expect(screen.getByRole('button', { name: 'Claude all targets: OFF (0 of 2 selected)' })).toHaveAttribute('aria-pressed', 'false')
    await user.click(screen.getByRole('button', { name: 'Claude all targets: OFF (0 of 2 selected)' }))
    await user.click(screen.getByRole('button', { name: 'Codex all targets: ON (2 of 2 selected)' }))

    expect(screen.getByRole('button', { name: 'Claude all targets: ON (2 of 2 selected)' })).toHaveAttribute('aria-pressed', 'true')
    expect(screen.getByRole('button', { name: 'Codex all targets: OFF (0 of 2 selected)' })).toHaveAttribute('aria-pressed', 'false')
    expect(screen.getByRole('button', { name: 'Muse all targets: ON (3 of 3 selected)' })).toHaveAttribute('aria-pressed', 'true')
    expect(screen.getByRole('button', { name: 'Grok all targets: ON (3 of 3 selected)' })).toHaveAttribute('aria-pressed', 'true')
    await user.click(screen.getByRole('button', { name: 'Review 8 targets' }))
    await waitFor(() => expect(backend.reviewInstall).toHaveBeenCalledWith('draft:bulk', expect.arrayContaining([
      { skillName: 'alpha', tool: 'claude' },
      { skillName: 'beta', tool: 'claude' },
    ])))
    expect(screen.getByRole('textbox', { name: 'Filter discovered skills' })).toBeDisabled()
    expect(screen.getByRole('button', { name: 'Claude all targets: ON (2 of 2 selected)' })).toBeDisabled()
    expect(screen.getByRole('checkbox', { name: 'alpha claude' })).toBeDisabled()

    finishReview?.()
    expect(await screen.findByText('Ready to install')).toBeInTheDocument()
  })

  it('disables a column toggle when every target conflicts', async () => {
    const user = userEvent.setup()
    const backend = mockBackend()
    backend.prepareGitInstall = vi.fn(async () => new gui.InstallDraft({
      draftId: 'draft:conflict',
      kind: 'git',
      group: 'demo/skills',
      location: 'https://github.com/demo/skills',
      candidates: [installCandidate('alpha', 'available', 'conflict', 'conflict', 'conflict')],
      cloned: false,
      reused: true,
      retainedClone: false,
      cancelled: false,
    }))

    render(<App backend={backend} />)
    await screen.findByRole('heading', { name: 'Dashboard' })
    await user.click(screen.getByRole('button', { name: /Sources/ }))
    await user.click(screen.getByRole('button', { name: /^Install source$/ }))
    await user.type(screen.getByRole('textbox', { name: 'HTTPS or SSH Git URL' }), 'https://github.com/demo/skills')
    await user.click(screen.getByRole('button', { name: 'Clone & inspect' }))

    expect(await screen.findByRole('button', { name: 'Codex all targets: N/A (no available targets)' })).toBeDisabled()
    expect(screen.getByRole('button', { name: 'Muse all targets: N/A (no available targets)' })).toBeDisabled()
    expect(screen.getByRole('button', { name: 'Grok all targets: N/A (no available targets)' })).toBeDisabled()
    expect(screen.getByRole('checkbox', { name: 'alpha codex' })).toBeDisabled()
    expect(screen.getByRole('button', { name: 'Review 1 target' })).toBeEnabled()
  })

  it('extends managed sources to the preselected tool after preview', async () => {
    const user = userEvent.setup()
    const backend = mockBackend()
    render(<App backend={backend} />)
    await screen.findByRole('heading', { name: 'Dashboard' })
    await user.click(screen.getByRole('button', { name: /Sources/ }))

    const extendButton = screen.getByRole('button', { name: 'Extend to tool' })
    expect(extendButton).toBeEnabled()
    await user.click(extendButton)

    const dialog = screen.getByRole('dialog')
    expect(within(dialog).getByRole('radio', { name: 'Codex' })).toBeChecked()
    await waitFor(() => expect(backend.previewExtend).toHaveBeenCalledWith('codex'))
    expect(await within(dialog).findByText('Ready to extend')).toBeInTheDocument()

    await user.click(within(dialog).getByRole('radio', { name: 'Muse' }))
    await waitFor(() => expect(backend.previewExtend).toHaveBeenCalledWith('muse'))

    await user.click(within(dialog).getByRole('button', { name: 'Extend to Muse' }))
    await waitFor(() => expect(backend.extendSources).toHaveBeenCalledWith('muse', false))
    await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument())
  })

  it('keeps the extend dialog open on preview errors and stop-at-first-failure finish', async () => {
    const user = userEvent.setup()
    const backend = mockBackend()
    const preview = new gui.ExtendPreview({ tool: 'codex', sources: [new gui.ExtendPreviewSource({ kind: 'git', group: 'demo/skills', skillNames: ['alpha'], skillCount: 1, created: 1, alreadyInstalled: 0, disabledAfter: 0, status: 'ready', reason: '', skipped: [], conflicts: [] })], createCount: 1, blockedCount: 0 })
    backend.previewExtend = vi.fn()
      .mockRejectedValueOnce(new Error('unknown tool "orb" (supported: claude, codex, muse)'))
      .mockResolvedValue(preview)
    render(<App backend={backend} />)
    await screen.findByRole('heading', { name: 'Dashboard' })
    await user.click(screen.getByRole('button', { name: /Sources/ }))
    await user.click(screen.getByRole('button', { name: 'Extend to tool' }))

    const dialog = screen.getByRole('dialog')
    expect(await within(dialog).findByText(/unknown tool/)).toBeInTheDocument()
    expect(within(dialog).getByRole('button', { name: /Extend to / })).toBeDisabled()

    await user.click(within(dialog).getByRole('button', { name: 'Reset tool' }))
    expect(await within(dialog).findByText('Ready to extend')).toBeInTheDocument()
    expect(within(dialog).getByRole('button', { name: 'Extend to Codex' })).toBeEnabled()

    backend.extendSources = vi.fn(async () => new gui.SourceMutationResult({
      message: '0 source(s) extended to codex: 0 created, 0 already installed.',
      completed: [],
      createdLinks: 0,
      alreadyInstalled: 0,
      failure: new gui.SourceMutationFailure({ stage: 'extend', group: 'demo/skills', message: 'extend --tool codex failed for source demo/skills: target path already exists' }),
      snapshot: fixtureSnapshot(),
    }))
    await user.click(within(dialog).getByRole('button', { name: 'Extend to Codex' }))
    expect(await within(dialog).findByText(/failed for source demo\/skills/)).toBeInTheDocument()
    expect(screen.getByRole('dialog')).toBeInTheDocument()
  })

  it('surfaces blocked sources and disables extend confirm without new links', async () => {
    const user = userEvent.setup()
    const backend = mockBackend()
    backend.previewExtend = vi.fn(async (tool) => new gui.ExtendPreview({
      tool,
      sources: [new gui.ExtendPreviewSource({
        kind: 'git',
        group: 'demo/skills',
        skillNames: [],
        skillCount: 0,
        created: 0,
        alreadyInstalled: 0,
        disabledAfter: 0,
        status: 'blocked',
        reason: '',
        skipped: [],
        conflicts: [new gui.InstallConflict({ skillName: 'alpha', tool: 'codex', reason: 'target path already exists', path: '/tmp/blocker' })],
      })],
      createCount: 0,
      blockedCount: 1,
    }))
    render(<App backend={backend} />)
    await screen.findByRole('heading', { name: 'Dashboard' })
    await user.click(screen.getByRole('button', { name: /Sources/ }))
    await user.click(screen.getByRole('button', { name: 'Extend to tool' }))

    const dialog = screen.getByRole('dialog')
    expect(await within(dialog).findByText('1 blocked source')).toBeInTheDocument()
    expect(within(dialog).getByText(/alpha: target path already exists/)).toBeInTheDocument()
    expect(within(dialog).getByRole('button', { name: 'Extend to Codex' })).toBeDisabled()
    expect(backend.extendSources).not.toHaveBeenCalled()
  })

  it('disables the extend button when every source already uses all four tools', async () => {
    const user = userEvent.setup()
    const snapshot = fixtureSnapshot()
    snapshot.managedSources = snapshot.managedSources.map((source) => ({ ...source, claudeCount: source.skillCount, codexCount: source.skillCount, museCount: source.skillCount, grokCount: source.skillCount }))
    const backend = mockBackend(snapshot)
    render(<App backend={backend} />)
    await screen.findByRole('heading', { name: 'Dashboard' })
    await user.click(screen.getByRole('button', { name: /Sources/ }))
    expect(screen.getByRole('button', { name: 'Extend to tool' })).toBeDisabled()
  })

  it('requires the exact group name before uninstall', async () => {
    const user = userEvent.setup()
    const backend = mockBackend()
    render(<App backend={backend} />)
    await screen.findByRole('heading', { name: 'Dashboard' })
    await user.click(screen.getByRole('button', { name: /Sources/ }))
    await user.click(screen.getByRole('button', { name: 'Uninstall demo/skills' }))

    const confirm = await screen.findByRole('button', { name: 'Uninstall source' })
    expect(confirm).toBeDisabled()
    await user.type(screen.getByRole('textbox', { name: /Type demo\/skills/ }), 'demo/skills')
    expect(confirm).toBeEnabled()
  })

  it('warns without blocking when uninstall affects saved Skill Sets', async () => {
    const user = userEvent.setup()
    const backend = mockBackend()
    backend.previewUninstall = vi.fn(async (sourceId) => new gui.UninstallPreview({
      sourceId, kind: 'git', group: 'demo/skills', location: '/tmp/demo', activeLinks: 2, disabledLinks: 0,
      removesCheckout: true, preservesSource: false, skillSetImpactWarning: '',
      affectedSkillSets: [{ setId: 'set:review-support', name: 'Review support', skills: ['alpha-skill'] }],
    }))
    render(<App backend={backend} />)
    await screen.findByRole('heading', { name: 'Dashboard' })
    await user.click(screen.getByRole('button', { name: /Sources/ }))
    await user.click(screen.getByRole('button', { name: 'Uninstall demo/skills' }))

    expect(await screen.findByText('1 Skill Set contains skills installed by this source')).toBeInTheDocument()
    expect(screen.getByText('Review support: alpha-skill')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Uninstall source' })).toBeDisabled()
  })

  it('warns without pruning when uninstall affects favorites', async () => {
    const user = userEvent.setup()
    const backend = mockBackend()
    backend.previewUninstall = vi.fn(async (sourceId) => new gui.UninstallPreview({
      sourceId, kind: 'git', group: 'demo/skills', location: '/tmp/demo', activeLinks: 2, disabledLinks: 0,
      removesCheckout: true, preservesSource: false, skillSetImpactWarning: '', affectedSkillSets: [],
      affectedFavorites: ['alpha-skill'], favoriteImpactWarning: '',
    }))
    render(<App backend={backend} />)
    await screen.findByRole('heading', { name: 'Dashboard' })
    await user.click(screen.getByRole('button', { name: /Sources/ }))
    await user.click(screen.getByRole('button', { name: 'Uninstall demo/skills' }))

    expect(await screen.findByText('1 favorite skill is installed by this source')).toBeInTheDocument()
    expect(screen.getByText('Favorites are remembered; removed skills may become unavailable until reinstalled.')).toBeInTheDocument()
  })

  /* Discover UI regression scenarios remain as implementation notes while the
     experimental catalog is excluded from the public preview build.

  it('browses the skills.sh rankings and debounces catalog search', async () => {
    const user = userEvent.setup()
    const backend = mockBackend()
    render(<App backend={backend} />)
    await screen.findByRole('heading', { name: 'Dashboard' })

    await user.click(screen.getByRole('button', { name: 'Discover' }))
    expect(await screen.findByRole('heading', { name: 'Discover' })).toBeInTheDocument()
    await waitFor(() => expect(backend.getDiscoverPage).toHaveBeenCalledWith('all-time', 0, false))
    expect(screen.getByRole('tab', { name: 'All Time' })).toHaveAttribute('aria-selected', 'true')
    expect(screen.getByText('demo/skills')).toBeInTheDocument()
    expect(screen.getAllByText('Available').length).toBeGreaterThan(0)

    const search = screen.getByRole('textbox', { name: 'Search the skills.sh catalog' })
    await user.type(search, 'react')
    await waitFor(() => expect(backend.searchDiscover).toHaveBeenCalledWith('react'), { timeout: 1000 })
  })

  it('ignores an older catalog search response that resolves last', async () => {
    const user = userEvent.setup()
    const backend = mockBackend()
    let resolveOld: ((value: gui.DiscoverPage) => void) | undefined
    backend.searchDiscover = vi.fn((query: string) => {
      if (query === 'react') return new Promise<gui.DiscoverPage>((resolve) => { resolveOld = resolve })
      return Promise.resolve(new gui.DiscoverPage({ view: 'search', page: 0, total: 1, hasMore: false, fetchedAt: '2026-08-12T10:00:00Z', offline: false, fromCache: false, skills: [new gui.DiscoverSkill({ id: 'new/skills/new-result', skillId: 'new-result', name: 'new-result', source: 'new/skills', installs: 1, sourceType: 'github', url: 'https://www.skills.sh/new/skills/new-result', installable: true, claude: { tool: 'claude', status: 'available' }, codex: { tool: 'codex', status: 'available' } })] }))
    })
    render(<App backend={backend} />)
    await screen.findByRole('heading', { name: 'Dashboard' })
    await user.click(screen.getByRole('button', { name: 'Discover' }))
    const search = screen.getByRole('textbox', { name: 'Search the skills.sh catalog' })
    await user.type(search, 'react')
    await waitFor(() => expect(backend.searchDiscover).toHaveBeenCalledWith('react'))
    await user.type(search, ' native')
    await waitFor(() => expect(screen.getByText('new-result')).toBeInTheDocument(), { timeout: 1000 })
    resolveOld?.(new gui.DiscoverPage({ view: 'search', page: 0, total: 1, hasMore: false, fetchedAt: '2026-08-12T10:00:00Z', offline: false, fromCache: false, skills: [new gui.DiscoverSkill({ id: 'old/skills/old-result', skillId: 'old-result', name: 'old-result', source: 'old/skills', installs: 1, sourceType: 'github', url: 'https://www.skills.sh/old/skills/old-result', installable: true, claude: { tool: 'claude', status: 'available' }, codex: { tool: 'codex', status: 'available' } })] }))
    await Promise.resolve()
    expect(screen.queryByText('old-result')).not.toBeInTheDocument()
  })

  it('shows well-known details but keeps installation external', async () => {
    const user = userEvent.setup()
    const backend = mockBackend()
    render(<App backend={backend} />)
    await screen.findByRole('heading', { name: 'Dashboard' })
    await user.click(screen.getByRole('button', { name: 'Discover' }))
    await screen.findByText('example.com')

    await user.click(screen.getByText('provider-skill'))
    expect(await screen.findByText(/supported in the next installation scope/)).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Install' })).toBeDisabled()
    expect(backend.installDiscoverSkill).not.toHaveBeenCalled()
  })

  it('keeps an offline catalog browsable but disables installation', async () => {
    const user = userEvent.setup()
    const backend = mockBackend()
    backend.getDiscoverPage = vi.fn(async () => new gui.DiscoverPage({ view: 'all-time', page: 0, total: 1, hasMore: false, fetchedAt: '2026-08-12T10:00:00Z', offline: true, fromCache: true, skills: [new gui.DiscoverSkill({ id: 'demo/skills/alpha', skillId: 'alpha', name: 'alpha', source: 'demo/skills', installs: 1, sourceType: 'github', url: 'https://www.skills.sh/demo/skills/alpha', installable: true, claude: { tool: 'claude', status: 'available' }, codex: { tool: 'codex', status: 'available' } })] }))
    render(<App backend={backend} />)
    await screen.findByRole('heading', { name: 'Dashboard' })
    await user.click(screen.getByRole('button', { name: 'Discover' }))
    expect(await screen.findByText(/Offline cache/)).toBeInTheDocument()
    await user.click(screen.getByText('alpha'))
    expect(await screen.findByText(/New installation is disabled/)).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Install' })).toBeDisabled()
  })

  it('confirms risk and installs one catalog skill for selected agents', async () => {
    const user = userEvent.setup()
    const backend = mockBackend()
    render(<App backend={backend} />)
    await screen.findByRole('heading', { name: 'Dashboard' })
    await user.click(screen.getByRole('button', { name: 'Discover' }))
    await user.click(await screen.findByText('alpha'))
    await screen.findByText('A catalog skill description.')
    await user.click(screen.getByRole('button', { name: 'Install' }))

    const dialog = screen.getByRole('dialog', { name: 'alpha' })
    expect(within(dialog).getByText(/third-party skills/)).toBeInTheDocument()
    expect(within(dialog).getByText(/link only this selected skill/)).toBeInTheDocument()
    expect(within(dialog).getByRole('checkbox', { name: /Claude Code/ })).toBeChecked()
    expect(within(dialog).getByRole('checkbox', { name: /Codex/ })).toBeDisabled()
    await user.click(within(dialog).getByRole('button', { name: 'Install for 1 agent' }))
    await waitFor(() => expect(backend.installDiscoverSkill).toHaveBeenCalledWith('demo/skills/alpha', ['claude'], false))
  })

  it('keeps the confirmation open and shows a rejected catalog install', async () => {
    const user = userEvent.setup()
    const backend = mockBackend()
    backend.installDiscoverSkill = vi.fn(async () => { throw new Error('skills.sh is unavailable') })
    render(<App backend={backend} />)
    await screen.findByRole('heading', { name: 'Dashboard' })
    await user.click(screen.getByRole('button', { name: 'Discover' }))
    await user.click(await screen.findByText('alpha'))
    await screen.findByText('A catalog skill description.')
    await user.click(screen.getByRole('button', { name: 'Install' }))
    await user.click(screen.getByRole('button', { name: 'Install for 1 agent' }))

    expect(await screen.findByRole('alert')).toHaveTextContent('skills.sh is unavailable')
    expect(screen.getByRole('dialog', { name: 'alpha' })).toBeInTheDocument()
  })

  it('blocks catalog installation while visibility changes are pending', async () => {
    const user = userEvent.setup()
    const snapshot = fixtureSnapshot()
    snapshot.pending = [{ tool: 'claude', skillName: 'alpha-skill', operation: 'disable' }] as never
    const backend = mockBackend(snapshot)
    render(<App backend={backend} />)
    await screen.findByRole('heading', { name: 'Dashboard' })
    await user.click(screen.getByRole('button', { name: 'Discover' }))
    await user.click(await screen.findByText('alpha'))
    await screen.findByText('A catalog skill description.')
    await user.click(screen.getByRole('button', { name: 'Install' }))
    expect(screen.getByText(/Apply or clear 1 pending visibility change/)).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Install for 1 agent' })).toBeDisabled()
  })
  */
})

function withAvailableSkills() {
  const snapshot = fixtureSnapshot()
  snapshot.rows.push(skillRow('camera-helper'), skillRow('layout-helper'))
  snapshot.groups.push({ group: 'android/skills', rows: 2, claude: { on: 0, off: 2, conflict: 0, readOnly: 0 }, codex: { on: 0, off: 2, conflict: 0, readOnly: 0 }, sources: ['symlink repo'] } as never)
  snapshot.sources = [...snapshot.sources, 'symlink repo']
  snapshot.stats.managedSkills += 2
  snapshot.stats.claude.off += 2
  snapshot.stats.codex.off += 2
  return snapshot
}

function skillRow(name: string) {
  const makeCell = (tool: string) => new gui.SkillCell({
    tool,
    name,
    displayName: name,
    description: '',
    state: 'OFF',
    effectiveState: 'OFF',
    source: 'symlink repo',
    group: 'android/skills',
    entryType: 'symlink',
    activePath: `/tmp/${tool}/${name}`,
    disabledPath: `/tmp/disabled/${tool}/${name}`,
    skillFilePath: `/tmp/disabled/${tool}/${name}/SKILL.md`,
    symlinkTarget: `/tmp/android-skills/${name}`,
    repoOrigin: 'https://github.com/android/skills.git',
    repoCommit: '1234567890ab',
    readOnly: false,
  })
  return new gui.SkillRow({ name, description: `${name} description`, source: 'symlink repo', group: 'android/skills', claude: makeCell('claude'), codex: makeCell('codex'), muse: makeCell('muse') })
}

function installCandidate(name: string, claudeStatus = 'available', codexStatus = 'available', museStatus = 'available', grokStatus = 'available') {
  return {
    name,
    relativePath: `skills/${name}`,
    claude: { tool: 'claude', status: claudeStatus, message: claudeStatus === 'conflict' ? 'Claude target conflict.' : '' },
    codex: { tool: 'codex', status: codexStatus, message: codexStatus === 'conflict' ? 'Codex target conflict.' : '' },
    muse: { tool: 'muse', status: museStatus, message: museStatus === 'conflict' ? 'Muse target conflict.' : '' },
    grok: { tool: 'grok', status: grokStatus, message: grokStatus === 'conflict' ? 'Grok target conflict.' : '' },
  }
}
