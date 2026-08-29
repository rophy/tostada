import { useEffect, useState } from 'react'
import {
  Workspace, Server, Device,
  fetchWorkspaces, fetchSessions, fetchCurrentUser, fetchDevices,
  launchWorkspace, stopSession, getConnectURL, getDeviceConnectURL, logout,
} from '../api'
import { WorkspaceCard } from '../components/WorkspaceCard'
import { SessionList } from '../components/SessionList'

export function Dashboard() {
  const [workspaces, setWorkspaces] = useState<Workspace[]>([])
  const [sessions, setSessions] = useState<Record<string, Server>>({})
  const [devices, setDevices] = useState<Device[]>([])
  const [username, setUsername] = useState<string | null>(null)

  const refresh = async () => {
    const [ws, sess, devs] = await Promise.all([fetchWorkspaces(), fetchSessions(), fetchDevices()])
    setWorkspaces(ws)
    setSessions(sess)
    setDevices(devs)
  }

  useEffect(() => {
    fetchCurrentUser().then(setUsername)
    refresh()
  }, [])

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
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
        <h1>Tostada</h1>
        <div>
          {username ? (
            <span>
              {username}{' '}
              <button onClick={logout} style={{ marginLeft: '8px' }}>Logout</button>
            </span>
          ) : (
            <a href="/api/auth/login">Login</a>
          )}
        </div>
      </div>

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

      {devices.length > 0 && (
        <>
          <h2>Devices</h2>
          <table style={{ width: '100%', borderCollapse: 'collapse' }}>
            <thead>
              <tr>
                <th style={{ textAlign: 'left', padding: '8px', borderBottom: '1px solid #ccc' }}>Name</th>
                <th style={{ textAlign: 'left', padding: '8px', borderBottom: '1px solid #ccc' }}>Protocol</th>
                <th style={{ textAlign: 'left', padding: '8px', borderBottom: '1px solid #ccc' }}>Actions</th>
              </tr>
            </thead>
            <tbody>
              {devices.map(d => (
                <tr key={d.name}>
                  <td style={{ padding: '8px', borderBottom: '1px solid #eee' }}>{d.displayName}</td>
                  <td style={{ padding: '8px', borderBottom: '1px solid #eee' }}>{d.protocol.toUpperCase()}</td>
                  <td style={{ padding: '8px', borderBottom: '1px solid #eee' }}>
                    <button onClick={async () => {
                      const url = await getDeviceConnectURL(d.name)
                      window.open(url, '_blank')
                    }}>Connect</button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </>
      )}
    </div>
  )
}
