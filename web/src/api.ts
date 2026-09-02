export interface Workspace {
  name: string
  displayName: string
  description: string
  icon: string
  type: 'jupyterhub' | 'guacamole'
  image: string
  port: number
}

export interface Server {
  name: string
  ready: boolean
  pending: boolean
  url: string
}

export async function fetchWorkspaces(): Promise<Workspace[]> {
  const res = await fetch('/api/workspaces')
  if (res.status === 401) return []
  return res.json()
}

export async function fetchSessions(): Promise<Record<string, Server>> {
  const res = await fetch('/api/sessions')
  if (res.status === 401) return {}
  return res.json()
}

export async function launchWorkspace(workspace: string, serverName: string): Promise<void> {
  await fetch('/api/sessions', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ workspace, serverName }),
  })
}

export async function stopSession(name: string): Promise<void> {
  await fetch(`/api/sessions/${name}`, { method: 'DELETE' })
}

export async function getConnectURL(name: string): Promise<string> {
  const res = await fetch(`/api/sessions/${name}/connect`)
  const data = await res.json()
  return data.url
}

export interface CurrentUser {
  username: string
  isAdmin: boolean
}

export async function fetchCurrentUser(): Promise<CurrentUser | null> {
  const res = await fetch('/api/auth/me')
  if (res.status === 401) return null
  return res.json()
}

export async function logout(): Promise<void> {
  await fetch('/api/auth/logout', { method: 'POST' })
  window.location.href = '/'
}

export interface Device {
  name: string
  displayName: string
  protocol: string
  host: string
  port: number
  username: string
}

export async function fetchDevices(): Promise<Device[]> {
  const res = await fetch('/api/devices')
  if (res.status === 401) return []
  return res.json()
}

// Admin API

export interface AdminUser {
  id: number
  username: string
  isAdmin: boolean
  lastLogin: string
  createdAt: string
}

export interface AdminDevice {
  ID: number
  name: string
  displayName: string
  protocol: string
  host: string
  port: number
  username: string
  password: string
  grants: string[]
}

export interface AdminSession {
  username: string
  serverName: string
  ready: boolean
  url: string
}

export async function adminListUsers(): Promise<AdminUser[]> {
  const res = await fetch('/api/admin/users')
  if (!res.ok) throw new Error(await res.text())
  return res.json()
}

export async function adminUpdateUser(username: string, updates: { isAdmin?: boolean }): Promise<AdminUser> {
  const res = await fetch(`/api/admin/users/${username}`, {
    method: 'PATCH',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(updates),
  })
  if (!res.ok) throw new Error(await res.text())
  return res.json()
}

export async function adminDeleteUser(username: string): Promise<void> {
  const res = await fetch(`/api/admin/users/${username}`, { method: 'DELETE' })
  if (!res.ok) throw new Error(await res.text())
}

export async function adminListDevices(): Promise<AdminDevice[]> {
  const res = await fetch('/api/admin/devices')
  if (!res.ok) throw new Error(await res.text())
  return res.json()
}

export async function adminCreateDevice(device: Omit<AdminDevice, 'ID' | 'grants'>): Promise<AdminDevice> {
  const res = await fetch('/api/admin/devices', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(device),
  })
  if (!res.ok) throw new Error(await res.text())
  return res.json()
}

export async function adminUpdateDevice(name: string, updates: Record<string, unknown>): Promise<void> {
  const res = await fetch(`/api/admin/devices/${name}`, {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(updates),
  })
  if (!res.ok) throw new Error(await res.text())
}

export async function adminDeleteDevice(name: string): Promise<void> {
  const res = await fetch(`/api/admin/devices/${name}`, { method: 'DELETE' })
  if (!res.ok) throw new Error(await res.text())
}

export async function adminGrantAccess(deviceName: string, username: string): Promise<void> {
  const res = await fetch(`/api/admin/devices/${deviceName}/grants`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ username }),
  })
  if (!res.ok) throw new Error(await res.text())
}

export async function adminRevokeAccess(deviceName: string, username: string): Promise<void> {
  const res = await fetch(`/api/admin/devices/${deviceName}/grants/${username}`, { method: 'DELETE' })
  if (!res.ok) throw new Error(await res.text())
}

export async function adminListSessions(): Promise<AdminSession[]> {
  const res = await fetch('/api/admin/sessions')
  if (!res.ok) throw new Error(await res.text())
  return res.json()
}

export async function adminStopSession(username: string, server: string): Promise<void> {
  const res = await fetch(`/api/admin/sessions/${username}/${server}`, { method: 'DELETE' })
  if (!res.ok) throw new Error(await res.text())
}
