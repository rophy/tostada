import { useEffect, useState } from 'react'
import { Layout, Typography, Button, Card, Table, Space, Row, Col, Spin } from 'antd'
import {
  LoginOutlined, LogoutOutlined, LinkOutlined,
  DesktopOutlined,
} from '@ant-design/icons'
import {
  Workspace, Server, Device,
  fetchWorkspaces, fetchSessions, fetchCurrentUser, fetchDevices,
  launchWorkspace, stopSession, getConnectURL, getDeviceConnectInfo, logout,
} from '../api'
import { WorkspaceCard } from '../components/WorkspaceCard'
import { SessionList } from '../components/SessionList'

const { Header, Content } = Layout
const { Title } = Typography

export function Dashboard() {
  const [workspaces, setWorkspaces] = useState<Workspace[]>([])
  const [sessions, setSessions] = useState<Record<string, Server>>({})
  const [devices, setDevices] = useState<Device[]>([])
  const [username, setUsername] = useState<string | null>(null)
  const [authChecked, setAuthChecked] = useState(false)

  const refresh = async () => {
    const [ws, sess, devs] = await Promise.all([fetchWorkspaces(), fetchSessions(), fetchDevices()])
    setWorkspaces(ws)
    setSessions(sess)
    setDevices(devs)
  }

  useEffect(() => {
    fetchCurrentUser().then(user => {
      setUsername(user)
      setAuthChecked(true)
      if (user) refresh()
    })
  }, [])

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

  if (!authChecked) {
    return (
      <div style={{ display: 'flex', justifyContent: 'center', alignItems: 'center', minHeight: '100vh' }}>
        <Spin size="large" />
      </div>
    )
  }

  if (!username) {
    return (
      <div style={{ display: 'flex', justifyContent: 'center', alignItems: 'center', minHeight: '100vh' }}>
        <Card style={{ textAlign: 'center', maxWidth: 400 }}>
          <Title level={2}>Tostada</Title>
          <p style={{ margin: '24px 0', color: '#666' }}>
            Sign in to access your workspaces and devices.
          </p>
          <Button type="primary" size="large" icon={<LoginOutlined />} href="/api/auth/login">
            Login with OIDC
          </Button>
        </Card>
      </div>
    )
  }

  const deviceColumns = [
    { title: 'Name', dataIndex: 'displayName', key: 'displayName' },
    { title: 'Host', key: 'host', render: (_: unknown, d: Device) => `${d.host}:${d.port}` },
    { title: 'User', dataIndex: 'username', key: 'username' },
    { title: 'Protocol', key: 'protocol', render: (_: unknown, d: Device) => d.protocol.toUpperCase() },
    {
      title: 'Actions',
      key: 'actions',
      align: 'right' as const,
      render: (_: unknown, d: Device) => (
        <Button
          type="link"
          icon={<LinkOutlined />}
          onClick={async () => {
            const info = await getDeviceConnectInfo(d.name)
            const params = new URLSearchParams({ token: info.token, id: info.connectionId })
            window.open(`/connect.html?${params}`, '_blank')
          }}
        >
          Connect
        </Button>
      ),
    },
  ]

  return (
    <Layout style={{ minHeight: '100vh', background: '#f5f5f5' }}>
      <Header style={{
        display: 'flex', justifyContent: 'space-between', alignItems: 'center',
        background: '#fff', padding: '0 24px', boxShadow: '0 1px 4px rgba(0,0,0,0.08)',
      }}>
        <Title level={3} style={{ margin: 0 }}>Tostada</Title>
        <Space>
          <span>{username}</span>
          <Button type="text" icon={<LogoutOutlined />} onClick={logout}>Logout</Button>
        </Space>
      </Header>
      <Content style={{ maxWidth: 1080, width: '100%', margin: '24px auto', padding: '0 24px' }}>
        <Title level={4}>Workspaces</Title>
        <Row gutter={[16, 16]}>
          {workspaces.map(ws => (
            <Col key={ws.name} xs={24} sm={12} md={8}>
              <WorkspaceCard workspace={ws} onLaunch={handleLaunch} />
            </Col>
          ))}
        </Row>

        {Object.keys(sessions).length > 0 && (
          <>
            <Title level={4} style={{ marginTop: 32 }}>Active Sessions</Title>
            <SessionList sessions={sessions} onConnect={handleConnect} onStop={handleStop} />
          </>
        )}

        {devices.length > 0 && (
          <>
            <Title level={4} style={{ marginTop: 32 }}>
              <DesktopOutlined style={{ marginRight: 8 }} />
              Devices
            </Title>
            <Table
              dataSource={devices.map(d => ({ ...d, key: d.name }))}
              columns={deviceColumns}
              pagination={false}
              size="middle"
            />
          </>
        )}
      </Content>
    </Layout>
  )
}
