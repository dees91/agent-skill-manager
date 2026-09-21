import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { vi } from 'vitest'
import AdvisorSettings from './AdvisorSettings'
import { mockBackend } from '../test/fixtures'

describe('AdvisorSettings', () => {
  it('defaults to local and uses a password field', async () => {
    const backend = mockBackend()
    render(<AdvisorSettings backend={backend} />)
    expect(await screen.findByRole('heading', { name: 'Advisor' })).toBeInTheDocument()
    expect(backend.getAdvisorSettings).toHaveBeenCalledTimes(1)
    const input = screen.getByLabelText('TypeSafe API key')
    expect(input).toHaveAttribute('type', 'password')
  })

  it('saves a key once and clears the input', async () => {
    const user = userEvent.setup()
    const backend = mockBackend()
    render(<AdvisorSettings backend={backend} />)
    await screen.findByRole('heading', { name: 'Advisor' })
    const input = screen.getByLabelText('TypeSafe API key')
    await user.type(input, 'sk-test-key-value')
    await user.click(screen.getByRole('button', { name: 'Save key' }))
    await waitFor(() => expect(backend.setAdvisorKey).toHaveBeenCalledTimes(1))
    expect(backend.setAdvisorKey).toHaveBeenCalledWith('sk-test-key-value')
    expect(input).toHaveValue('')
    expect(document.body.innerHTML).not.toContain('sk-test-key-value')
    expect(window.localStorage.length).toBe(0)
  })

  it('clears the input on cancel and reports check/remove flows', async () => {
    const user = userEvent.setup()
    const backend = mockBackend()
    render(<AdvisorSettings backend={backend} />)
    await screen.findByRole('heading', { name: 'Advisor' })
    const input = screen.getByLabelText('TypeSafe API key')
    await user.type(input, 'sk-cancel-me')
    await user.click(screen.getByRole('button', { name: 'Cancel' }))
    expect(input).toHaveValue('')
    await user.click(screen.getByRole('button', { name: 'Check connection' }))
    await waitFor(() => expect(backend.checkAdvisorConnection).toHaveBeenCalledTimes(1))
    expect(await screen.findByText(/Check failed: demo_offline/)).toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: 'Remove key' }))
    await waitFor(() => expect(backend.removeAdvisorKey).toHaveBeenCalledTimes(1))
  })

  it('shows an alert on save error', async () => {
    const user = userEvent.setup()
    const backend = mockBackend()
    vi.mocked(backend.saveAdvisorProvider).mockRejectedValueOnce(new Error('could not save'))
    render(<AdvisorSettings backend={backend} />)
    await screen.findByRole('heading', { name: 'Advisor' })
    await user.click(screen.getByRole('button', { name: /TypeSafe/ }))
    await user.click(screen.getByRole('button', { name: 'Save' }))
    expect(await screen.findByRole('alert')).toHaveTextContent('could not save')
  })
})
