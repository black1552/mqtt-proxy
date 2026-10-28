import React, { useEffect, useState } from 'react'
import {
  Table, Button, Space, Modal, Form, Input, Select, Switch, InputNumber, Tag, App as AntApp, Popconfirm, Alert,
} from 'antd'
import { PlusOutlined } from '@ant-design/icons'
import { api, type ACLRule } from '../api'
import { useAuth } from '../auth'
import { accessLabel, accessColor, effectLabel, effectColor, accessOptions, effectOptions } from '../labels'

export default function ACL() {
  const { message } = AntApp.useApp()
  const { user } = useAuth()
  const canWrite = user?.role !== 'readonly'

  const [rows, setRows] = useState<ACLRule[]>([])
  const [loading, setLoading] = useState(false)
  const [open, setOpen] = useState(false)
  const [editing, setEditing] = useState<ACLRule | null>(null)
  const [form] = Form.useForm()

  const load = () => {
    setLoading(true)
    api.listACL().then(setRows).catch((e) => message.error(api.errMsg(e))).finally(() => setLoading(false))
  }
  useEffect(load, [])

  const openModal = (rec?: ACLRule) => {
    setEditing(rec || null)
    form.resetFields()
    if (rec) form.setFieldsValue(rec)
    else form.setFieldsValue({ access: 'both', effect: 'allow', max_qos: 2, priority: 100, enabled: true })
    setOpen(true)
  }

  const submit = async () => {
    const v = await form.validateFields()
    try {
      if (editing) await api.updateACL(editing.id, v)
      else await api.createACL(v)
      message.success('保存成功')
      setOpen(false)
      load()
    } catch (e) {
      message.error(api.errMsg(e))
    }
  }

  const columns = [
    { title: 'ID', dataIndex: 'id', width: 60 },
    { title: '名称', dataIndex: 'name' },
    { title: '客户端ID', dataIndex: 'client_id', render: (v: string) => v || <span style={{ color: '#bbb' }}>任意</span> },
    { title: '用户名', dataIndex: 'username', render: (v: string) => v || <span style={{ color: '#bbb' }}>任意</span> },
    { title: '主题', dataIndex: 'topic', render: (v: string) => <code>{v}</code> },
    {
      title: '动作', dataIndex: 'access', width: 110,
      render: (v: string) => <Tag color={accessColor(v)} bordered={false}>{accessLabel(v)}</Tag>,
    },
    {
      title: '效果', dataIndex: 'effect', width: 90,
      render: (v: string) => <Tag color={effectColor(v)} bordered={false}>{effectLabel(v)}</Tag>,
    },
    { title: 'MaxQoS', dataIndex: 'max_qos', width: 80 },
    { title: '优先级', dataIndex: 'priority', width: 80 },
    { title: '启用', dataIndex: 'enabled', width: 70, render: (e: boolean) => (e ? <Tag color="green">是</Tag> : <Tag>否</Tag>) },
    {
      title: '操作',
      width: 140,
      render: (_: any, rec: ACLRule) =>
        canWrite ? (
          <Space>
            <a onClick={() => openModal(rec)}>编辑</a>
            <Popconfirm title="确认删除?" onConfirm={() => api.deleteACL(rec.id).then(load).catch((e) => message.error(api.errMsg(e)))}>
              <a style={{ color: '#ff4d4f' }}>删除</a>
            </Popconfirm>
          </Space>
        ) : null,
    },
  ]

  return (
    <>
      <Alert
        style={{ marginBottom: 16 }}
        type="info"
        message="规则按“优先级”升序匹配，命中即生效；超级管理员账号绕过所有 ACL。若无任何启用规则则默认放行。"
      />
      <Space style={{ marginBottom: 16 }}>
        <Button type="primary" icon={<PlusOutlined />} onClick={() => openModal()} disabled={!canWrite}>新建规则</Button>
      </Space>
      <Table rowKey="id" loading={loading} columns={columns as any} dataSource={rows} scroll={{ x: 1000 }} />

      <Modal title={editing ? '编辑规则' : '新建规则'} open={open} onOk={submit} onCancel={() => setOpen(false)} destroyOnClose width={640}>
        <Form form={form} layout="vertical">
          <Form.Item name="name" label="规则名称" rules={[{ required: true }]}><Input /></Form.Item>
          <Space style={{ width: '100%' }} size="large">
            <Form.Item name="client_id" label="客户端ID(可空,支持*前缀)" style={{ width: 300 }}>
              <Input placeholder="例如 device-*" />
            </Form.Item>
            <Form.Item name="username" label="用户名(可空)" style={{ width: 260 }}>
              <Input />
            </Form.Item>
          </Space>
          <Form.Item name="topic" label="主题过滤器(支持 + / #)" rules={[{ required: true }]}>
            <Input placeholder="例如 devices/+/telemetry 或 sensors/#" />
          </Form.Item>
          <Space style={{ width: '100%' }} size="large">
            <Form.Item name="access" label="动作" style={{ width: 160 }}>
              <Select options={accessOptions} />
            </Form.Item>
            <Form.Item name="effect" label="效果" style={{ width: 140 }}>
              <Select options={effectOptions} />
            </Form.Item>
            <Form.Item name="max_qos" label="最大QoS" style={{ width: 120 }}>
              <InputNumber min={0} max={2} />
            </Form.Item>
            <Form.Item name="priority" label="优先级" style={{ width: 120 }}>
              <InputNumber min={0} max={9999} />
            </Form.Item>
          </Space>
          <Form.Item name="enabled" label="启用" valuePropName="checked"><Switch /></Form.Item>
        </Form>
      </Modal>
    </>
  )
}
