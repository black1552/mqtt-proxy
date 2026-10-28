import React, { useCallback, useEffect, useRef, useState } from 'react'
import {
  Row, Col, Card, Statistic, Table, Tag, Space, Input, Select, Button, Switch, App as AntApp, Badge, List, Empty,
} from 'antd'
import { api, wsURL, type MessageLog, type StatsSnapshot } from '../api'
import { useAuth } from '../auth'

// 一个无依赖的极简 SVG 迷你折线/趋势图。
function Trend({ data, color, height = 120 }: { data: number[]; color: string; height?: number }) {
  const width = 600
  if (data.length < 2) {
    return <div style={{ height, color: '#bbb', display: 'flex', alignItems: 'center' }}>数据采集中…</div>
  }
  const max = Math.max(...data, 1)
  const stepX = width / (data.length - 1)
  const pts = data.map((v, i) => `${i * stepX},${height - (v / max) * height}`).join(' ')
  return (
    <svg width="100%" height={height} viewBox={`0 0 ${width} ${height}`} preserveAspectRatio="none">
      <polyline fill="none" stroke={color} strokeWidth={2} points={pts} />
      <polygon fill={color} opacity={0.12} points={`0,${height} ${pts} ${width},${height}`} />
    </svg>
  )
}

export default function Monitor() {
  const { message } = AntApp.useApp()
  const { user } = useAuth()
  const canWrite = user?.role !== 'readonly'

  const [info, setInfo] = useState<Record<string, number>>({})
  const [clients, setClients] = useState(0)
  const [feed, setFeed] = useState<string[]>([])
  const [logs, setLogs] = useState<MessageLog[]>([])
  const [conn, setConn] = useState(false)
  const [trend, setTrend] = useState<number[]>([])
  const wsRef = useRef<WebSocket | null>(null)
  const reconnectRef = useRef<any>(null)

  const [topic, setTopic] = useState('test/topic')
  const [payload, setPayload] = useState('hello mqtt-proxy')
  const [qos, setQos] = useState(0)
  const [retain, setRetain] = useState(false)

  const connect = useCallback(() => {
    const ws = new WebSocket(wsURL())
    wsRef.current = ws
    ws.onopen = () => setConn(true)
    ws.onclose = () => {
      setConn(false)
      reconnectRef.current = setTimeout(connect, 2000)
    }
    ws.onmessage = (ev) => {
      try {
        const frame = JSON.parse(ev.data)
        if (frame.type === 'stats') {
          setInfo(frame.info || {})
          setClients(frame.clients || 0)
          setTrend((t) => {
            const next = [...t, Number(frame.info?.messages_received ?? 0)]
            return next.slice(-60)
          })
        } else if (frame.type === 'message' && frame.message) {
          setLogs((l) => [frame.message, ...l].slice(0, 50))
        } else if (frame.type === 'client_connected' || frame.type === 'client_disconnected') {
          const text = frame.type === 'client_connected'
            ? `+ 上线 ${frame.client?.client_id || ''}`
            : `- 下线 ${frame.detail || ''}`
          setFeed((f) => [`${new Date(frame.ts).toLocaleTimeString()} ${text}`, ...f].slice(0, 100))
        }
      } catch {
        /* 忽略格式错误的帧 */
      }
    }
  }, [])

  useEffect(() => {
    api.history(60).then((h: StatsSnapshot[]) => setTrend(h.map((x) => x.messages_received))).catch(() => {})
    api.messages(50).then(setLogs).catch(() => {})
    connect()
    return () => {
      clearTimeout(reconnectRef.current)
      wsRef.current?.close()
    }
  }, [connect])

  const doPublish = async () => {
    try {
      await api.publish(topic, payload, qos, retain)
      message.success('已发布')
    } catch (e) {
      message.error(api.errMsg(e))
    }
  }

  const logColumns = [
    { title: '时间', dataIndex: 'ts', width: 170, render: (t: string) => new Date(t).toLocaleTimeString() },
    { title: '主题', dataIndex: 'topic', render: (v: string) => <code>{v}</code> },
    { title: '客户端', dataIndex: 'client_id', width: 140 },
    { title: 'QoS', dataIndex: 'qos', width: 60 },
    { title: '大小', dataIndex: 'size', width: 80, render: (v: number) => `${v}B` },
    { title: '保留', dataIndex: 'retained', width: 60, render: (v: boolean) => (v ? <Tag>是</Tag> : '-') },
  ]

  return (
    <Space direction="vertical" size={16} style={{ width: '100%' }}>
      <Card
        title={<Space>实时状态 <Badge status={conn ? 'success' : 'error'} text={conn ? 'WebSocket 已连接' : '连接中断'} /></Space>}
      >
        <Row gutter={16}>
          <Col span={4}><Statistic title="在线客户端" value={clients} /></Col>
          <Col span={4}><Statistic title="订阅数" value={Number(info.subscriptions ?? 0)} /></Col>
          <Col span={4}><Statistic title="接收消息" value={Number(info.messages_received ?? 0)} /></Col>
          <Col span={4}><Statistic title="发送消息" value={Number(info.messages_sent ?? 0)} /></Col>
          <Col span={4}><Statistic title="保留消息" value={Number(info.retained ?? 0)} /></Col>
          <Col span={4}><Statistic title="运行时长(s)" value={Number(info.uptime ?? 0)} /></Col>
        </Row>
        <div style={{ marginTop: 16 }}>
          <div style={{ marginBottom: 6, color: '#888' }}>接收消息趋势</div>
          <Trend data={trend} color="#1677ff" />
        </div>
      </Card>

      <Row gutter={16}>
        <Col span={16}>
          <Card title="消息流水 (最近 50 条)">
            <Table rowKey="id" size="small" columns={logColumns as any} dataSource={logs} pagination={false} scroll={{ y: 300 }} />
          </Card>
        </Col>
        <Col span={8}>
          <Card title="连接/断开事件">
            {feed.length === 0 ? <Empty description="暂无事件" /> : (
              <List size="small" dataSource={feed} renderItem={(x) => <List.Item><span style={{ fontSize: 12 }}>{x}</span></List.Item>}
                style={{ maxHeight: 300, overflow: 'auto' }} />
            )}
          </Card>
        </Col>
      </Row>

      <Card title="消息发布测试">
        <Space wrap>
          <Input style={{ width: 240 }} value={topic} onChange={(e) => setTopic(e.target.value)} placeholder="主题" disabled={!canWrite} />
          <Input style={{ width: 280 }} value={payload} onChange={(e) => setPayload(e.target.value)} placeholder="消息内容" disabled={!canWrite} />
          <Select style={{ width: 120 }} value={qos} onChange={setQos} disabled={!canWrite}
            options={[0, 1, 2].map((v) => ({ label: `QoS ${v}`, value: v }))} />
          <Space>保留 <Switch checked={retain} onChange={setRetain} disabled={!canWrite} /></Space>
          <Button type="primary" onClick={doPublish} disabled={!canWrite}>发布</Button>
        </Space>
      </Card>
    </Space>
  )
}
