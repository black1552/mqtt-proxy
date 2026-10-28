import React, { useEffect, useState } from 'react'
import { Row, Col, Card, Statistic, Descriptions, App as AntApp } from 'antd'
import { api, type Dashboard } from '../api'

export default function Dashboard() {
  const { message } = AntApp.useApp()
  const [d, setD] = useState<Dashboard | null>(null)

  const load = () => api.dashboard().then(setD).catch((e) => message.error(api.errMsg(e)))

  useEffect(() => {
    load()
    const t = setInterval(load, 5000)
    return () => clearInterval(t)
  }, [])

  const broker = d?.broker || {}

  return (
    <>
      <Row gutter={16}>
        <Col span={4}><Card><Statistic title="在线客户端" value={d?.connected ?? 0} /></Card></Col>
        <Col span={4}><Card><Statistic title="注册用户" value={d?.users ?? 0} /></Card></Col>
        <Col span={4}><Card><Statistic title="ACL 规则" value={d?.acl_rules ?? 0} /></Card></Col>
        <Col span={4}><Card><Statistic title="已登记设备" value={d?.registered ?? 0} /></Card></Col>
        <Col span={4}><Card><Statistic title="管理主题" value={d?.managed_topics ?? 0} /></Card></Col>
        <Col span={4}><Card><Statistic title="订阅数" value={Number(broker.subscriptions ?? 0)} /></Card></Col>
      </Row>

      <Card title="Broker 运行指标 ($SYS)" style={{ marginTop: 16 }}>
        <Descriptions column={3} bordered size="small">
          <Descriptions.Item label="接收消息">{Number(broker.messages_received ?? 0)}</Descriptions.Item>
          <Descriptions.Item label="发送消息">{Number(broker.messages_sent ?? 0)}</Descriptions.Item>
          <Descriptions.Item label="丢弃消息">{Number(broker.messages_dropped ?? 0)}</Descriptions.Item>
          <Descriptions.Item label="接收字节">{Number(broker.bytes_received ?? 0)}</Descriptions.Item>
          <Descriptions.Item label="发送字节">{Number(broker.bytes_sent ?? 0)}</Descriptions.Item>
          <Descriptions.Item label="保留消息">{Number(broker.retained ?? 0)}</Descriptions.Item>
          <Descriptions.Item label="连接总数">{Number(broker.clients_total ?? 0)}</Descriptions.Item>
          <Descriptions.Item label="在线客户端">{Number(broker.clients_connected ?? 0)}</Descriptions.Item>
          <Descriptions.Item label="运行时长(s)">{Number(broker.uptime ?? 0)}</Descriptions.Item>
        </Descriptions>
      </Card>
    </>
  )
}
