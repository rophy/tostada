import { Server } from '../api'

interface Props {
  sessions: Record<string, Server>
  onConnect: (name: string) => void
  onStop: (name: string) => void
}

export function SessionList({ sessions, onConnect, onStop }: Props) {
  const entries = Object.entries(sessions)
  if (entries.length === 0) return null

  return (
    <div>
      <h2>Active Sessions</h2>
      <table style={{ width: '100%', borderCollapse: 'collapse' }}>
        <thead>
          <tr>
            <th style={{ textAlign: 'left', padding: '8px', borderBottom: '1px solid #ddd' }}>Name</th>
            <th style={{ textAlign: 'left', padding: '8px', borderBottom: '1px solid #ddd' }}>Status</th>
            <th style={{ textAlign: 'right', padding: '8px', borderBottom: '1px solid #ddd' }}>Actions</th>
          </tr>
        </thead>
        <tbody>
          {entries.map(([name, server]) => (
            <tr key={name}>
              <td style={{ padding: '8px' }}>{name}</td>
              <td style={{ padding: '8px' }}>
                {server.ready ? '✅ Ready' : server.pending ? '⏳ Starting...' : '⚠️ Unknown'}
              </td>
              <td style={{ padding: '8px', textAlign: 'right' }}>
                {server.ready && (
                  <button
                    onClick={() => onConnect(name)}
                    style={{ marginRight: '8px', padding: '4px 12px', cursor: 'pointer' }}
                  >
                    Connect
                  </button>
                )}
                <button
                  onClick={() => onStop(name)}
                  style={{ padding: '4px 12px', cursor: 'pointer', color: '#dc2626' }}
                >
                  Stop
                </button>
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  )
}
