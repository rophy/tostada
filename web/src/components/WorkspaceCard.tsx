import { Workspace } from '../api'

const iconMap: Record<string, string> = {
  notebook: '\u{1F4D3}',
  desktop: '\u{1F5A5}',
  terminal: '\u{1F4BB}',
}

interface Props {
  workspace: Workspace
  onLaunch: (workspace: Workspace) => void
}

export function WorkspaceCard({ workspace, onLaunch }: Props) {
  return (
    <div style={{
      border: '1px solid #ddd',
      borderRadius: '8px',
      padding: '20px',
      display: 'flex',
      flexDirection: 'column',
      gap: '8px',
    }}>
      <div style={{ fontSize: '2rem' }}>
        {iconMap[workspace.icon] || '\u{1F4E6}'}
      </div>
      <h3 style={{ margin: 0 }}>{workspace.displayName}</h3>
      <p style={{ margin: 0, color: '#666', fontSize: '0.9rem' }}>
        {workspace.description}
      </p>
      <button
        onClick={() => onLaunch(workspace)}
        style={{
          marginTop: 'auto',
          padding: '8px 16px',
          background: '#2563eb',
          color: '#fff',
          border: 'none',
          borderRadius: '4px',
          cursor: 'pointer',
        }}
      >
        Launch
      </button>
    </div>
  )
}
