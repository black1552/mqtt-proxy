import React, { useEffect, useState } from 'react'
import {
  Table, Button, Space, Modal, Form, Input, Switch, Tag, App as AntApp, Popconfirm, Tabs, Statistic, Card, Row, Col,
} from 'antd'
import { PlusOutlined, ReloadOutlined } from '@ant-design/icons'
import { api, type Topic } from '../api'
import { useAuth } from '../auth'

function Managed() {
  const { message } = AntApp.useApp()
  const { user } = useAuth()
  const canWrite = user?.role !== 'readonly'
  const [rows, setRows] = useState<Topic[]>([])
  const [loading, setLoading] = useState(false)
  const [open, setOpen] = useState(false)
  const [editing, setEditing] = useState<Topic | null>(null)
  const [form] = Form.useForm()

  const load = () => {
    setLoading(true)
    api.listTopics().then(setRows).catch((e) => message.error(api.errMsg(e))).finally(() => setLoading(false))
  }
  useEffect(load, [])

  const submit = async () => {
    const v = await form.validateFields()
    try {
      if (editing) await api.updateTopic(editing.id, v)
      else await api.createTopic(v)
      message.success('保存成功')
      setOpen(false)
      load()
    } catch (e) {
      message.error(api.errMsg(e))
    }
  }

  const columns = [
    { title: 'ID', dataIndex: 'id', width: 60 },
    { title: '主题', dataIndex: 'topic', render: (v: string) => <code>{v}</code> },
    { title: '描述', dataIndex: 'description' },
    { title: '启用', dataIndex: 'enabled', render: (e: boolean) => (e ? <Tag color="green">是</Tag> : <Tag>否</Tag>) },
    { title: '创建时间', dataIndex: 'created_at', render: (t: string) => new Date(t).toLocaleString() },
    {
      title: '操作', width: 140, render: (_: any, rec: Topic) =>
        canWrite ? (
          <Space>
            <a onClick={() => { setEditing(rec); form.resetFields(); form.setFieldsValue(rec); setOpen(true) }}>编辑</a>
            <Popconfirm title="确认删除?" onConfirm={() => api.deleteTopic(rec.id).then(load).catch((e) => message.error(api.errMsg(e)))}>
              <a style={{ color: '#ff4d4f' }}>删除</a>
            </Popconfirm>
          </Space>
        ) : null,
    },
  ]

  return (
    <>
      <Space style={{ marginBottom: 16 }}>
        <Button type="primary" icon={<PlusOutlined />} disabled={!canWrite}
          onClick={() => { setEditing(null); form.resetFields(); form.setFieldsValue({ enabled: true }); setOpen(true) }}>
          新建主题
        </Button>
        <Button icon={<ReloadOutlined />} onClick={load}>刷新</Button>
      </Space>
      <Table rowKey="id" loading={loading} columns={columns as any} dataSource={rows} />
      <Modal title={editing ? '编辑主题' : '新建主题'} open={open} onOk={submit} onCancel={() => setOpen(false)} destroyOnClose>
        <Form form={form} layout="vertical">
          <Form.Item name="topic" label="主题/模式" rules={[{ required: true }]}><Input placeholder="例如 home/livingroom/light" /></Form.Item>
          <Form.Item name="description" label="描述"><Input.TextArea rows={2} /></Form.Item>
          <Form.Item name="enabled" label="启用" valuePropName="checked"><Switch /></Form.Item>
        </Form>
      </Modal>
    </>
  )
}

function LiveTopics() {
  const { message } = AntApp.useApp()
  const [data, setData] = useState<Record<string, number>>({})

  const load = () => api.liveTopics().then(setData).catch((e) => message.error(api.errMsg(e)))
  useEffect(() => {
    load()
    const t = setInterval(load, 3000)
    return () => clearInterval(t)
  }, [])

  const rows = Object.entries(data).map(([topic, count]) => ({ topic, count }))
  rows.sort((a, b) => b.count - a.count)

  return (
    <>
      <Row gutter={16} style={{ marginBottom: 16 }}>
        <Col span={6}><Card><Statistic title="活跃主题数" value={rows.length} /></Card></Col>
      </Row>
      <Table
        rowKey="topic"
        dataSource={rows}
        columns={[
          { title: '主题', dataIndex: 'topic', render: (v: string) => <code>{v}</code> },
          { title: '消息计数', dataIndex: 'count', width: 140, sorter: (a: any, b: any) => a.count - b.count },
        ]}
      />
    </>
  )
}

export default function Topics() {
  return (
    <Tabs
      defaultActiveKey="managed"
      items={[
        { key: 'managed', label: '主题目录', children: <Managed /> },
        { key: 'live', label: '实时主题', children: <LiveTopics /> },
      ]}
    />
  )
}
