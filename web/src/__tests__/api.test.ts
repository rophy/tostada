import { describe, it, expect, vi, beforeEach } from 'vitest'
import {
  fetchWorkspaces, fetchSessions, fetchCurrentUser,
  fetchDevices, launchWorkspace, stopSession, logout,
  adminListUsers, adminUpdateUser, adminDeleteUser,
  adminListDevices, adminCreateDevice, adminDeleteDevice,
  adminGrantAccess, adminRevokeAccess,
  adminListSessions, adminStopSession,
  subscribeProgress,
} from '../api'
import { MockEventSource } from '../test-setup'

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
    it('returns user object on success', async () => {
      mockFetch.mockResolvedValue({ status: 200, json: () => Promise.resolve({ username: 'alice', isAdmin: true }) })
      const result = await fetchCurrentUser()
      expect(result).toEqual({ username: 'alice', isAdmin: true })
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

  describe('subscribeProgress', () => {
    beforeEach(() => {
      MockEventSource.instances = []
    })

    it('creates EventSource with correct URL', () => {
      subscribeProgress('my-session', () => {}, () => {})
      expect(MockEventSource.instances).toHaveLength(1)
      expect(MockEventSource.instances[0].url).toBe('/api/sessions/my-session/progress')
    })

    it('calls onEvent for each message', () => {
      const onEvent = vi.fn()
      subscribeProgress('my-session', onEvent, () => {})
      const es = MockEventSource.instances[0]
      es.simulateMessage('{"progress":0,"message":"Server requested"}')
      es.simulateMessage('{"progress":50,"message":"Pulling image"}')
      expect(onEvent).toHaveBeenCalledTimes(2)
      expect(onEvent).toHaveBeenCalledWith({ progress: 0, message: 'Server requested' })
      expect(onEvent).toHaveBeenCalledWith({ progress: 50, message: 'Pulling image' })
    })

    it('calls onDone and closes when ready', () => {
      const onDone = vi.fn()
      const onEvent = vi.fn()
      subscribeProgress('my-session', onEvent, onDone)
      const es = MockEventSource.instances[0]
      const closeSpy = vi.spyOn(es, 'close')
      es.simulateMessage('{"progress":100,"ready":true,"message":"Server ready"}')
      expect(onDone).toHaveBeenCalledTimes(1)
      expect(closeSpy).toHaveBeenCalled()
    })

    it('calls onDone on error', () => {
      const onDone = vi.fn()
      subscribeProgress('my-session', () => {}, onDone)
      const es = MockEventSource.instances[0]
      const closeSpy = vi.spyOn(es, 'close')
      es.simulateError()
      expect(onDone).toHaveBeenCalledTimes(1)
      expect(closeSpy).toHaveBeenCalled()
    })

    it('returns cleanup function that closes EventSource', () => {
      const cleanup = subscribeProgress('my-session', () => {}, () => {})
      const es = MockEventSource.instances[0]
      const closeSpy = vi.spyOn(es, 'close')
      cleanup()
      expect(closeSpy).toHaveBeenCalled()
    })
  })

  describe('admin API', () => {
    describe('adminListUsers', () => {
      it('returns users on success', async () => {
        const data = [{ username: 'alice', isAdmin: true }]
        mockFetch.mockResolvedValue({ ok: true, json: () => Promise.resolve(data) })
        const result = await adminListUsers()
        expect(result).toEqual(data)
        expect(mockFetch).toHaveBeenCalledWith('/api/admin/users')
      })

      it('throws on error', async () => {
        mockFetch.mockResolvedValue({ ok: false, text: () => Promise.resolve('forbidden') })
        await expect(adminListUsers()).rejects.toThrow('forbidden')
      })
    })

    describe('adminUpdateUser', () => {
      it('sends PATCH with updates', async () => {
        mockFetch.mockResolvedValue({ ok: true, json: () => Promise.resolve({ username: 'alice', isAdmin: true }) })
        await adminUpdateUser('alice', { isAdmin: true })
        expect(mockFetch).toHaveBeenCalledWith('/api/admin/users/alice', {
          method: 'PATCH',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ isAdmin: true }),
        })
      })
    })

    describe('adminDeleteUser', () => {
      it('sends DELETE', async () => {
        mockFetch.mockResolvedValue({ ok: true })
        await adminDeleteUser('alice')
        expect(mockFetch).toHaveBeenCalledWith('/api/admin/users/alice', { method: 'DELETE' })
      })
    })

    describe('adminListDevices', () => {
      it('returns devices on success', async () => {
        const data = [{ name: 'mac', grants: ['alice'] }]
        mockFetch.mockResolvedValue({ ok: true, json: () => Promise.resolve(data) })
        const result = await adminListDevices()
        expect(result).toEqual(data)
      })
    })

    describe('adminCreateDevice', () => {
      it('sends POST with device data', async () => {
        const device = { name: 'new', displayName: 'New', protocol: 'rdp', host: '10.0.0.1', port: 3389, username: 'u', password: 'p' }
        mockFetch.mockResolvedValue({ ok: true, json: () => Promise.resolve(device) })
        await adminCreateDevice(device)
        expect(mockFetch).toHaveBeenCalledWith('/api/admin/devices', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify(device),
        })
      })
    })

    describe('adminDeleteDevice', () => {
      it('sends DELETE', async () => {
        mockFetch.mockResolvedValue({ ok: true })
        await adminDeleteDevice('mac')
        expect(mockFetch).toHaveBeenCalledWith('/api/admin/devices/mac', { method: 'DELETE' })
      })
    })

    describe('adminGrantAccess', () => {
      it('sends POST with username', async () => {
        mockFetch.mockResolvedValue({ ok: true })
        await adminGrantAccess('mac', 'bob')
        expect(mockFetch).toHaveBeenCalledWith('/api/admin/devices/mac/grants', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ username: 'bob' }),
        })
      })
    })

    describe('adminRevokeAccess', () => {
      it('sends DELETE', async () => {
        mockFetch.mockResolvedValue({ ok: true })
        await adminRevokeAccess('mac', 'bob')
        expect(mockFetch).toHaveBeenCalledWith('/api/admin/devices/mac/grants/bob', { method: 'DELETE' })
      })
    })

    describe('adminListSessions', () => {
      it('returns sessions on success', async () => {
        const data = [{ username: 'alice', serverName: 'jupyter', ready: true }]
        mockFetch.mockResolvedValue({ ok: true, json: () => Promise.resolve(data) })
        const result = await adminListSessions()
        expect(result).toEqual(data)
      })
    })

    describe('adminStopSession', () => {
      it('sends DELETE', async () => {
        mockFetch.mockResolvedValue({ ok: true })
        await adminStopSession('alice', 'jupyter')
        expect(mockFetch).toHaveBeenCalledWith('/api/admin/sessions/alice/jupyter', { method: 'DELETE' })
      })
    })
  })
})
