import { Card, Button, Typography } from 'antd'
import { RocketOutlined } from '@ant-design/icons'
import { Workspace } from '../api'

const { Text, Paragraph } = Typography

const iconMap: Record<string, string> = {
  notebook: '\u{1F4D3}',
  desktop: '\u{1F5A5}',
  terminal: '\u{1F4BB}',
  browser: '\u{1F310}',
}

interface Props {
  workspace: Workspace
  onLaunch: (workspace: Workspace) => void
}

export function WorkspaceCard({ workspace, onLaunch }: Props) {
  return (
    <Card
      hoverable
      style={{ height: '100%', display: 'flex', flexDirection: 'column' }}
      styles={{ body: { flex: 1 } }}
      actions={[
        <Button
          key="launch"
          type="primary"
          icon={<RocketOutlined />}
          onClick={() => onLaunch(workspace)}
        >
          Launch
        </Button>,
      ]}
    >
      <div style={{ fontSize: '2rem', lineHeight: 1, marginBottom: 8 }}>
        {iconMap[workspace.icon] || '\u{1F4E6}'}
      </div>
      <Text strong style={{ fontSize: '1rem' }}>{workspace.displayName}</Text>
      <Paragraph type="secondary" style={{ marginTop: 4, marginBottom: 0 }}>
        {workspace.description}
      </Paragraph>
    </Card>
  )
}
