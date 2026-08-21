import { createContext, useCallback, useContext, useEffect, useState } from 'react'
import { api } from '../../api'
import { AppCtx } from '../../App'
import type { NewTokenResp, SessionItem, SettingsResp } from '../../types'

interface SettingsCtx {
  s: SettingsResp | null
  secret: string
  msg: string
  err: string
  sessions: SessionItem[]
  setSecret: (v: string) => void
  setMsg: (v: string) => void
  setErr: (v: string) => void
  update: (k: keyof SettingsResp, v: number | string) => void
  save: () => Promise<void>
  testR2: () => Promise<void>
  changePassword: (oldPw: string, newPw: string) => Promise<void>
  loadSessions: () => Promise<void>
  createUploadToken: (name: string, days: number) => Promise<NewTokenResp>
  revokeSession: (id: number) => Promise<void>
}

const Ctx = createContext<SettingsCtx | null>(null)

export function useSettingsCtx(): SettingsCtx {
  const c = useContext(Ctx)
  if (!c) throw new Error('useSettingsCtx must be used within SettingsProvider')
  return c
}

export function SettingsProvider({ children }: { children: React.ReactNode }) {
  const [s, setS] = useState<SettingsResp | null>(null)
  const [secret, setSecret] = useState('')
  const [msg, setMsg] = useState('')
  const [err, setErr] = useState('')
  const [sessions, setSessions] = useState<SessionItem[]>([])
  const { refresh } = useContext(AppCtx)

  const load = useCallback(async () => {
    try {
      const r = await api.get<SettingsResp>('/admin/settings')
      setS(r.data)
      setErr('')
    } catch (e: any) {
      setErr(e.message)
    }
  }, [])

  const loadSessions = useCallback(async () => {
    try {
      const r = await api.get<{ sessions: SessionItem[] }>('/admin/sessions')
      setSessions(r.data.sessions)
    } catch {
      /* ignore */
    }
  }, [])

  useEffect(() => {
    void load()
    void loadSessions()
  }, [load, loadSessions])

  const update = (k: keyof SettingsResp, v: number | string) => setS((p) => (p ? { ...p, [k]: v } : p))

  const save = async () => {
    setErr('')
    setMsg('')
    if (!s) return
    const body: Record<string, unknown> = {
      cdn_host: s.cdn_host,
      proxy_mode: s.proxy_mode,
      r2_endpoint: s.r2_endpoint,
      r2_access_key_id: s.r2_access_key_id,
      r2_bucket: s.r2_bucket,
      max_upload_bytes: s.max_upload_bytes,
      webp_quality: s.webp_quality,
      resize_max_dim: s.resize_max_dim,
      cache_max_age: s.cache_max_age,
      login_fail_limit: s.login_fail_limit,
      login_fail_window: s.login_fail_window,
      login_ban_seconds: s.login_ban_seconds,
      session_ttl: s.session_ttl,
      background_image: s.background_image
    }
    if (secret) body.r2_secret_access_key = secret
    try {
      const r = await api.post<SettingsResp>('/admin/settings', body)
      setS(r.data)
      setSecret('')
      setMsg('已保存')
      void refresh()
    } catch (e: any) {
      setErr(e.message)
    }
  }

  const testR2 = async () => {
    setErr('')
    setMsg('')
    try {
      await api.post('/admin/settings/test-r2')
      setMsg('连接正常')
    } catch (e: any) {
      setErr('连接失败：' + e.message)
    }
  }

  const changePassword = async (oldPw: string, newPw: string) => {
    setErr('')
    setMsg('')
    try {
      await api.post('/admin/settings', { old_password: oldPw, new_password: newPw })
      setMsg('密码已修改')
    } catch (e: any) {
      setErr(e.message)
    }
  }

  const createUploadToken = async (name: string, days: number) => {
    setErr('')
    setMsg('')
    try {
      const r = await api.post<NewTokenResp>('/admin/tokens', { name, days })
      await loadSessions()
      setMsg('令牌已创建，请立即复制保存（仅显示一次）')
      return r.data
    } catch (e: any) {
      setErr(e.message)
      throw e
    }
  }

  const revokeSession = async (id: number) => {
    setErr('')
    setMsg('')
    try {
      await api.post(`/admin/sessions/${id}/revoke`)
      await loadSessions()
      setMsg('已吊销')
    } catch (e: any) {
      setErr(e.message)
    }
  }

  return (
    <Ctx.Provider
      value={{
        s, secret, msg, err, sessions,
        setSecret, setMsg, setErr,
        update, save, testR2, changePassword,
        loadSessions, createUploadToken, revokeSession
      }}
    >
      {children}
    </Ctx.Provider>
  )
}
