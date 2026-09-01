import { useEffect, useState } from 'react'
import { Table, Button, Popconfirm, Tag, message, Empty } from 'antd'
import { StopOutlined } from '@ant-design/icons'
import { AdminSession, adminListSessions, adminStopSession } from '../api'

export function AdminSessions() {
  const [sessions, setSessions] = useState<AdminSession[]>([])
  const [loading, setLoading] = useState(true)

  const refresh = async () => {
    setLoading(true)
    try {
      const data = await adminListSessions()
      setSessions(data || [])
    } catch (e) {
      message.error('Failed to load sessions')
    }
    setLoading(false)
  }

  useEffect(() => { refresh() }, [])

  const handleStop = async (username: string, server: string) => {
    try {
      await adminStopSession(username, server)
      message.success(`Stopped ${server} for ${username}`)
      refresh()
    } catch (e) {
      message.error(String(e))
    }
  }

  if (!loading && sessions.length === 0) {
    return <Empty description="No active sessions" />
  }

  return (
    <Table
      dataSource={sessions.map(s => ({ ...s, key: `${s.username}/${s.serverName}` }))}
      loading={loading}
      pagination={false}
      size="middle"
      columns={[
        { title: 'User', dataIndex: 'username', key: 'username' },
        { title: 'Server', dataIndex: 'serverName', key: 'serverName' },
        {
          title: 'Status', key: 'status',
          render: (_, s) => s.ready ? <Tag color="success">Ready</Tag> : <Tag color="processing">Starting</Tag>,
        },
        {
          title: '', key: 'actions', width: 80, align: 'right' as const,
          render: (_, s) => (
            <Popconfirm title={`Stop ${s.serverName} for ${s.username}?`} onConfirm={() => handleStop(s.username, s.serverName)}>
              <Button type="text" danger icon={<StopOutlined />} size="small">Stop</Button>
            </Popconfirm>
          ),
        },
      ]}
    />
  )
}
