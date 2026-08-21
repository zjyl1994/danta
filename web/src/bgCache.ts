const KEY = 'danta.background_url'

// 读取缓存的背景图片 URL（未缓存返回空串）
export function loadBackgroundUrl(): string {
  try {
    return localStorage.getItem(KEY) ?? ''
  } catch {
    return ''
  }
}

// 保存背景图片 URL（空串=清除）
export function saveBackgroundUrl(url: string) {
  try {
    localStorage.setItem(KEY, url)
  } catch {
    /* ignore */
  }
}
