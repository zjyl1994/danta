import axios, { AxiosError, InternalAxiosRequestConfig } from 'axios'

export class ApiError extends Error {
  code: string
  status?: number
  constructor(code: string, message: string, status?: number) {
    super(message)
    this.code = code
    this.status = status
  }
}

export const api = axios.create({ baseURL: '/api' })

// ---- 令牌存储：勾选"记住我"存 localStorage（跨会话），否则 sessionStorage ----
const K_ACCESS = 'danta.token'
const K_REFRESH = 'danta.refresh'
const K_REMEMBER = 'danta.remember'

export function isRemember(): boolean {
  return localStorage.getItem(K_REMEMBER) !== '0'
}

export function setRemember(v: boolean) {
  localStorage.setItem(K_REMEMBER, v ? '1' : '0')
}

function mem(): Storage {
  return isRemember() ? localStorage : sessionStorage
}

export function getToken(): string | null {
  return mem().getItem(K_ACCESS)
}

export function getRefreshToken(): string | null {
  return mem().getItem(K_REFRESH)
}

export function setAuth(access: string, refresh: string) {
  mem().setItem(K_ACCESS, access)
  mem().setItem(K_REFRESH, refresh)
}

function setAccess(t: string) {
  mem().setItem(K_ACCESS, t)
}

export function clearToken() {
  localStorage.removeItem(K_ACCESS)
  localStorage.removeItem(K_REFRESH)
  sessionStorage.removeItem(K_ACCESS)
  sessionStorage.removeItem(K_REFRESH)
}

// ---- 静默续期 ----
function tokenExpired(): boolean {
  const t = getToken()
  if (!t) return true
  try {
    const p = JSON.parse(atob(t.split('.')[1]))
    return Date.now() >= (p.exp ?? 0) * 1000
  } catch {
    return true
  }
}

export async function refreshAccessToken(): Promise<string | null> {
  const rt = getRefreshToken()
  if (!rt) return null
  try {
    const r = await axios.post<{ token: string }>('/api/refresh', { refresh_token: rt })
    setAccess(r.data.token)
    return r.data.token
  } catch (e) {
    if ((e as AxiosError).response) clearToken()
    return null
  }
}

// 启动/鉴权前调用：刷新令牌存在且访问令牌缺失或过期时静默续期
export async function ensureAuth(): Promise<void> {
  if (getRefreshToken() && tokenExpired()) await refreshAccessToken()
}

let refreshing: Promise<string | null> | null = null

api.interceptors.request.use((cfg) => {
  const t = getToken()
  if (t) cfg.headers.Authorization = `Bearer ${t}`
  return cfg
})

api.interceptors.response.use(
  (r) => r,
  async (err: AxiosError) => {
    const cfg = err.config as (InternalAxiosRequestConfig & { _retried?: boolean }) | undefined
    const url: string = cfg?.url ?? ''
    const status = err.response?.status
    const authOk =
      cfg &&
      status === 401 &&
      !url.includes('/login') &&
      !url.includes('/refresh') &&
      !cfg._retried &&
      getRefreshToken()
    if (authOk) {
      if (!refreshing) refreshing = refreshAccessToken()
      const tok = await refreshing
      refreshing = null
      if (tok) {
        cfg._retried = true
        cfg.headers.Authorization = `Bearer ${tok}`
        return api.request(cfg)
      }
    }
    if (status === 401 && !url.includes('/login') && !url.includes('/refresh')) {
      clearToken()
      window.dispatchEvent(new Event('danta:auth-expired'))
    }
    const data = (err.response?.data ?? {}) as { code?: string; message?: string }
    return Promise.reject(new ApiError(data.code ?? 'error', data.message ?? err.message ?? '请求失败', status))
  }
)
