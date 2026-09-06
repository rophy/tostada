import { useEffect, useState } from 'react'
import { Table, Button, Popconfirm, Tag, message, Empty, Card, Progress, Space, Typography } from 'antd'
import { StopOutlined } from '@ant-design/icons'
import { AdminSession, ResourceUsage, adminListSessions, adminStopSession, adminFetchQuota } from '../api'

const { Text } = Typography

function parseResource(value: string): number {
  const s = value.trim()
  if (s.endsWith('Gi')) return parseFloat(s) * 1024 * 1024 * 1024
  if (s.endsWith('Mi')) return parseFloat(s) * 1024 * 1024
  if (s.endsWith('Ki')) return parseFloat(s) * 1024
  if (s.endsWith('m')) return parseFloat(s) / 1000
  return parseFloat(s)
}

function formatResource(resource: string, value: string): string {
  if (value === '∞') return '∞'
  if (resource.startsWith('cpu')) {
    const n = parseResource(value)
    return n < 1 ? `${Math.round(n * 1000)}m` : `${n}`
  }
  if (resource.startsWith('memory')) {
    const bytes = parseResource(value)
    if (bytes >= 1024 * 1024 * 1024) return `${(bytes / (1024 * 1024 * 1024)).toFixed(1)}Gi`
    if (bytes >= 1024 * 1024) return `${(bytes / (1024 * 1024)).toFixed(0)}Mi`
    return value
  }
  return value
}

function ResourceCard({ items }: { items: ResourceUsage[] }) {
  if (items.length === 0) return null

  return (
    <Card size="small" title="Namespace Resources" style={{ marginBottom: 16 }}>
      <Space size="large" wrap>
        {items.map(r => {
          const isCapped = r.hard !== '∞'
          const used = parseResource(r.used)
          const hard = isCapped ? parseResource(r.hard) : 0
          const pct = isCapped && hard > 0 ? Math.round((used / hard) * 100) : 0
          const status = pct >= 90 ? 'exception' : pct >= 70 ? 'normal' : 'success'
          return (
            <div key={r.resource} style={{ textAlign: 'center', minWidth: 110 }}>
              {isCapped ? (
                <Progress type="circle" percent={pct} size={60} status={status} />
              ) : (
                <div style={{ width: 60, height: 60, margin: '0 auto', display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
                  <Text style={{ fontSize: 20 }}>{formatResource(r.resource, r.used)}</Text>
                </div>
              )}
              <div style={{ marginTop: 4 }}>
                <Text strong style={{ fontSize: 12 }}>{r.resource}</Text>
              </div>
              <Text type="secondary" style={{ fontSize: 12 }}>
                {formatResource(r.resource, r.used)} / {formatResource(r.resource, r.hard)}
              </Text>
            </div>
          )
        })}
      </Space>
    </Card>
  )
}

export function AdminSessions() {
  const [sessions, setSessions] = useState<AdminSession[]>([])
  const [resources, setResources] = useState<ResourceUsage[]>([])
  const [loading, setLoading] = useState(true)

  const refresh = async () => {
    setLoading(true)
    try {
      const [sessData, resData] = await Promise.all([
        adminListSessions(),
        adminFetchQuota(),
      ])
      setSessions(sessData || [])
      setResources(resData || [])
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

  if (!loading && sessions.length === 0 && resources.length === 0) {
    return <Empty description="No active sessions" />
  }

  return (
    <>
      <ResourceCard items={resources} />
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
    </>
  )
}
