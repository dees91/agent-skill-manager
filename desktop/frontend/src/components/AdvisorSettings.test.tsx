import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { vi } from 'vitest'
import AdvisorSettings from './AdvisorSettings'
import { mockBackend } from '../test/fixtures'

describe('AdvisorSettings', () => {
  it('defaults to local and hides the key field until TypeSafe is selected', async () => {
    const user = userEvent.setup()
    const backend = mockBackend()
    render(<AdvisorSettings backend={backend} />)
    expect(await screen.findByRole('heading', { name: 'Advisor' })).toBeInTheDocument()
    expect(backend.getAdvisorSettings).toHaveBeenCalledTimes(1)
    expect(screen.queryByLabelText('TypeSafe API key')).not.toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: 'TypeSafe' }))
    const input = screen.getByLabelText('TypeSafe API key')
    expect(input).toHaveAttribute('type', 'password')
    await user.click(screen.getByRole('button', { name: 'Local' }))
    expect(screen.queryByLabelText('TypeSafe API key')).not.toBeInTheDocument()
  })

  it('shows the key field when TypeSafe is already saved', async () => {
    const backend = mockBackend()
    vi.mocked(backend.getAdvisorSettings).mockResolvedValueOnce({
      mode: 'typesafe',
      environmentKey: false,
      storedKey: 'absent',
      credentialStore: 'memory',
      model: 'jev-1.13.0',
    })
    render(<AdvisorSettings backend={backend} />)
    expect(await screen.findByLabelText('TypeSafe API key')).toHaveAttribute('type', 'password')
  })

  it('saves a key once and clears the input', async () => {
    const user = userEvent.setup()
    const backend = mockBackend()
    vi.mocked(backend.setAdvisorKey).mockResolvedValue({
      mode: 'typesafe',
      environmentKey: false,
      storedKey: 'present',
      credentialStore: 'memory',
      model: 'jev-1.13.0',
    })
    render(<AdvisorSettings backend={backend} />)
    await screen.findByRole('heading', { name: 'Advisor' })
    await user.click(screen.getByRole('button', { name: 'TypeSafe' }))
    const input = screen.getByLabelText('TypeSafe API key')
    await user.type(input, 'sk-test-key-value')
    await user.click(screen.getByRole('button', { name: 'Save key' }))
    await waitFor(() => expect(backend.setAdvisorKey).toHaveBeenCalledTimes(1))
    expect(backend.setAdvisorKey).toHaveBeenCalledWith('sk-test-key-value')
    expect(screen.getByLabelText('TypeSafe API key')).toHaveValue('')
    expect(document.body.innerHTML).not.toContain('sk-test-key-value')
    expect(window.localStorage.length).toBe(0)
  })

  it('clears the input on cancel and reports check/remove flows', async () => {
    const user = userEvent.setup()
    const backend = mockBackend()
    render(<AdvisorSettings backend={backend} />)
    await screen.findByRole('heading', { name: 'Advisor' })
    await user.click(screen.getByRole('button', { name: 'TypeSafe' }))
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
