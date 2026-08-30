import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, it, expect, vi } from 'vitest'
import { SessionList } from '../components/SessionList'

describe('SessionList', () => {
  it('renders nothing when there are no sessions', () => {
    const { container } = render(
      <SessionList sessions={{}} onConnect={() => {}} onStop={() => {}} />
    )
    expect(container.innerHTML).toBe('')
  })

  it('renders session names and status', () => {
    const sessions = {
      'jupyter-abc': { name: 'jupyter-abc', ready: true, pending: false, url: '/user/alice/jupyter-abc/' },
      'ubuntu-def': { name: 'ubuntu-def', ready: false, pending: true, url: '' },
    }
    render(<SessionList sessions={sessions} onConnect={() => {}} onStop={() => {}} />)
    expect(screen.getByText('jupyter-abc')).toBeInTheDocument()
    expect(screen.getByText('ubuntu-def')).toBeInTheDocument()
    expect(screen.getByText('Ready')).toBeInTheDocument()
    expect(screen.getByText('Starting...')).toBeInTheDocument()
  })

  it('shows Connect button only for ready sessions', () => {
    const sessions = {
      'ready-one': { name: 'ready-one', ready: true, pending: false, url: '/url' },
      'pending-one': { name: 'pending-one', ready: false, pending: true, url: '' },
    }
    render(<SessionList sessions={sessions} onConnect={() => {}} onStop={() => {}} />)
    const connectButtons = screen.getAllByText('Connect')
    expect(connectButtons).toHaveLength(1)
  })

  it('calls onConnect with session name', async () => {
    const onConnect = vi.fn()
    const sessions = {
      'my-session': { name: 'my-session', ready: true, pending: false, url: '/url' },
    }
    render(<SessionList sessions={sessions} onConnect={onConnect} onStop={() => {}} />)
    await userEvent.click(screen.getByText('Connect'))
    expect(onConnect).toHaveBeenCalledWith('my-session')
  })

  it('calls onStop with session name', async () => {
    const onStop = vi.fn()
    const sessions = {
      'my-session': { name: 'my-session', ready: true, pending: false, url: '/url' },
    }
    render(<SessionList sessions={sessions} onConnect={() => {}} onStop={onStop} />)
    await userEvent.click(screen.getByText('Stop'))
    expect(onStop).toHaveBeenCalledWith('my-session')
  })
})
