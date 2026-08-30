import { Table, Button, Tag, Space } from 'antd'
import { LinkOutlined, StopOutlined } from '@ant-design/icons'
import { Server } from '../api'

interface Props {
  sessions: Record<string, Server>
  onConnect: (name: string) => void
  onStop: (name: string) => void
}

export function SessionList({ sessions, onConnect, onStop }: Props) {
  const entries = Object.entries(sessions)
  if (entries.length === 0) return null

  const dataSource = entries.map(([sessionName, server]) => ({
    ...server,
    key: sessionName,
    name: sessionName,
  }))

  return (
    <Table
      dataSource={dataSource}
      pagination={false}
      size="middle"
      columns={[
        {
          title: 'Name',
          dataIndex: 'name',
          key: 'name',
        },
        {
          title: 'Status',
          key: 'status',
          render: (_, record) =>
            record.ready ? (
              <Tag color="success">Ready</Tag>
            ) : record.pending ? (
              <Tag color="processing">Starting...</Tag>
            ) : (
              <Tag color="warning">Unknown</Tag>
            ),
        },
        {
          title: 'Actions',
          key: 'actions',
          align: 'right' as const,
          render: (_, record) => (
            <Space>
              {record.ready && (
                <Button
                  type="link"
                  icon={<LinkOutlined />}
                  onClick={() => onConnect(record.name)}
                >
                  Connect
                </Button>
              )}
              <Button
                type="link"
                danger
                icon={<StopOutlined />}
                onClick={() => onStop(record.name)}
              >
                Stop
              </Button>
            </Space>
          ),
        },
      ]}
    />
  )
}
