import { useEffect, useRef, useState } from 'react'
import { Table, Button, Tag, Space, Progress } from 'antd'
import { LinkOutlined, StopOutlined, LoadingOutlined } from '@ant-design/icons'
import { Server, ProgressEvent, subscribeProgress } from '../api'

interface Props {
  sessions: Record<string, Server>
  onConnect: (name: string) => void
  onStop: (name: string) => void
}

function SessionProgress({ name }: { name: string }) {
  const [events, setEvents] = useState<ProgressEvent[]>([])
  const unsubRef = useRef<(() => void) | null>(null)

  useEffect(() => {
    unsubRef.current = subscribeProgress(
      name,
      (e) => setEvents((prev) => [...prev, e]),
      () => {},
    )
    return () => unsubRef.current?.()
  }, [name])

  if (events.length === 0) return <LoadingOutlined style={{ marginLeft: 8 }} />

  const last = events[events.length - 1]

  return (
    <div style={{ marginTop: 4 }}>
      <Progress
        percent={last.progress}
        size="small"
        status={last.ready ? 'success' : 'active'}
        style={{ maxWidth: 200 }}
      />
      <div style={{
        fontSize: 12,
        color: '#888',
        maxHeight: 80,
        overflowY: 'auto',
        marginTop: 4,
      }}>
        {events.map((e, i) => (
          <div key={i}>{e.message}</div>
        ))}
      </div>
    </div>
  )
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
              <>
                <Tag color="processing">Starting...</Tag>
                <SessionProgress name={record.name} />
              </>
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
