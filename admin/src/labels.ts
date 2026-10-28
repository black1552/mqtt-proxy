// 与 Go 后端共享的枚举取值 -> 中文标签 / 颜色的统一映射。
// 表格里展示、表单下拉选项都从这里取，保证全站文案一致。

// ---- 用户角色 ----
export const ROLE_LABELS: Record<string, string> = {
  superadmin: '超级管理员',
  admin: '管理员',
  readonly: '只读',
  mqtt: '设备账号',
}

export const ROLE_COLORS: Record<string, string> = {
  superadmin: 'volcano',
  admin: 'blue',
  readonly: 'orange',
  mqtt: 'green',
}

export const roleLabel = (r: string) => ROLE_LABELS[r] ?? r
export const roleColor = (r: string) => ROLE_COLORS[r] ?? 'default'

// ---- ACL 动作 ----
export const ACCESS_LABELS: Record<string, string> = {
  publish: '发布',
  subscribe: '订阅',
  both: '发布/订阅',
}

export const ACCESS_COLORS: Record<string, string> = {
  publish: 'geekblue',
  subscribe: 'purple',
  both: 'cyan',
}

export const accessLabel = (a: string) => ACCESS_LABELS[a] ?? a
export const accessColor = (a: string) => ACCESS_COLORS[a] ?? 'default'

// ---- ACL 效果 ----
export const EFFECT_LABELS: Record<string, string> = {
  allow: '允许',
  deny: '拒绝',
}

export const EFFECT_COLORS: Record<string, string> = {
  allow: 'success',
  deny: 'error',
}

export const effectLabel = (e: string) => EFFECT_LABELS[e] ?? e
export const effectColor = (e: string) => EFFECT_COLORS[e] ?? 'default'

// 供表单下拉使用的选项列表。
export const roleOptions = (values: string[]) =>
  values.map((v) => ({ label: ROLE_LABELS[v] ?? v, value: v }))

export const accessOptions = (['publish', 'subscribe', 'both'] as const).map((v) => ({
  label: ACCESS_LABELS[v],
  value: v,
}))

export const effectOptions = (['allow', 'deny'] as const).map((v) => ({
  label: EFFECT_LABELS[v],
  value: v,
}))
