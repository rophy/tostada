import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, it, expect, vi } from 'vitest'
import { WorkspaceCard } from '../components/WorkspaceCard'

const workspace = {
  name: 'jupyter',
  displayName: 'Jupyter Notebook',
  description: 'Python data science environment',
  icon: 'notebook',
  type: 'jupyterhub' as const,
  image: 'jupyter/minimal-notebook:latest',
  port: 8888,
}

describe('WorkspaceCard', () => {
  it('renders workspace name and description', () => {
    render(<WorkspaceCard workspace={workspace} onLaunch={() => {}} />)
    expect(screen.getByText('Jupyter Notebook')).toBeInTheDocument()
    expect(screen.getByText('Python data science environment')).toBeInTheDocument()
  })

  it('calls onLaunch when Launch button is clicked', async () => {
    const onLaunch = vi.fn()
    render(<WorkspaceCard workspace={workspace} onLaunch={onLaunch} />)
    await userEvent.click(screen.getByText('Launch'))
    expect(onLaunch).toHaveBeenCalledWith(workspace)
  })

  it('renders the correct icon for known types', () => {
    render(<WorkspaceCard workspace={workspace} onLaunch={() => {}} />)
    expect(screen.getByText('\u{1F4D3}')).toBeInTheDocument()
  })

  it('renders fallback icon for unknown types', () => {
    const ws = { ...workspace, icon: 'unknown' }
    render(<WorkspaceCard workspace={ws} onLaunch={() => {}} />)
    expect(screen.getByText('\u{1F4E6}')).toBeInTheDocument()
  })
})
