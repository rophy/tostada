import { useEffect, useState } from 'react'
import { Table, Switch, Button, Popconfirm, message, Tag } from 'antd'
import { DeleteOutlined } from '@ant-design/icons'
import { AdminUser, adminListUsers, adminUpdateUser, adminDeleteUser } from '../api'

export function AdminUsers() {
  const [users, setUsers] = useState<AdminUser[]>([])
  const [loading, setLoading] = useState(true)

  const refresh = async () => {
    setLoading(true)
    try {
      setUsers(await adminListUsers())
    } catch (e) {
      message.error('Failed to load users')
    }
    setLoading(false)
  }

  useEffect(() => { refresh() }, [])

  const toggleAdmin = async (username: string, isAdmin: boolean) => {
    try {
      await adminUpdateUser(username, { isAdmin })
      refresh()
    } catch (e) {
      message.error(String(e))
    }
  }

  const deleteUser = async (username: string) => {
    try {
      await adminDeleteUser(username)
      message.success(`User ${username} deleted`)
      refresh()
    } catch (e) {
      message.error(String(e))
    }
  }

  return (
    <Table
      dataSource={users.map(u => ({ ...u, key: u.username }))}
      loading={loading}
      pagination={false}
      size="middle"
      columns={[
        { title: 'Username', dataIndex: 'username', key: 'username' },
        {
          title: 'Role', key: 'role',
          render: (_, u) => u.isAdmin ? <Tag color="blue">Admin</Tag> : <Tag>User</Tag>,
        },
        {
          title: 'Admin', key: 'admin', width: 80,
          render: (_, u) => (
            <Switch
              checked={u.isAdmin}
              onChange={(checked) => toggleAdmin(u.username, checked)}
              size="small"
            />
          ),
        },
        {
          title: 'Last Login', key: 'lastLogin',
          render: (_, u) => u.lastLogin ? new Date(u.lastLogin).toLocaleString() : 'Never',
        },
        {
          title: '', key: 'actions', width: 60, align: 'right' as const,
          render: (_, u) => (
            <Popconfirm title={`Delete user ${u.username}?`} onConfirm={() => deleteUser(u.username)}>
              <Button type="text" danger icon={<DeleteOutlined />} size="small" />
            </Popconfirm>
          ),
        },
      ]}
    />
  )
}
