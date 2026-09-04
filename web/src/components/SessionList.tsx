import { useEffect, useRef, useState } from 'react'
import { Table, Button, Tag, Space, Progress } from 'antd'
import { LinkOutlined, StopOutlined, LoadingOutlined, DownOutlined, RightOutlined } from '@ant-design/icons'
import { Server, ProgressEvent, subscribeProgress } from '../api'

interface Props {
  sessions: Record<string, Server>
  onConnect: (name: string) => void
  onStop: (name: string) => void
}

function SessionProgress({ name }: { name: string }) {
  const [events, setEvents] = useState<ProgressEvent[]>([])
  const [expanded, setExpanded] = useState(true)
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
      {expanded && (
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
      )}
    </div>
  )
}

function StatusTag({ record }: { record: { name: string; ready: boolean; pending?: boolean } }) {
  const [expanded, setExpanded] = useState(false)
  const [events, setEvents] = useState<ProgressEvent[]>([])
  const unsubRef = useRef<(() => void) | null>(null)
  const subscribedRef = useRef(false)

  useEffect(() => {
    if (!record.pending || subscribedRef.current) return
    subscribedRef.current = true
    unsubRef.current = subscribeProgress(
      record.name,
      (e) => setEvents((prev) => [...prev, e]),
      () => {},
    )
    return () => unsubRef.current?.()
  }, [record.name, record.pending])

  if (record.ready) {
    if (events.length === 0) return <Tag color="success">Ready</Tag>
    const last = events[events.length - 1]
    return (
      <div>
        <Tag
          color="success"
          style={{ cursor: 'pointer' }}
          onClick={() => setExpanded((v) => !v)}
        >
          Ready {expanded ? <DownOutlined /> : <RightOutlined />}
        </Tag>
        {expanded && (
          <div style={{ fontSize: 12, color: '#888', maxHeight: 80, overflowY: 'auto', marginTop: 4 }}>
            {events.map((e, i) => (
              <div key={i}>{e.message}</div>
            ))}
          </div>
        )}
      </div>
    )
  }

  if (record.pending) {
    return (
      <div>
        <Tag
          color="processing"
          style={{ cursor: 'pointer' }}
          onClick={() => setExpanded((v) => !v)}
        >
          Starting... {expanded ? <DownOutlined /> : <RightOutlined />}
        </Tag>
        {events.length === 0 && <LoadingOutlined style={{ marginLeft: 8 }} />}
        {events.length > 0 && (
          <div style={{ marginTop: 4 }}>
            <Progress
              percent={events[events.length - 1].progress}
              size="small"
              status="active"
              style={{ maxWidth: 200 }}
            />
            {expanded && (
              <div style={{ fontSize: 12, color: '#888', maxHeight: 80, overflowY: 'auto', marginTop: 4 }}>
                {events.map((e, i) => (
                  <div key={i}>{e.message}</div>
                ))}
              </div>
            )}
          </div>
        )}
      </div>
    )
  }

  return <Tag color="warning">Unknown</Tag>
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
          render: (_, record) => <StatusTag record={record} />,
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
