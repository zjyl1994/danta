import { createContext, useCallback, useContext, useEffect, useState } from 'react'
import { api } from '../../api'
import type { SettingsResp } from '../../types'

interface SettingsCtx {
  s: SettingsResp | null
  secret: string
  msg: string
  err: string
  uploadKey: string
  setSecret: (v: string) => void
  setMsg: (v: string) => void
  setErr: (v: string) => void
  update: (k: keyof SettingsResp, v: number | string) => void
  save: () => Promise<void>
  testR2: () => Promise<void>
  changePassword: (oldPw: string, newPw: string) => Promise<void>
  resetUploadKey: () => Promise<void>
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
  const [uploadKey, setUploadKey] = useState('')

  const load = useCallback(async () => {
    try {
      const r = await api.get<SettingsResp>('/admin/settings')
      setS(r.data)
      setErr('')
    } catch (e: any) {
      setErr(e.message)
    }
  }, [])

  useEffect(() => {
    void load()
    void api
      .get<{ upload_key: string }>('/admin/upload-key')
      .then((r) => setUploadKey(r.data.upload_key))
      .catch(() => {})
  }, [load])

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
      login_ban_seconds: s.login_ban_seconds
    }
    if (secret) body.r2_secret_access_key = secret
    try {
      const r = await api.post<SettingsResp>('/admin/settings', body)
      setS(r.data)
      setSecret('')
      setMsg('已保存')
    } catch (e: any) {
      setErr(e.message)
    }
  }

  const testR2 = async () => {
    setErr('')
    setMsg('')
    try {
      await api.post('/admin/settings/test-r2')
      setMsg('R2 连接正常')
    } catch (e: any) {
      setErr('R2 连接失败：' + e.message)
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

  const resetUploadKey = async () => {
    if (!window.confirm('重置后旧上传 Key 立即失效，确认？')) return
    try {
      const r = await api.post<{ upload_key: string }>('/admin/upload-key')
      setUploadKey(r.data.upload_key)
      setMsg('上传 Key 已重置')
    } catch (e: any) {
      setErr(e.message)
    }
  }

  return (
    <Ctx.Provider
      value={{ s, secret, msg, err, uploadKey, setSecret, setMsg, setErr, update, save, testR2, changePassword, resetUploadKey }}
    >
      {children}
    </Ctx.Provider>
  )
}
