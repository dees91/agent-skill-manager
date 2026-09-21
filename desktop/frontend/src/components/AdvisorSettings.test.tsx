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

  it('saves a key once without snapping back to Local', async () => {
    const user = userEvent.setup()
    const backend = mockBackend()
    render(<AdvisorSettings backend={backend} />)
    await screen.findByRole('heading', { name: 'Advisor' })
    await user.click(screen.getByRole('button', { name: 'TypeSafe' }))
    const input = screen.getByLabelText('TypeSafe API key')
    await user.type(input, 'sk-test-key-value')
    await user.click(screen.getByRole('button', { name: 'Save key' }))
    await waitFor(() => expect(backend.setAdvisorKey).toHaveBeenCalledTimes(1))
    expect(backend.setAdvisorKey).toHaveBeenCalledWith('sk-test-key-value')
    expect(screen.getByRole('button', { name: 'TypeSafe' })).toHaveAttribute('aria-pressed', 'true')
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

  it('shows key status and Remove key when a key is stored under Local', async () => {
    const backend = mockBackend()
    vi.mocked(backend.getAdvisorSettings).mockResolvedValueOnce({
      mode: 'local',
      environmentKey: false,
      storedKey: 'present',
      credentialStore: 'memory',
      model: 'jev-1.13.0',
    })
    render(<AdvisorSettings backend={backend} />)
    expect(await screen.findByRole('button', { name: 'Remove key' })).toBeInTheDocument()
    expect(screen.getByText(/Stored key: present/)).toBeInTheDocument()
    expect(screen.queryByLabelText('TypeSafe API key')).not.toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Check connection' })).not.toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Local' })).toHaveAttribute('aria-pressed', 'true')
    expect(screen.getByRole('button', { name: 'TypeSafe' })).toHaveAttribute('aria-pressed', 'false')
  })

  it('shows only the key status for an environment key under Local', async () => {
    const backend = mockBackend()
    vi.mocked(backend.getAdvisorSettings).mockResolvedValueOnce({
      mode: 'local',
      environmentKey: true,
      storedKey: 'absent',
      credentialStore: 'memory',
      model: 'jev-1.13.0',
    })
    render(<AdvisorSettings backend={backend} />)
    expect(await screen.findByText(/Environment key: present/)).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Remove key' })).not.toBeInTheDocument()
    expect(screen.queryByLabelText('TypeSafe API key')).not.toBeInTheDocument()
  })

  it('keeps a typed key on provider save and resets status after removal', async () => {
    const user = userEvent.setup()
    const backend = mockBackend()
    vi.mocked(backend.removeAdvisorKey).mockResolvedValueOnce({
      mode: 'local',
      environmentKey: true,
      storedKey: 'absent',
      credentialStore: 'memory',
      model: 'jev-1.13.0',
    })
    render(<AdvisorSettings backend={backend} />)
    await screen.findByRole('heading', { name: 'Advisor' })
    await user.click(screen.getByRole('button', { name: 'TypeSafe' }))
    await user.type(screen.getByLabelText('TypeSafe API key'), 'sk-typed-not-saved')
    await user.click(screen.getByRole('button', { name: 'Save' }))
    await waitFor(() => expect(backend.saveAdvisorProvider).toHaveBeenCalledWith('typesafe'))
    expect(screen.getByLabelText('TypeSafe API key')).toHaveValue('sk-typed-not-saved')
    await user.click(screen.getByRole('button', { name: 'Check connection' }))
    expect(await screen.findByText(/Check failed: demo_offline/)).toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: 'Remove key' }))
    await waitFor(() => expect(backend.removeAdvisorKey).toHaveBeenCalledTimes(1))
    expect(screen.queryByText(/Check failed/)).not.toBeInTheDocument()
    expect(screen.getByText(/Environment key: present/)).toBeInTheDocument()
    expect(screen.queryByLabelText('TypeSafe API key')).not.toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Local' })).toHaveAttribute('aria-pressed', 'true')
    expect(document.body.innerHTML).not.toContain('sk-typed-not-saved')
  })

  it('shows the reset provider when key removal is refused', async () => {
    const user = userEvent.setup()
    const backend = mockBackend()
    vi.mocked(backend.getAdvisorSettings)
      .mockResolvedValueOnce({ mode: 'typesafe', environmentKey: false, storedKey: 'present', credentialStore: 'memory', model: 'jev-1.13.0' })
      .mockResolvedValueOnce({ mode: 'local', environmentKey: false, storedKey: 'present', credentialStore: 'memory', model: 'jev-1.13.0' })
    vi.mocked(backend.removeAdvisorKey).mockRejectedValueOnce(new Error('credential store denied'))
    render(<AdvisorSettings backend={backend} />)
    await user.click(await screen.findByRole('button', { name: 'Remove key' }))
    expect(await screen.findByRole('alert')).toHaveTextContent('credential store denied')
    await waitFor(() => expect(screen.getByRole('button', { name: 'Local' })).toHaveAttribute('aria-pressed', 'true'))
    expect(screen.getByText(/Stored key: present/)).toBeInTheDocument()
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
