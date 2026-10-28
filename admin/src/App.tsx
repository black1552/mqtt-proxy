import React, { useState } from 'react'
import { Navigate, Route, Routes, useLocation, useNavigate } from 'react-router-dom'
import { Layout, Menu, Spin, Dropdown, Avatar, Tag, App as AntApp } from 'antd'
import {
  DashboardOutlined,
  TeamOutlined,
  SafetyCertificateOutlined,
  ApiOutlined,
  NodeIndexOutlined,
  FundProjectionScreenOutlined,
  LogoutOutlined,
  UserOutlined,
  ClusterOutlined,
  MenuFoldOutlined,
  MenuUnfoldOutlined,
} from '@ant-design/icons'
import { useAuth } from './auth'
import { roleLabel, roleColor } from './labels'
import Bootstrap from './pages/Bootstrap'
import Login from './pages/Login'
import Dashboard from './pages/Dashboard'
import Users from './pages/Users'
import ACL from './pages/ACL'
import Clients from './pages/Clients'
import Topics from './pages/Topics'
import Monitor from './pages/Monitor'

const { Header, Sider, Content } = Layout

const menuItems = [
  { key: '/', icon: <DashboardOutlined />, label: '概览' },
  { key: '/monitor', icon: <FundProjectionScreenOutlined />, label: '实时监控' },
  { key: '/clients', icon: <ApiOutlined />, label: '客户端管理' },
  { key: '/topics', icon: <NodeIndexOutlined />, label: '主题管理' },
  { key: '/acls', icon: <SafetyCertificateOutlined />, label: 'ACL 规则' },
  { key: '/users', icon: <TeamOutlined />, label: '用户管理' },
]

const titleOf = (path: string) => menuItems.find((m) => m.key === path)?.label ?? '管理后台'

function Shell() {
  const { user, logout } = useAuth()
  const nav = useNavigate()
  const loc = useLocation()
  const { message } = AntApp.useApp()
  const [collapsed, setCollapsed] = useState(false)

  const canWrite = user && user.role !== 'readonly'

  return (
    <Layout style={{ minHeight: '100vh' }}>
      <Sider
        breakpoint="lg"
        collapsedWidth="0"
        collapsible
        collapsed={collapsed}
        onCollapse={setCollapsed}
        trigger={null}
        width={228}
        style={{ boxShadow: '2px 0 8px rgba(0,21,41,.08)' }}
      >
        <div className="brand">
          <span className="brand-badge">
            <ClusterOutlined />
          </span>
          {!collapsed && <span>MQTT Proxy</span>}
        </div>
        <Menu
          theme="dark"
          mode="inline"
          selectedKeys={[loc.pathname]}
          items={menuItems}
          onClick={(e) => nav(e.key)}
        />
      </Sider>
      <Layout>
        <Header className="app-header">
          <div className="page-title">
            <span
              style={{ cursor: 'pointer', fontSize: 18, color: 'rgba(0,0,0,.65)' }}
              onClick={() => setCollapsed((c) => !c)}
            >
              {collapsed ? <MenuUnfoldOutlined /> : <MenuFoldOutlined />}
            </span>
            <span className="title-bar" />
            <span>{titleOf(loc.pathname)}</span>
          </div>
          <Dropdown
            menu={{
              items: [
                {
                  key: 'logout',
                  icon: <LogoutOutlined />,
                  label: '退出登录',
                  onClick: () => {
                    logout()
                    message.success('已退出')
                    nav('/login')
                  },
                },
              ],
            }}
          >
            <div className="user-trigger">
              <Avatar
                size={32}
                style={{ background: 'linear-gradient(135deg,#1677ff,#0958d9)' }}
                icon={<UserOutlined />}
              />
              <span style={{ fontWeight: 500 }}>{user?.username}</span>
              {user && <Tag color={roleColor(user.role)} bordered={false}>{roleLabel(user.role)}</Tag>}
            </div>
          </Dropdown>
        </Header>
        <Content className="app-content">
          {!canWrite && (
            <div style={{ marginBottom: 12, color: '#d46b08' }}>当前为只读账号，仅可查看，无法修改配置。</div>
          )}
          <Routes>
            <Route path="/" element={<Dashboard />} />
            <Route path="/monitor" element={<Monitor />} />
            <Route path="/clients" element={<Clients />} />
            <Route path="/topics" element={<Topics />} />
            <Route path="/acls" element={<ACL />} />
            <Route path="/users" element={<Users />} />
            <Route path="*" element={<Navigate to="/" replace />} />
          </Routes>
        </Content>
      </Layout>
    </Layout>
  )
}

export default function App() {
  const { user, loading, bootstrap } = useAuth()
  if (loading) {
    return (
      <div style={{ display: 'flex', justifyContent: 'center', alignItems: 'center', minHeight: '100vh' }}>
        <Spin size="large" />
      </div>
    )
  }
  // 启动时未检测到配置文件：先展示初始配置页，保存后自动跳转登录页。
  if (bootstrap?.needs_bootstrap) {
    return <Bootstrap configFile={bootstrap.config_file} />
  }
  return (
    <Routes>
      <Route path="/login" element={user ? <Navigate to="/" replace /> : <Login />} />
      <Route
        path="/*"
        element={user ? <Shell /> : <Navigate to="/login" replace />}
      />
    </Routes>
  )
}
