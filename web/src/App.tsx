import { useEffect, useState } from 'react'
import Box from '@mui/material/Box'
import CircularProgress from '@mui/material/CircularProgress'
import { BrowserRouter, Routes, Route, Navigate } from 'react-router-dom'
import { api } from './api'
import type { StatusResp } from './types'
import Layout from './components/Layout'
import SetupPage from './pages/SetupPage'
import LoginPage from './pages/LoginPage'
import HomePage from './pages/HomePage'
import ManagePage from './pages/ManagePage'
import SettingsPage from './pages/settings/SettingsPage'
import { SettingsProvider } from './pages/settings/context'

const Spinner = () => (
  <Box sx={{ display: 'flex', justifyContent: 'center', alignItems: 'center', height: '100vh' }}>
    <CircularProgress />
  </Box>
)

export default function App() {
  const [state, setState] = useState<{ loaded: boolean; configured: boolean; authed: boolean; cfg: StatusResp | null }>({
    loaded: false,
    configured: false,
    authed: false,
    cfg: null
  })

  useEffect(() => {
    api
      .get<StatusResp>('/status')
      .then((r) => setState({ loaded: true, configured: r.data.configured, authed: r.data.authed, cfg: r.data }))
      .catch(() => setState({ loaded: true, configured: false, authed: false, cfg: null }))
  }, [])

  if (!state.loaded) {
    return <Spinner />
  }

  if (!state.configured) {
    return <SetupPage />
  }
  if (!state.authed) {
    return <LoginPage onSuccess={() => setState((s) => ({ ...s, authed: true }))} />
  }

  return (
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
  )
}
