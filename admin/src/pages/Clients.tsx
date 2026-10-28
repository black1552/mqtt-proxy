import React, { useEffect, useState } from 'react'
import {
  Tabs, Table, Button, Space, Modal, Form, Input, Switch, Tag, App as AntApp, Popconfirm, Badge,
} from 'antd'
import { PlusOutlined, ReloadOutlined } from '@ant-design/icons'
import { api, type ClientDevice, type ConnectedClient } from '../api'
import { useAuth } from '../auth'

function Registered() {
  const { message } = AntApp.useApp()
  const { user } = useAuth()
  const canWrite = user?.role !== 'readonly'
  const [rows, setRows] = useState<ClientDevice[]>([])
  const [loading, setLoading] = useState(false)
  const [open, setOpen] = useState(false)
  const [editing, setEditing] = useState<ClientDevice | null>(null)
  const [form] = Form.useForm()

  const load = () => {
    setLoading(true)
    api.listClients().then(setRows).catch((e) => message.error(api.errMsg(e))).finally(() => setLoading(false))
  }
  useEffect(load, [])

  const submit = async () => {
    const v = await form.validateFields()
    try {
      if (editing) await api.updateClient(editing.id, v)
      else await api.createClient(v)
      message.success('保存成功')
      setOpen(false)
      load()
    } catch (e) {
      message.error(api.errMsg(e))
    }
  }

  const columns = [
    { title: '客户端ID', dataIndex: 'id', render: (v: string) => <code>{v}</code> },
    { title: '名称', dataIndex: 'name' },
    { title: '描述', dataIndex: 'description' },
    { title: '启用', dataIndex: 'enabled', render: (e: boolean) => (e ? <Tag color="green">允许</Tag> : <Tag color="red">禁用</Tag>) },
    { title: '创建时间', dataIndex: 'created_at', render: (t: string) => new Date(t).toLocaleString() },
    {
      title: '操作', width: 140, render: (_: any, rec: ClientDevice) =>
        canWrite ? (
          <Space>
            <a onClick={() => { setEditing(rec); form.resetFields(); form.setFieldsValue(rec); setOpen(true) }}>编辑</a>
            <Popconfirm title="确认删除?" onConfirm={() => api.deleteClient(rec.id).then(load).catch((e) => message.error(api.errMsg(e)))}>
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
          登记设备
        </Button>
        <Button icon={<ReloadOutlined />} onClick={load}>刷新</Button>
      </Space>
      <Table rowKey="id" loading={loading} columns={columns as any} dataSource={rows} />
      <Modal title={editing ? '编辑设备' : '登记设备'} open={open} onOk={submit} onCancel={() => setOpen(false)} destroyOnClose>
        <Form form={form} layout="vertical">
          <Form.Item name="id" label="客户端ID" rules={[{ required: true }]}>
            <Input disabled={!!editing} />
          </Form.Item>
          <Form.Item name="name" label="名称"><Input /></Form.Item>
          <Form.Item name="description" label="描述"><Input.TextArea rows={2} /></Form.Item>
          <Form.Item name="enabled" label="允许连接" valuePropName="checked"><Switch /></Form.Item>
        </Form>
      </Modal>
    </>
  )
}

function Live() {
  const { message } = AntApp.useApp()
  const { user } = useAuth()
  const canWrite = user?.role !== 'readonly'
  const [rows, setRows] = useState<ConnectedClient[]>([])
  const [loading, setLoading] = useState(false)

  const load = () => {
    setLoading(true)
    api.connected().then(setRows).catch((e) => message.error(api.errMsg(e))).finally(() => setLoading(false))
  }
  useEffect(() => {
    load()
    const t = setInterval(load, 3000)
    return () => clearInterval(t)
  }, [])

  const columns = [
    { title: '客户端ID', dataIndex: 'client_id', render: (v: string) => <code>{v}</code> },
    { title: '用户名', dataIndex: 'username', render: (v: string) => v || '-' },
    { title: '协议', dataIndex: 'protocol', width: 90, render: (v: string) => <Tag color="blue">MQTT {v}</Tag> },
    { title: '来源地址', dataIndex: 'remote' },
    { title: '监听器', dataIndex: 'listener', width: 90 },
    { title: '会话清除', dataIndex: 'clean_session', width: 90, render: (v: boolean) => (v ? <Tag color="blue">是</Tag> : <Tag>否</Tag>) },
    { title: '连接时间', dataIndex: 'connected_at', render: (t: string) => new Date(t).toLocaleString() },
    {
      title: '订阅', dataIndex: 'subscriptions', render: (s: string[]) =>
        (s && s.length ? <Space size={4} wrap>{s.map((x) => <Tag key={x}>{x}</Tag>)}</Space> : <span style={{ color: '#bbb' }}>无</span>),
    },
    {
      title: '操作', width: 100, render: (_: any, rec: ConnectedClient) =>
        canWrite ? (
          <Popconfirm title="强制断开?" onConfirm={() => api.disconnectClient(rec.client_id).then(load).catch((e) => message.error(api.errMsg(e)))}>
            <a style={{ color: '#ff4d4f' }}>断开</a>
          </Popconfirm>
        ) : null,
    },
  ]

  return (
    <>
      <Space style={{ marginBottom: 16 }}>
        <Badge status="processing" text={`在线 ${rows.length}`} />
        <Button icon={<ReloadOutlined />} onClick={load}>刷新</Button>
      </Space>
      <Table rowKey="client_id" loading={loading} columns={columns as any} dataSource={rows} scroll={{ x: 1100 }} />
    </>
  )
}

export default function Clients() {
  return (
    <Tabs
      defaultActiveKey="live"
      items={[
        { key: 'live', label: '在线客户端', children: <Live /> },
        { key: 'reg', label: '设备登记', children: <Registered /> },
      ]}
    />
  )
}
