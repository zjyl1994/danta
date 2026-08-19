import axios, { AxiosError } from 'axios'

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

export function getToken(): string | null {
  return sessionStorage.getItem('token')
}

export function setToken(t: string) {
  sessionStorage.setItem('token', t)
}

export function clearToken() {
  sessionStorage.removeItem('token')
}

api.interceptors.request.use((cfg) => {
  const t = getToken()
  if (t) cfg.headers.Authorization = `Bearer ${t}`
  return cfg
})

api.interceptors.response.use(
  (r) => r,
  (err: AxiosError) => {
    const url: string = err.config?.url ?? ''
    const status = err.response?.status
    if (status === 401 && !url.includes('/login')) {
      clearToken()
    }
    const data = (err.response?.data ?? {}) as { code?: string; message?: string }
    return Promise.reject(new ApiError(data.code ?? 'error', data.message ?? err.message ?? '请求失败', status))
  }
)
