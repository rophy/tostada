import { useEffect, useState } from 'react'
import {
  Workspace, Server,
  fetchWorkspaces, fetchSessions,
  launchWorkspace, stopSession, getConnectURL,
} from '../api'
import { WorkspaceCard } from '../components/WorkspaceCard'
import { SessionList } from '../components/SessionList'

export function Dashboard() {
  const [workspaces, setWorkspaces] = useState<Workspace[]>([])
  const [sessions, setSessions] = useState<Record<string, Server>>({})

  const refresh = async () => {
    const [ws, sess] = await Promise.all([fetchWorkspaces(), fetchSessions()])
    setWorkspaces(ws)
    setSessions(sess)
  }

  useEffect(() => { refresh() }, [])

  // Poll while any session is pending
  useEffect(() => {
    const hasPending = Object.values(sessions).some(s => s.pending || !s.ready)
    if (!hasPending || Object.keys(sessions).length === 0) return
    const timer = setInterval(refresh, 2000)
    return () => clearInterval(timer)
  }, [sessions])

  const handleLaunch = async (ws: Workspace) => {
    const name = `${ws.name}-${Date.now().toString(36)}`
    await launchWorkspace(ws.name, name)
    refresh()
  }

  const handleConnect = async (name: string) => {
    const url = await getConnectURL(name)
    window.open(url, '_blank')
  }

  const handleStop = async (name: string) => {
    await stopSession(name)
    refresh()
  }

  return (
    <div style={{ maxWidth: '960px', margin: '0 auto', padding: '20px' }}>
      <h1>Tostada</h1>

      <h2>Workspaces</h2>
      <div style={{
        display: 'grid',
        gridTemplateColumns: 'repeat(auto-fill, minmax(260px, 1fr))',
        gap: '16px',
      }}>
        {workspaces.map(ws => (
          <WorkspaceCard key={ws.name} workspace={ws} onLaunch={handleLaunch} />
        ))}
      </div>

      <SessionList sessions={sessions} onConnect={handleConnect} onStop={handleStop} />
    </div>
  )
}
