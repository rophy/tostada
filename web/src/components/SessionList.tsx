import { useEffect, useRef, useState } from 'react'
import { Table, Button, Tag, Space, Progress, Popconfirm, message } from 'antd'
import { LinkOutlined, StopOutlined, LoadingOutlined, DownOutlined, RightOutlined } from '@ant-design/icons'
import { Server, ProgressEvent, subscribeProgress } from '../api'

interface Props {
  sessions: Record<string, Server>
  onConnect: (name: string) => void
  onStop: (name: string) => Promise<void>
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

function StatusTag({ record, expanded, onToggle }: {
  record: SessionRecord
  expanded: boolean
  onToggle: () => void
}) {
  const chevron = expanded
    ? <DownOutlined style={{ fontSize: 10, marginLeft: 4 }} />
    : <RightOutlined style={{ fontSize: 10, marginLeft: 4 }} />

  if (record.ready) {
    return (
      <Tag color="success" style={{ cursor: 'pointer' }} onClick={onToggle}>
        Ready {chevron}
      </Tag>
    )
  }
  if (record.pending) {
    return (
      <Tag color="processing" style={{ cursor: 'pointer' }} onClick={onToggle}>
        Starting... {chevron}
      </Tag>
    )
  }
  return <Tag color="warning">Unknown</Tag>
}

function StopAction({ record, onConnect, onStop }: {
  record: SessionRecord
  onConnect: (name: string) => void
  onStop: (name: string) => Promise<void>
}) {
  const [stopping, setStopping] = useState(false)

  const handleStop = async () => {
    setStopping(true)
    try {
      await onStop(record.name)
    } catch {
      message.error(`Failed to stop session "${record.name}"`)
      setStopping(false)
    }
  }

  return (
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
      <Popconfirm
        title="Stop this session?"
        onConfirm={handleStop}
        okText="Stop"
        okButtonProps={{ danger: true }}
      >
        <Button
          type="link"
          danger
          icon={<StopOutlined />}
          loading={stopping}
        >
          Stop
        </Button>
      </Popconfirm>
    </Space>
  )
}

export function SessionList({ sessions, onConnect, onStop }: Props) {
  const entries = Object.entries(sessions)
  if (entries.length === 0) return null

  const [expandedKeys, setExpandedKeys] = useState<string[]>([])

  const toggleExpand = (key: string) => {
    setExpandedKeys((prev) =>
      prev.includes(key) ? prev.filter((k) => k !== key) : [...prev, key],
    )
  }

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
        expandedRowKeys: expandedKeys,
        expandedRowRender: (record) => <ExpandedProgress record={record} />,
        expandIcon: () => null,
        columnWidth: 0,
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
          render: (_, record) => (
            <StatusTag
              record={record}
              expanded={expandedKeys.includes(record.key)}
              onToggle={() => toggleExpand(record.key)}
            />
          ),
        },
        {
          title: 'Actions',
          key: 'actions',
          align: 'right' as const,
          render: (_, record) => (
            <StopAction
              record={record}
              onConnect={onConnect}
              onStop={onStop}
            />
          ),
        },
      ]}
    />
  )
}
