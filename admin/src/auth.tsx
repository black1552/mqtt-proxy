import React, { createContext, useContext, useEffect, useState } from 'react'
import { api, TOKEN_KEY, USER_KEY, type BootstrapStatus, type User } from './api'

interface AuthState {
  user: User | null
  loading: boolean
  bootstrap: BootstrapStatus | null // 引导模式状态（启动时未检测到配置文件）
  login: (username: string, password: string) => Promise<void>
  logout: () => void
  refresh: () => Promise<void>
}

const Ctx = createContext<AuthState>(null as unknown as AuthState)

export const useAuth = () => useContext(Ctx)

export function AuthProvider({ children }: { children: React.ReactNode }) {
  const [user, setUser] = useState<User | null>(null)
  const [loading, setLoading] = useState(true)
  const [bootstrap, setBootstrap] = useState<BootstrapStatus | null>(null)

  const refresh = async () => {
    // 优先检测引导模式：后端启动时未发现配置文件时，先展示初始配置页。
    try {
      const st = await api.bootstrapStatus()
      setBootstrap(st)
      if (st.needs_bootstrap) {
        setUser(null)
        setLoading(false)
        return
      }
    } catch {
      // 旧版本后端无此接口：视为无需引导，按正常流程继续。
      setBootstrap({ needs_bootstrap: false, config_file: '' })
    }
    if (!localStorage.getItem(TOKEN_KEY)) {
      setUser(null)
      setLoading(false)
      return
    }
    try {
      setUser(await api.me())
    } catch {
      setUser(null)
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    refresh()
  }, [])

  const login = async (username: string, password: string) => {
    const res: any = await api.login(username, password)
    localStorage.setItem(TOKEN_KEY, res.token)
    localStorage.setItem(USER_KEY, JSON.stringify(res.user))
    setUser(res.user)
  }

  const logout = () => {
    localStorage.removeItem(TOKEN_KEY)
    localStorage.removeItem(USER_KEY)
    setUser(null)
  }

  return (
    <Ctx.Provider value={{ user, loading, bootstrap, login, logout, refresh }}>
      {children}
    </Ctx.Provider>
  )
}
