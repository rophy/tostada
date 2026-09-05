import { render, screen, waitFor, cleanup } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, it, expect, vi, afterEach } from 'vitest'
import { message } from 'antd'
import { SessionList } from '../components/SessionList'

describe('SessionList', () => {
  afterEach(() => {
    cleanup()
    message.destroy()
  })
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

  it('calls onStop with session name after confirmation', async () => {
    const onStop = vi.fn().mockResolvedValue(undefined)
    const sessions = {
      'my-session': { name: 'my-session', ready: true, pending: false, url: '/url' },
    }
    render(<SessionList sessions={sessions} onConnect={() => {}} onStop={onStop} />)
    await userEvent.click(screen.getByText('Stop'))
    const confirmBtn = await screen.findByRole('button', { name: 'Stop' })
    await userEvent.click(confirmBtn)
    expect(onStop).toHaveBeenCalledWith('my-session')
  })

  it('does not call onStop if confirmation is cancelled', async () => {
    const onStop = vi.fn().mockResolvedValue(undefined)
    const sessions = {
      'my-session': { name: 'my-session', ready: true, pending: false, url: '/url' },
    }
    render(<SessionList sessions={sessions} onConnect={() => {}} onStop={onStop} />)
    await userEvent.click(screen.getByText('Stop'))
    const cancelBtn = await screen.findByRole('button', { name: 'Cancel' })
    await userEvent.click(cancelBtn)
    expect(onStop).not.toHaveBeenCalled()
  })

  it('shows error message when stop fails', async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true })
    const onStop = vi.fn().mockRejectedValue(new Error('hub error'))
    const sessions = {
      'my-session': { name: 'my-session', ready: true, pending: false, url: '/url' },
    }
    const user = userEvent.setup({ advanceTimers: vi.advanceTimersByTime })
    render(<SessionList sessions={sessions} onConnect={() => {}} onStop={onStop} />)
    await user.click(screen.getByText('Stop'))
    const confirmBtn = await screen.findByRole('button', { name: 'Stop' })
    await user.click(confirmBtn)
    await waitFor(() => {
      expect(screen.getByText(/Failed to stop session/)).toBeInTheDocument()
    })
    message.destroy()
    await vi.runAllTimersAsync()
    vi.useRealTimers()
  })

  it('toggles expanded row when status tag is clicked', async () => {
    const sessions = {
      'my-session': { name: 'my-session', ready: true, pending: false, url: '/url' },
    }
    render(<SessionList sessions={sessions} onConnect={() => {}} onStop={vi.fn().mockResolvedValue(undefined)} />)
    const readyTag = screen.getByText('Ready')

    // Expand: click to show the expanded row
    await userEvent.click(readyTag)
    const expandedRow = document.querySelector('.ant-table-expanded-row') as HTMLElement
    expect(expandedRow).toBeInTheDocument()
    expect(expandedRow.style.display).not.toBe('none')

    // Collapse: click again to hide
    await userEvent.click(readyTag)
    await waitFor(() => {
      const row = document.querySelector('.ant-table-expanded-row') as HTMLElement
      expect(row?.style.display).toBe('none')
    })
  })

  it('shows chevron icon on status tags', () => {
    const sessions = {
      'ready-one': { name: 'ready-one', ready: true, pending: false, url: '/url' },
      'pending-one': { name: 'pending-one', ready: false, pending: true, url: '' },
    }
    render(<SessionList sessions={sessions} onConnect={() => {}} onStop={vi.fn().mockResolvedValue(undefined)} />)
    const readyTag = screen.getByText('Ready')
    const startingTag = screen.getByText('Starting...')
    expect(readyTag.querySelector('[aria-label="right"]')).toBeInTheDocument()
    expect(startingTag.querySelector('[aria-label="right"]')).toBeInTheDocument()
  })
})
