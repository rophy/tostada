import { useEffect, useState } from 'react'
import { Layout, Typography, Button, Tabs, Spin, Space, Result } from 'antd'
import {
  ArrowLeftOutlined, LogoutOutlined,
  UserOutlined, DesktopOutlined, CloudServerOutlined,
} from '@ant-design/icons'
import { Link } from 'react-router-dom'
import { CurrentUser, fetchCurrentUser, logout } from '../api'
import { AdminUsers } from '../components/AdminUsers'
import { AdminDevices } from '../components/AdminDevices'
import { AdminSessions } from '../components/AdminSessions'

const { Header, Content } = Layout
const { Title } = Typography

export function Admin() {
  const [user, setUser] = useState<CurrentUser | null>(null)
  const [authChecked, setAuthChecked] = useState(false)

  useEffect(() => {
    fetchCurrentUser().then(u => {
      setUser(u)
      setAuthChecked(true)
    })
  }, [])

  if (!authChecked) {
    return (
      <div style={{ display: 'flex', justifyContent: 'center', alignItems: 'center', minHeight: '100vh' }}>
        <Spin size="large" />
      </div>
    )
  }

  if (!user) {
    window.location.href = '/api/auth/login'
    return null
  }

  if (!user.isAdmin) {
    return <Result status="403" title="Access Denied" subTitle="Admin access required." extra={<Link to="/"><Button type="primary">Back to Dashboard</Button></Link>} />
  }

  return (
    <Layout style={{ minHeight: '100vh', background: '#f5f5f5' }}>
      <Header style={{
        display: 'flex', justifyContent: 'space-between', alignItems: 'center',
        background: '#fff', padding: '0 24px', boxShadow: '0 1px 4px rgba(0,0,0,0.08)',
      }}>
        <Space>
          <Link to="/">
            <Button type="text" icon={<ArrowLeftOutlined />} />
          </Link>
          <Title level={3} style={{ margin: 0 }}>Admin</Title>
        </Space>
        <Space>
          <span>{user.username}</span>
          <Button type="text" icon={<LogoutOutlined />} onClick={logout}>Logout</Button>
        </Space>
      </Header>
      <Content style={{ maxWidth: 1080, width: '100%', margin: '24px auto', padding: '0 24px' }}>
        <Tabs
          defaultActiveKey="users"
          items={[
            { key: 'users', label: <span><UserOutlined /> Users</span>, children: <AdminUsers /> },
            { key: 'devices', label: <span><DesktopOutlined /> Devices</span>, children: <AdminDevices /> },
            { key: 'sessions', label: <span><CloudServerOutlined /> Sessions</span>, children: <AdminSessions /> },
          ]}
        />
      </Content>
    </Layout>
  )
}
