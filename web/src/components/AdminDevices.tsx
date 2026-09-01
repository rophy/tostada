import { useEffect, useState } from 'react'
import { Table, Button, Modal, Form, Input, InputNumber, Select, Tag, Space, Popconfirm, message } from 'antd'
import { PlusOutlined, DeleteOutlined, UserAddOutlined, UserDeleteOutlined } from '@ant-design/icons'
import {
  AdminDevice, adminListDevices, adminCreateDevice, adminDeleteDevice,
  adminGrantAccess, adminRevokeAccess,
} from '../api'

export function AdminDevices() {
  const [devices, setDevices] = useState<AdminDevice[]>([])
  const [loading, setLoading] = useState(true)
  const [addOpen, setAddOpen] = useState(false)
  const [grantOpen, setGrantOpen] = useState<string | null>(null)
  const [grantUsername, setGrantUsername] = useState('')
  const [form] = Form.useForm()

  const refresh = async () => {
    setLoading(true)
    try {
      setDevices(await adminListDevices())
    } catch (e) {
      message.error('Failed to load devices')
    }
    setLoading(false)
  }

  useEffect(() => { refresh() }, [])

  const handleAdd = async () => {
    try {
      const values = await form.validateFields()
      await adminCreateDevice(values)
      message.success(`Device ${values.name} created`)
      setAddOpen(false)
      form.resetFields()
      refresh()
    } catch (e) {
      if (e && typeof e === 'object' && 'errorFields' in e) return
      message.error(String(e))
    }
  }

  const handleDelete = async (name: string) => {
    try {
      await adminDeleteDevice(name)
      message.success(`Device ${name} deleted`)
      refresh()
    } catch (e) {
      message.error(String(e))
    }
  }

  const handleGrant = async () => {
    if (!grantOpen || !grantUsername.trim()) return
    try {
      await adminGrantAccess(grantOpen, grantUsername.trim())
      message.success(`Granted ${grantUsername.trim()} access to ${grantOpen}`)
      setGrantOpen(null)
      setGrantUsername('')
      refresh()
    } catch (e) {
      message.error(String(e))
    }
  }

  const handleRevoke = async (deviceName: string, username: string) => {
    try {
      await adminRevokeAccess(deviceName, username)
      message.success(`Revoked ${username} access to ${deviceName}`)
      refresh()
    } catch (e) {
      message.error(String(e))
    }
  }

  return (
    <>
      <div style={{ marginBottom: 16 }}>
        <Button type="primary" icon={<PlusOutlined />} onClick={() => setAddOpen(true)}>
          Add Device
        </Button>
      </div>

      <Table
        dataSource={devices.map(d => ({ ...d, key: d.name }))}
        loading={loading}
        pagination={false}
        size="middle"
        expandable={{
          expandedRowRender: (d) => (
            <Space wrap>
              {(d.grants || []).map(u => (
                <Tag
                  key={u}
                  closable
                  onClose={(e) => { e.preventDefault(); handleRevoke(d.name, u) }}
                >
                  {u}
                </Tag>
              ))}
              <Button
                type="dashed"
                size="small"
                icon={<UserAddOutlined />}
                onClick={() => { setGrantOpen(d.name); setGrantUsername('') }}
              >
                Grant
              </Button>
            </Space>
          ),
        }}
        columns={[
          { title: 'Name', dataIndex: 'name', key: 'name' },
          { title: 'Display', dataIndex: 'displayName', key: 'displayName' },
          { title: 'Protocol', dataIndex: 'protocol', key: 'protocol', render: (v: string) => v.toUpperCase() },
          { title: 'Host', key: 'host', render: (_, d) => `${d.host}:${d.port}` },
          { title: 'User', dataIndex: 'username', key: 'username' },
          {
            title: 'Grants', key: 'grants',
            render: (_, d) => <Tag>{(d.grants || []).length} users</Tag>,
          },
          {
            title: '', key: 'actions', width: 60, align: 'right' as const,
            render: (_, d) => (
              <Popconfirm title={`Delete device ${d.name}?`} onConfirm={() => handleDelete(d.name)}>
                <Button type="text" danger icon={<DeleteOutlined />} size="small" />
              </Popconfirm>
            ),
          },
        ]}
      />

      <Modal
        title="Add Device"
        open={addOpen}
        onOk={handleAdd}
        onCancel={() => { setAddOpen(false); form.resetFields() }}
        destroyOnClose
      >
        <Form form={form} layout="vertical">
          <Form.Item name="name" label="Name" rules={[{ required: true }]}>
            <Input placeholder="e.g. linux-rdp" />
          </Form.Item>
          <Form.Item name="displayName" label="Display Name" rules={[{ required: true }]}>
            <Input placeholder="e.g. Linux Desktop" />
          </Form.Item>
          <Form.Item name="protocol" label="Protocol" rules={[{ required: true }]} initialValue="rdp">
            <Select options={[{ value: 'rdp', label: 'RDP' }, { value: 'vnc', label: 'VNC' }]} />
          </Form.Item>
          <Form.Item name="host" label="Host" rules={[{ required: true }]}>
            <Input placeholder="e.g. 192.168.1.100" />
          </Form.Item>
          <Form.Item name="port" label="Port" rules={[{ required: true }]} initialValue={3389}>
            <InputNumber style={{ width: '100%' }} />
          </Form.Item>
          <Form.Item name="username" label="Username" rules={[{ required: true }]}>
            <Input />
          </Form.Item>
          <Form.Item name="password" label="Password" rules={[{ required: true }]}>
            <Input.Password />
          </Form.Item>
        </Form>
      </Modal>

      <Modal
        title={`Grant access to ${grantOpen}`}
        open={!!grantOpen}
        onOk={handleGrant}
        onCancel={() => setGrantOpen(null)}
      >
        <Input
          placeholder="Username"
          value={grantUsername}
          onChange={(e) => setGrantUsername(e.target.value)}
          onPressEnter={handleGrant}
        />
      </Modal>
    </>
  )
}
