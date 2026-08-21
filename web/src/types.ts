export interface StatusResp {
  configured: boolean
  authed: boolean
  resize_max_dim: number
  webp_quality: number
  max_upload_bytes: number
  background_url: string
}

export interface UploadResp {
  id: number
  original: boolean
  url: string
  size: number
  width: number
  height: number
  mime: string
  created_at: string
}

export interface ImageItem {
  id: number
  objectkey: string
  name: string
  original: boolean
  mime: string
  size: number
  width: number
  height: number
  created_at: string
  url: string
}

export interface ListResp {
  items: ImageItem[]
  total: number
  page: number
  size: number
}

export interface StatsResp {
  images: number
  total_size: number
  uploads_24h: number
  originals: number
}

export interface SettingsResp {
  cdn_host: string
  proxy_mode: string
  max_upload_bytes: number
  webp_quality: number
  resize_max_dim: number
  cache_max_age: number
  r2_endpoint: string
  r2_access_key_id: string
  r2_secret_access_key: string
  r2_bucket: string
  login_fail_limit: number
  login_fail_window: number
  login_ban_seconds: number
  session_ttl: number
  has_password: boolean
  background_image: string
  background_url: string
}

export interface LoginResp {
  token: string
  refresh_token: string
  device_id: number
  expires_at: string
}

export interface SessionItem {
  id: number
  kind: 'login' | 'upload'
  name: string
  created_at: string
  last_used_at: string | null
  expires_at: string | null
  revoked_at: string | null
}

export interface NewTokenResp {
  id: number
  token: string
  name: string
  expires_at: string | null
}

export interface ScanItem {
  key: string
  size: number
  last_modified: string
}

export interface ScanResp {
  total: number
  new: number
  existing: number
  ignored: number
  items: ScanItem[]
}

export interface RunResp {
  imported: number
  skipped: number
  ignored: number
  errors: string[]
}
