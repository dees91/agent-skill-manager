import { render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { vi } from 'vitest'
import AdvisorReceipts from './AdvisorReceipts'
import { gui } from '../../wailsjs/go/models'
import { fixtureSnapshot, mockBackend } from '../test/fixtures'

const HEALTHY = 'aaaaaaaa11111111111111111111111a'
const BLOCKED = 'bbbbbbbb22222222222222222222222b'

function receipts() {
  return [
    new gui.AdvisorReceipt({ receiptId: HEALTHY, tool: 'claude', createdAt: new Date(Date.now() - 3 * 86_400_000).toISOString(), blocked: false, skills: [new gui.AdvisorReceiptSkill({ name: 'alpha', action: 'disable' }), new gui.AdvisorReceiptSkill({ name: 'beta', action: 'release' })] }),
    new gui.AdvisorReceipt({ receiptId: BLOCKED, tool: 'codex', createdAt: new Date(Date.now() - 2 * 3_600_000).toISOString(), blocked: true, cause: 'codex/gamma changed after the advisor enabled it.', skills: [new gui.AdvisorReceiptSkill({ name: 'gamma' })] }),
  ]
}

function setup(pendingCount = 0) {
  const backend = mockBackend()
  backend.listAdvisorReceipts = vi.fn(async () => receipts())
  const onResult = vi.fn()
  const onBusy = vi.fn()
  render(<AdvisorReceipts backend={backend} includeReadOnly={false} pendingCount={pendingCount} busy={false} onBusy={onBusy} onResult={onResult} />)
  return { backend, onResult }
}

describe('AdvisorReceipts', () => {
  it('lists receipts with their age, cleanup actions, and blocked cause', async () => {
    setup()
    const healthy = await screen.findByRole('listitem', { name: /Claude receipt aaaaaaaa/ })
    expect(within(healthy).getByText('3 days ago')).toBeInTheDocument()
    expect(within(healthy).getByText('alpha')).toBeInTheDocument()
    expect(within(healthy).getByText('turns off')).toBeInTheDocument()
    expect(within(healthy).getByText('stays on, shared')).toBeInTheDocument()
    expect(within(healthy).queryByRole('button', { name: /Forget/ })).not.toBeInTheDocument()
    const blocked = screen.getByRole('listitem', { name: /Codex receipt bbbbbbbb/ })
    expect(within(blocked).getByText('2 hours ago')).toBeInTheDocument()
    expect(within(blocked).getByText(/codex\/gamma changed/)).toBeInTheDocument()
    expect(within(blocked).queryByRole('button', { name: /^Clean up/ })).not.toBeInTheDocument()
  })

  it('cleans up every receipt that is not blocked after confirmation', async () => {
    const user = userEvent.setup()
    const { backend, onResult } = setup()
    const remaining = receipts().slice(1)
    backend.cleanupAllAdvisorReceipts = vi.fn(async () => new gui.AdvisorReceiptResult({ message: 'Cleaned up 1 receipt(s); 1 blocked receipt(s) left.', cleaned: [HEALTHY], forgotten: [], receipts: remaining, snapshot: fixtureSnapshot() }))

    await user.click(await screen.findByRole('button', { name: 'Clean up all' }))
    const dialog = screen.getByRole('dialog')
    expect(within(dialog).getByText(/1 blocked receipt is skipped/)).toBeInTheDocument()
    expect(within(dialog).getByText(/open agent session can still use/)).toBeInTheDocument()
    await user.click(within(dialog).getByRole('button', { name: 'Clean up 1 receipt' }))

    await waitFor(() => expect(backend.cleanupAllAdvisorReceipts).toHaveBeenCalledWith(false))
    expect(onResult).toHaveBeenCalled()
    await waitFor(() => expect(screen.queryByRole('listitem', { name: /Claude receipt/ })).not.toBeInTheDocument())
  })

  it('forgets a blocked receipt and says that skill links stay unchanged', async () => {
    const user = userEvent.setup()
    const { backend } = setup()
    backend.forgetAdvisorReceipt = vi.fn(async () => new gui.AdvisorReceiptResult({ message: 'Forgot 1 receipt.', cleaned: [], forgotten: [BLOCKED], receipts: receipts().slice(0, 1), snapshot: fixtureSnapshot() }))

    const blocked = await screen.findByRole('listitem', { name: /Codex receipt bbbbbbbb/ })
    await user.click(within(blocked).getByRole('button', { name: 'Forget' }))
    const dialog = screen.getByRole('dialog')
    expect(within(dialog).getByText(/will not turn these skills off/)).toBeInTheDocument()
    await user.click(within(dialog).getByRole('button', { name: 'Forget receipt' }))

    await waitFor(() => expect(backend.forgetAdvisorReceipt).toHaveBeenCalledWith(BLOCKED, false))
  })

  it('keeps keyboard focus inside the confirmation and returns it to the trigger', async () => {
    const user = userEvent.setup()
    setup()
    const trigger = await screen.findByRole('button', { name: 'Clean up all' })
    await user.click(trigger)
    const dialog = screen.getByRole('dialog')
    const close = within(dialog).getByRole('button', { name: 'Close dialog' })
    const confirm = within(dialog).getByRole('button', { name: 'Clean up 1 receipt' })

    confirm.focus()
    await user.tab()
    expect(close).toHaveFocus()
    await user.tab({ shift: true })
    expect(confirm).toHaveFocus()

    await user.keyboard('{Escape}')
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
    expect(trigger).toHaveFocus()
  })

  it('blocks receipt actions while skill changes are pending', async () => {
    setup(2)
    expect(await screen.findByText(/Apply or clear pending skill changes/)).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Clean up all' })).toBeDisabled()
  })
})
