import { projectPending } from './api'
import { fixtureSnapshot } from './test/fixtures'

describe('projectPending', () => {
  it('projects effective state without changing the scanned state', () => {
    const snapshot = fixtureSnapshot()
    const projected = projectPending(snapshot, [
      { tool: 'claude', skillName: 'alpha-skill', operation: 'disable' },
      { tool: 'codex', skillName: 'alpha-skill', operation: 'enable' },
    ] as never, snapshot.contextBudgets)

    expect(projected.rows[0].claude?.state).toBe('ON')
    expect(projected.rows[0].claude?.effectiveState).toBe('OFF')
    expect(projected.rows[0].codex?.state).toBe('OFF')
    expect(projected.rows[0].codex?.effectiveState).toBe('ON')
  })
})
