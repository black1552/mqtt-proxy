import React, { useEffect, useRef, useState } from 'react'
import {
  App as AntApp,
  Button,
  Card,
  Form,
  Input,
  Modal,
  Switch,
  Typography,
} from 'antd'
import axios from 'axios'
import { SaveOutlined } from '@ant-design/icons'
import { api, TOKEN_KEY, USER_KEY, type BootstrapConfig } from '../api'

const { Text, Title } = Typography

// 仅需人工决定的四项，其余配置项使用后端内置默认值，JWT 密钥保存时自动生成。
const defaults: BootstrapConfig = {
  allow_anonymous: true,
  default_acl_allow: true,
  admin_user: 'superadmin',
  admin_pass: '',
}

export default function Bootstrap({ configFile }: { configFile: string }) {
  const { message } = AntApp.useApp()
  const [form] = Form.useForm<BootstrapConfig>()
  const [saving, setSaving] = useState(false)
  const [saved, setSaved] = useState(false)
  const timerRef = useRef<number>()

  // 保存后服务会用新配置自动启动。轮询引导状态：只有当后端明确报告
  // 「无需引导」（新配置已加载）时才跳转登录页，避免命中尚未关闭的旧监听。
  useEffect(() => {
    if (!saved) return
    const deadline = Date.now() + 60 * 1000
    timerRef.current = window.setInterval(async () => {
      try {
        const res = await axios.get('/api/bootstrap/status', { timeout: 1500 })
        const st = res?.data?.data as { needs_bootstrap?: boolean } | undefined
        if (st && st.needs_bootstrap === false) {
          if (timerRef.current) window.clearInterval(timerRef.current)
          // 清除旧登录态，进入全新配置对应的登录页。
          localStorage.removeItem(TOKEN_KEY)
          localStorage.removeItem(USER_KEY)
          window.location.href = '/login'
          return
        }
      } catch {
        // 服务正在切换，忽略连接错误继续等待。
      }
      if (Date.now() > deadline && timerRef.current) {
        window.clearInterval(timerRef.current)
        message.warning('等待服务以新配置启动超时，请确认启动日志后刷新页面。')
      }
    }, 1000)
    return () => {
      if (timerRef.current) window.clearInterval(timerRef.current)
    }
  }, [saved, message])

  const onFinish = async (values: BootstrapConfig) => {
    setSaving(true)
    try {
      await api.bootstrapSave(values)
      setSaved(true)
    } catch (e) {
      message.error(api.errMsg(e))
    } finally {
      setSaving(false)
    }
  }

  return (
    <div
      style={{
        minHeight: '100vh',
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
        background: 'linear-gradient(135deg,#1677ff 0%,#0958d9 100%)',
        padding: '32px 16px',
      }}
    >
      <Card style={{ width: 520, boxShadow: '0 10px 40px rgba(0,0,0,.2)' }}>
        <div style={{ textAlign: 'center', marginBottom: 16 }}>
          <Title level={3} style={{ margin: 0 }}>
            初始配置
          </Title>
          <Text type="secondary">
            未检测到配置文件（{configFile || 'config.conf'}），请完成基础运行配置后保存
          </Text>
        </div>

        <Form form={form} layout="vertical" initialValues={defaults} onFinish={onFinish} disabled={saved}>
          <Form.Item name="allow_anonymous" label="允许匿名连接" valuePropName="checked">
            <Switch />
          </Form.Item>
          <Form.Item name="default_acl_allow" label="无匹配 ACL 规则时默认放行" valuePropName="checked">
            <Switch />
          </Form.Item>
          <Form.Item
            name="admin_user"
            label="超级管理员用户名"
            rules={[{ required: true, message: '请输入用户名' }]}
          >
            <Input placeholder="superadmin" />
          </Form.Item>
          <Form.Item
            name="admin_pass"
            label="超级管理员密码"
            rules={[
              { required: true, message: '请输入密码' },
              { min: 6, message: '密码至少 6 位' },
            ]}
          >
            <Input.Password placeholder="至少 6 位" autoComplete="new-password" />
          </Form.Item>
          <Text type="secondary" style={{ display: 'block', marginBottom: 16 }}>
            其余参数（监听地址、数据库、日志等）均使用默认值，JWT 密钥将自动生成。
          </Text>

          <Form.Item style={{ marginBottom: 0 }}>
            <Button type="primary" htmlType="submit" block size="large" icon={<SaveOutlined />} loading={saving}>
              保存配置
            </Button>
          </Form.Item>
        </Form>
      </Card>

      <Modal
        open={saved}
        title="配置已保存"
        closable={false}
        footer={null}
        maskClosable={false}
      >
        <p>配置文件已写入，服务正在使用新配置自动启动，就绪后将自动跳转到登录页。</p>
        <p style={{ color: '#888' }}>
          若长时间未跳转（例如端口被占用），请查看控制台输出后重新启动 mqtt-proxy。
        </p>
      </Modal>
    </div>
  )
}
