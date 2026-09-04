import { useEffect, useRef, useState } from 'react'
import { Table, Button, Tag, Space, Progress } from 'antd'
import { LinkOutlined, StopOutlined, LoadingOutlined } from '@ant-design/icons'
import { Server, ProgressEvent, subscribeProgress } from '../api'

interface Props {
  sessions: Record<string, Server>
  onConnect: (name: string) => void
  onStop: (name: string) => void
}

type SessionRecord = Server & { key: string; name: string }

const progressCache = new Map<string, ProgressEvent[]>()

function useSessionProgress(name: string, pending?: boolean) {
  const [events, setEvents] = useState<ProgressEvent[]>(progressCache.get(name) ?? [])
  const unsubRef = useRef<(() => void) | null>(null)
  const subscribedRef = useRef(false)

  useEffect(() => {
    if (!pending || subscribedRef.current) return
    subscribedRef.current = true
    unsubRef.current = subscribeProgress(
      name,
      (e) => {
        setEvents((prev) => {
          const next = [...prev, e]
          progressCache.set(name, next)
          return next
        })
      },
      () => {},
    )
    return () => unsubRef.current?.()
  }, [name, pending])

  return events
}

function ExpandedProgress({ record }: { record: SessionRecord }) {
  const events = useSessionProgress(record.name, record.pending)

  if (events.length === 0 && record.pending) {
    return <LoadingOutlined style={{ marginLeft: 8 }} />
  }
  if (events.length === 0) return null

  const last = events[events.length - 1]

  return (
    <div>
      <Progress
        percent={last.progress}
        size="small"
        status={last.ready ? 'success' : 'active'}
        style={{ maxWidth: 300 }}
      />
      <div style={{
        fontSize: 12,
        color: '#888',
        maxHeight: 120,
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
    <Table<SessionRecord>
      dataSource={dataSource}
      pagination={false}
      size="middle"
      expandable={{
        expandedRowRender: (record) => <ExpandedProgress record={record} />,
        rowExpandable: () => true,
      }}
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
