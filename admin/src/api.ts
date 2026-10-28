import axios, { AxiosError } from 'axios'

// ---- 与 Go 后端共享的类型 ----

export interface User {
  id: number
  username: string
  role: 'superadmin' | 'admin' | 'readonly' | 'mqtt'
  enabled: boolean
  remark: string
  created_at: string
  updated_at: string
}

export interface ACLRule {
  id: number
  name: string
  client_id: string
  username: string
  topic: string
  access: 'publish' | 'subscribe' | 'both'
  effect: 'allow' | 'deny'
  max_qos: number
  priority: number
  enabled: boolean
  created_at: string
  updated_at: string
}

export interface ClientDevice {
  id: string
  name: string
  description: string
  enabled: boolean
  created_at: string
  updated_at: string
}

export interface Topic {
  id: number
  topic: string
  description: string
  enabled: boolean
  created_at: string
  updated_at: string
}

export interface ConnectedClient {
  client_id: string
  username: string
  remote: string
  listener: string
  protocol: string
  proto_version: number
  keepalive: number
  clean_session: boolean
  connected_at: string
  subscriptions: string[]
}

export interface MessageLog {
  id: number
  ts: string
  client_id: string
  username: string
  topic: string
  qos: number
  retained: boolean
  size: number
  direction: string
}

export interface StatsSnapshot {
  id: number
  ts: string
  clients_connected: number
  subscriptions: number
  messages_received: number
  messages_sent: number
  retained: number
}

export interface Dashboard {
  users: number
  acl_rules: number
  registered: number
  managed_topics: number
  connected: number
  broker: Record<string, number>
}

// 引导模式：启动时未检测到配置文件，由页面提交基础运行配置
export interface BootstrapStatus {
  needs_bootstrap: boolean
  config_file: string
}

// 引导模式：仅需人工决定的四项，其余配置使用后端默认值，JWT 密钥自动生成
export interface BootstrapConfig {
  allow_anonymous: boolean
  default_acl_allow: boolean
  admin_user: string
  admin_pass: string
}

// ---- axios 封装 ----

export const TOKEN_KEY = 'mqtt_proxy_token'
export const USER_KEY = 'mqtt_proxy_user'

const http = axios.create({ baseURL: '/api', timeout: 15000 })

http.interceptors.request.use((cfg) => {
  const t = localStorage.getItem(TOKEN_KEY)
  if (t) cfg.headers.Authorization = `Bearer ${t}`
  return cfg
})

http.interceptors.response.use(
  (res) => res,
  (err: AxiosError<any>) => {
    if (err.response?.status === 401) {
      localStorage.removeItem(TOKEN_KEY)
      localStorage.removeItem(USER_KEY)
      if (window.location.pathname !== '/login') window.location.href = '/login'
    }
    return Promise.reject(err)
  },
)

// 解开 {code,msg,data} 封装，遇业务错误时抛出。
function unwrap<T>(res: { data: { code: number; msg?: string; data: T } }): T {
  if (res.data.code !== 0) throw new Error(res.data.msg || '请求失败')
  return res.data.data
}

function errMsg(e: any): string {
  return e?.response?.data?.msg || e?.message || '未知错误'
}

export const api = {
  errMsg,

  login: async (username: string, password: string) =>
    unwrap(await http.post('/login', { username, password })),

  // 引导模式
  bootstrapStatus: async () => unwrap<BootstrapStatus>(await http.get('/bootstrap/status')),
  bootstrapSave: (cfg: BootstrapConfig) => http.post('/bootstrap/save', cfg).then(unwrap),

  me: async () => unwrap<User>(await http.get('/me')),
  dashboard: async () => unwrap<Dashboard>(await http.get('/dashboard')),

  // 用户
  listUsers: async (includeMQTT = false) =>
    unwrap<User[]>(await http.get('/users', { params: { include_mqtt: includeMQTT ? '1' : '' } })),
  createUser: (u: Partial<User> & { password: string }) => http.post('/users', u).then(unwrap),
  updateUser: (id: number, u: Partial<User>) => http.put(`/users/${id}`, u).then(unwrap),
  deleteUser: (id: number) => http.delete(`/users/${id}`).then(unwrap),
  resetPassword: (id: number, password: string) =>
    http.post(`/users/${id}/password`, { password }).then(unwrap),

  // ACL
  listACL: async () => unwrap<ACLRule[]>(await http.get('/acls')),
  createACL: (r: Partial<ACLRule>) => http.post('/acls', r).then(unwrap),
  updateACL: (id: number, r: Partial<ACLRule>) => http.put(`/acls/${id}`, r).then(unwrap),
  deleteACL: (id: number) => http.delete(`/acls/${id}`).then(unwrap),

  // 客户端
  listClients: async () => unwrap<ClientDevice[]>(await http.get('/clients')),
  createClient: (c: Partial<ClientDevice> & { id: string }) => http.post('/clients', c).then(unwrap),
  updateClient: (id: string, c: Partial<ClientDevice>) => http.put(`/clients/${id}`, c).then(unwrap),
  deleteClient: (id: string) => http.delete(`/clients/${id}`).then(unwrap),
  disconnectClient: (id: string) => http.post(`/clients/${id}/disconnect`).then(unwrap),

  // 主题
  listTopics: async () => unwrap<Topic[]>(await http.get('/topics')),
  createTopic: (t: Partial<Topic> & { topic: string }) => http.post('/topics', t).then(unwrap),
  updateTopic: (id: number, t: Partial<Topic>) => http.put(`/topics/${id}`, t).then(unwrap),
  deleteTopic: (id: number) => http.delete(`/topics/${id}`).then(unwrap),

  // 监控
  info: async () => unwrap<Record<string, number>>(await http.get('/monitor/info')),
  history: async (limit = 120) => unwrap<StatsSnapshot[]>(await http.get('/monitor/history', { params: { limit } })),
  messages: async (limit = 50) => unwrap<MessageLog[]>(await http.get('/monitor/messages', { params: { limit } })),
  liveTopics: async () => unwrap<Record<string, number>>(await http.get('/monitor/topics')),
  connected: async () => unwrap<ConnectedClient[]>(await http.get('/monitor/clients')),
  publish: (topic: string, payload: string, qos: number, retain: boolean) =>
    http.post('/monitor/publish', { topic, payload, qos, retain }).then(unwrap),
}

export function wsURL(): string {
  const proto = window.location.protocol === 'https:' ? 'wss' : 'ws'
  const token = localStorage.getItem(TOKEN_KEY) || ''
  return `${proto}://${window.location.host}/api/monitor/ws?token=${encodeURIComponent(token)}`
}
