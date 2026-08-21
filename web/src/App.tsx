import { createContext, useCallback, useEffect, useState } from 'react'
import Box from '@mui/material/Box'
import CircularProgress from '@mui/material/CircularProgress'
import { BrowserRouter, Routes, Route, Navigate } from 'react-router-dom'
import { api } from './api'
import { loadBackgroundUrl, saveBackgroundUrl } from './bgCache'
import type { StatusResp } from './types'
import Layout from './components/Layout'
import SetupPage from './pages/SetupPage'
import LoginPage from './pages/LoginPage'
import HomePage from './pages/HomePage'
import ManagePage from './pages/ManagePage'
import SettingsPage from './pages/settings/SettingsPage'
import { SettingsProvider } from './pages/settings/context'

// AppCtx 提供启动状态与刷新能力（自定义背景保存后即时生效）
export interface AppCtxValue {
  cfg: StatusResp | null
  refresh: () => Promise<void>
}

export const AppCtx = createContext<AppCtxValue>({ cfg: null, refresh: async () => {} })

const Spinner = ({ backgroundUrl }: { backgroundUrl?: string }) => (
  <Box sx={{ position: 'relative', minHeight: '100vh' }}>
    {backgroundUrl && (
      <>
        <Box sx={{ position: 'absolute', inset: 0, backgroundImage: `url(${backgroundUrl})`, backgroundSize: 'cover', backgroundPosition: 'center' }} />
        <Box sx={{ position: 'absolute', inset: 0, bgcolor: 'rgba(255,255,255,0.5)' }} />
      </>
    )}
    <Box sx={{ position: 'relative', zIndex: 1, display: 'flex', justifyContent: 'center', alignItems: 'center', minHeight: '100vh' }}>
      <CircularProgress />
    </Box>
  </Box>
)

export default function App() {
  const [state, setState] = useState<{ loaded: boolean; configured: boolean; authed: boolean; cfg: StatusResp | null }>({
    loaded: false,
    configured: false,
    authed: false,
    // 先用 localStorage 缓存填充 background_url，让背景在 /api/status 返回前就开始加载
    cfg: { background_url: loadBackgroundUrl() } as StatusResp
  })

  const refresh = useCallback(async () => {
    try {
      const r = await api.get<StatusResp>('/status')
      saveBackgroundUrl(r.data.background_url)
      setState({ loaded: true, configured: r.data.configured, authed: r.data.authed, cfg: r.data })
    } catch {
      // 拉取失败时保留缓存背景，避免页面无背景
      setState({ loaded: true, configured: false, authed: false, cfg: { background_url: loadBackgroundUrl() } as StatusResp })
    }
  }, [])

  useEffect(() => {
    void refresh()
  }, [refresh])

  const value: AppCtxValue = { cfg: state.cfg, refresh }

  return (
    <AppCtx.Provider value={value}>
      {!state.loaded ? (
        <Spinner backgroundUrl={state.cfg?.background_url ?? ''} />
      ) : !state.configured ? (
        <SetupPage />
      ) : !state.authed ? (
        <LoginPage onSuccess={() => setState((s) => ({ ...s, authed: true }))} />
      ) : (
        <BrowserRouter>
          <Layout onLogout={() => setState((s) => ({ ...s, authed: false }))}>
            <Routes>
              <Route path="/" element={<HomePage cfg={state.cfg!} />} />
              <Route path="/manage" element={<ManagePage />} />
              <Route path="/stats" element={<Navigate to="/manage" replace />} />
              <Route path="/settings" element={<Navigate to="/settings/storage" replace />} />
              <Route
                path="/settings/:section"
                element={
                  <SettingsProvider>
                    <SettingsPage />
                  </SettingsProvider>
                }
              />
              <Route path="*" element={<HomePage cfg={state.cfg!} />} />
            </Routes>
          </Layout>
        </BrowserRouter>
      )}
    </AppCtx.Provider>
  )
}
