import { describe, it, expect, vi, beforeEach } from 'vitest'
import {
  fetchWorkspaces, fetchSessions, fetchCurrentUser,
  fetchDevices, launchWorkspace, stopSession, logout,
} from '../api'

const mockFetch = vi.fn()
global.fetch = mockFetch

describe('api', () => {
  beforeEach(() => {
    vi.resetAllMocks()
  })

  describe('fetchWorkspaces', () => {
    it('returns workspaces on success', async () => {
      const data = [{ name: 'jupyter', displayName: 'Jupyter' }]
      mockFetch.mockResolvedValue({ status: 200, json: () => Promise.resolve(data) })
      const result = await fetchWorkspaces()
      expect(result).toEqual(data)
      expect(mockFetch).toHaveBeenCalledWith('/api/workspaces')
    })

    it('returns empty array on 401', async () => {
      mockFetch.mockResolvedValue({ status: 401 })
      const result = await fetchWorkspaces()
      expect(result).toEqual([])
    })
  })

  describe('fetchSessions', () => {
    it('returns sessions on success', async () => {
      const data = { 'my-session': { ready: true } }
      mockFetch.mockResolvedValue({ status: 200, json: () => Promise.resolve(data) })
      const result = await fetchSessions()
      expect(result).toEqual(data)
    })

    it('returns empty object on 401', async () => {
      mockFetch.mockResolvedValue({ status: 401 })
      const result = await fetchSessions()
      expect(result).toEqual({})
    })
  })

  describe('fetchCurrentUser', () => {
    it('returns username on success', async () => {
      mockFetch.mockResolvedValue({ status: 200, json: () => Promise.resolve({ username: 'alice' }) })
      const result = await fetchCurrentUser()
      expect(result).toBe('alice')
    })

    it('returns null on 401', async () => {
      mockFetch.mockResolvedValue({ status: 401 })
      const result = await fetchCurrentUser()
      expect(result).toBeNull()
    })
  })

  describe('fetchDevices', () => {
    it('returns devices on success', async () => {
      const data = [{ name: 'mac', displayName: 'Mac Desktop' }]
      mockFetch.mockResolvedValue({ status: 200, json: () => Promise.resolve(data) })
      const result = await fetchDevices()
      expect(result).toEqual(data)
    })

    it('returns empty array on 401', async () => {
      mockFetch.mockResolvedValue({ status: 401 })
      const result = await fetchDevices()
      expect(result).toEqual([])
    })
  })

  describe('launchWorkspace', () => {
    it('sends POST with workspace and serverName', async () => {
      mockFetch.mockResolvedValue({ status: 200 })
      await launchWorkspace('jupyter', 'jupyter-abc')
      expect(mockFetch).toHaveBeenCalledWith('/api/sessions', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ workspace: 'jupyter', serverName: 'jupyter-abc' }),
      })
    })
  })

  describe('stopSession', () => {
    it('sends DELETE for the session', async () => {
      mockFetch.mockResolvedValue({ status: 200 })
      await stopSession('my-session')
      expect(mockFetch).toHaveBeenCalledWith('/api/sessions/my-session', { method: 'DELETE' })
    })
  })

  describe('logout', () => {
    it('sends POST to logout endpoint', async () => {
      delete (window as any).location
      ;(window as any).location = { href: '' }
      mockFetch.mockResolvedValue({ status: 200 })
      await logout()
      expect(mockFetch).toHaveBeenCalledWith('/api/auth/logout', { method: 'POST' })
      expect(window.location.href).toBe('/')
    })
  })
})
