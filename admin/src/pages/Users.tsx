import React, { useEffect, useMemo, useState } from 'react'
import {
  Table, Button, Space, Modal, Form, Input, Select, Switch, Tag, App as AntApp, Popconfirm, Checkbox,
} from 'antd'
import { PlusOutlined } from '@ant-design/icons'
import { api, type User } from '../api'
import { useAuth } from '../auth'
import { roleLabel, roleColor } from '../labels'

export default function Users() {
  const { message } = AntApp.useApp()
  const { user } = useAuth()
  const canWrite = user?.role !== 'readonly'
  const isAdmin = user?.role === 'superadmin' || user?.role === 'admin'

  const [rows, setRows] = useState<User[]>([])
  const [loading, setLoading] = useState(false)
  const [includeMQTT, setIncludeMQTT] = useState(true)
  const [open, setOpen] = useState(false)
  const [editing, setEditing] = useState<User | null>(null)
  const [form] = Form.useForm()

  const load = () => {
    setLoading(true)
    api.listUsers(includeMQTT)
      .then(setRows)
      .catch((e) => message.error(api.errMsg(e)))
      .finally(() => setLoading(false))
  }
  useEffect(load, [includeMQTT])

  const openModal = (rec?: User) => {
    setEditing(rec || null)
    form.resetFields()
    if (rec) form.setFieldsValue({ ...rec, password: '' })
    else form.setFieldsValue({ role: 'mqtt', enabled: true })
    setOpen(true)
  }

  const submit = async () => {
    const v = await form.validateFields()
    try {
      if (editing) {
        await api.updateUser(editing.id, { username: v.username, role: v.role, enabled: v.enabled, remark: v.remark })
        if (v.password) await api.resetPassword(editing.id, v.password)
      } else {
        await api.createUser(v)
      }
      message.success('保存成功')
      setOpen(false)
      load()
    } catch (e) {
      message.error(api.errMsg(e))
    }
  }

  const columns = useMemo(
    () => [
      { title: 'ID', dataIndex: 'id', width: 70 },
      { title: '用户名', dataIndex: 'username' },
      { title: '角色', dataIndex: 'role', render: (r: string) => <Tag color={roleColor(r)} bordered={false}>{roleLabel(r)}</Tag> },
      { title: '启用', dataIndex: 'enabled', render: (e: boolean) => (e ? <Tag color="green">是</Tag> : <Tag>否</Tag>) },
      { title: '备注', dataIndex: 'remark' },
      { title: '创建时间', dataIndex: 'created_at', render: (t: string) => new Date(t).toLocaleString() },
      {
        title: '操作',
        width: 180,
        render: (_: any, rec: User) =>
          canWrite ? (
            <Space>
              <a onClick={() => openModal(rec)}>编辑</a>
              <Popconfirm title="确认删除?" onConfirm={() => api.deleteUser(rec.id).then(load).catch((e) => message.error(api.errMsg(e)))}>
                <a style={{ color: rec.role === 'superadmin' ? '#ccc' : '#ff4d4f' }}>删除</a>
              </Popconfirm>
            </Space>
          ) : null,
      },
    ],
    [canWrite],
  )

  const roleOptions = isAdmin
    ? ['superadmin', 'admin', 'readonly', 'mqtt']
    : ['admin', 'readonly', 'mqtt']

  return (
    <>
      <Space style={{ marginBottom: 16 }}>
        <Button type="primary" icon={<PlusOutlined />} onClick={() => openModal()} disabled={!canWrite}>
          新建用户
        </Button>
        <Checkbox checked={includeMQTT} onChange={(e) => setIncludeMQTT(e.target.checked)}>
          显示 MQTT 设备账号
        </Checkbox>
      </Space>
      <Table rowKey="id" loading={loading} columns={columns as any} dataSource={rows} />

      <Modal title={editing ? '编辑用户' : '新建用户'} open={open} onOk={submit} onCancel={() => setOpen(false)} destroyOnClose>
        <Form form={form} layout="vertical">
          <Form.Item name="username" label="用户名" rules={[{ required: true }]}>
            <Input />
          </Form.Item>
          <Form.Item
            name="password"
            label="密码"
            rules={editing ? [] : [{ required: true, message: '请输入密码' }]}
            tooltip={editing ? '留空表示不修改密码' : undefined}
          >
            <Input.Password />
          </Form.Item>
          <Form.Item name="role" label="角色" rules={[{ required: true }]}>
            <Select options={roleOptions.map((r) => ({ label: roleLabel(r), value: r }))} />
          </Form.Item>
          <Form.Item name="enabled" label="启用" valuePropName="checked">
            <Switch />
          </Form.Item>
          <Form.Item name="remark" label="备注">
            <Input.TextArea rows={2} />
          </Form.Item>
        </Form>
      </Modal>
    </>
  )
}
