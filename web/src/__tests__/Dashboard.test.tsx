import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { Dashboard } from '../pages/Dashboard'
import * as api from '../api'

vi.mock('../api')
const mockedApi = vi.mocked(api)

const workspaces: api.Workspace[] = [
  { name: 'jupyter', displayName: 'Jupyter Notebook', description: 'Python env', icon: 'notebook', type: 'jupyterhub', image: 'jupyter:latest', port: 8888 },
  { name: 'ubuntu', displayName: 'Ubuntu Desktop', description: 'Full desktop', icon: 'desktop', type: 'jupyterhub', image: 'kasmweb/ubuntu:latest', port: 6901 },
]

const devices: api.Device[] = [
  { name: 'mac', displayName: 'Mac Desktop', protocol: 'rdp', host: '10.0.0.1', port: 3389, username: 'admin' },
]

describe('Dashboard', () => {
  beforeEach(() => {
    vi.resetAllMocks()
  })

  it('shows login page when not authenticated', async () => {
    mockedApi.fetchCurrentUser.mockResolvedValue(null)
    render(<Dashboard />)
    await waitFor(() => {
      expect(screen.getByText('Login with OIDC')).toBeInTheDocument()
    })
  })

  it('shows workspaces and devices when authenticated', async () => {
    mockedApi.fetchCurrentUser.mockResolvedValue('alice')
    mockedApi.fetchWorkspaces.mockResolvedValue(workspaces)
    mockedApi.fetchSessions.mockResolvedValue({})
    mockedApi.fetchDevices.mockResolvedValue(devices)

    render(<Dashboard />)
    await waitFor(() => {
      expect(screen.getByText('Jupyter Notebook')).toBeInTheDocument()
      expect(screen.getByText('Ubuntu Desktop')).toBeInTheDocument()
      expect(screen.getByText('Mac Desktop')).toBeInTheDocument()
      expect(screen.getByText('alice')).toBeInTheDocument()
    })
  })

  it('shows device username in the table', async () => {
    mockedApi.fetchCurrentUser.mockResolvedValue('alice')
    mockedApi.fetchWorkspaces.mockResolvedValue([])
    mockedApi.fetchSessions.mockResolvedValue({})
    mockedApi.fetchDevices.mockResolvedValue(devices)

    render(<Dashboard />)
    await waitFor(() => {
      expect(screen.getByText('admin')).toBeInTheDocument()
      expect(screen.getByText('RDP')).toBeInTheDocument()
    })
  })

  it('calls launchWorkspace when Launch is clicked', async () => {
    mockedApi.fetchCurrentUser.mockResolvedValue('alice')
    mockedApi.fetchWorkspaces.mockResolvedValue([workspaces[0]])
    mockedApi.fetchSessions.mockResolvedValue({})
    mockedApi.fetchDevices.mockResolvedValue([])
    mockedApi.launchWorkspace.mockResolvedValue(undefined)

    render(<Dashboard />)
    await waitFor(() => {
      expect(screen.getByText('Launch')).toBeInTheDocument()
    })
    await userEvent.click(screen.getByText('Launch'))
    expect(mockedApi.launchWorkspace).toHaveBeenCalledWith(
      'jupyter',
      expect.stringMatching(/^jupyter-/)
    )
  })

  it('does not show sessions section when empty', async () => {
    mockedApi.fetchCurrentUser.mockResolvedValue('alice')
    mockedApi.fetchWorkspaces.mockResolvedValue(workspaces)
    mockedApi.fetchSessions.mockResolvedValue({})
    mockedApi.fetchDevices.mockResolvedValue([])

    render(<Dashboard />)
    await waitFor(() => {
      expect(screen.getByText('Workspaces')).toBeInTheDocument()
    })
    expect(screen.queryByText('Active Sessions')).not.toBeInTheDocument()
  })

  it('does not show devices section when empty', async () => {
    mockedApi.fetchCurrentUser.mockResolvedValue('alice')
    mockedApi.fetchWorkspaces.mockResolvedValue(workspaces)
    mockedApi.fetchSessions.mockResolvedValue({})
    mockedApi.fetchDevices.mockResolvedValue([])

    render(<Dashboard />)
    await waitFor(() => {
      expect(screen.getByText('Workspaces')).toBeInTheDocument()
    })
    expect(screen.queryByText('Devices')).not.toBeInTheDocument()
  })
})
