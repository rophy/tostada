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
  if (res.status === 401) {
    window.location.href = '/api/auth/login'
    return []
  }
  return res.json()
}

export async function fetchSessions(): Promise<Record<string, Server>> {
  const res = await fetch('/api/sessions')
  if (res.status === 401) {
    window.location.href = '/api/auth/login'
    return {}
  }
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

export async function fetchCurrentUser(): Promise<string | null> {
  const res = await fetch('/api/auth/me')
  if (res.status === 401) return null
  const data = await res.json()
  return data.username
}

export async function logout(): Promise<void> {
  await fetch('/api/auth/logout', { method: 'POST' })
  window.location.href = '/'
}
