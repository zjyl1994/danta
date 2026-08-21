import React, { useContext, useState } from 'react'
import { useLocation, useNavigate } from 'react-router-dom'
import AppBar from '@mui/material/AppBar'
import Box from '@mui/material/Box'
import Button from '@mui/material/Button'
import Collapse from '@mui/material/Collapse'
import Divider from '@mui/material/Divider'
import Drawer from '@mui/material/Drawer'
import IconButton from '@mui/material/IconButton'
import List from '@mui/material/List'
import ListItemButton from '@mui/material/ListItemButton'
import ListItemIcon from '@mui/material/ListItemIcon'
import ListItemText from '@mui/material/ListItemText'
import Toolbar from '@mui/material/Toolbar'
import Typography from '@mui/material/Typography'
import ExpandLess from '@mui/icons-material/ExpandLess'
import ExpandMore from '@mui/icons-material/ExpandMore'
import MenuIcon from '@mui/icons-material/Menu'
import CloudUploadIcon from '@mui/icons-material/CloudUpload'
import PhotoLibraryIcon from '@mui/icons-material/PhotoLibrary'
import SettingsIcon from '@mui/icons-material/Settings'
import LogoutIcon from '@mui/icons-material/Logout'
import StorageIcon from '@mui/icons-material/Storage'
import SecurityIcon from '@mui/icons-material/Security'
import ApiIcon from '@mui/icons-material/Api'
import BuildIcon from '@mui/icons-material/Build'
import ImageIcon from '@mui/icons-material/Image'
import { clearToken } from '../api'
import { AppCtx } from '../App'
import { usePersistentState } from '../usePersistentState'

const DRAWER_WIDTH = 220

const TOP_ITEMS = [
  { p: '/', l: '上传', icon: <CloudUploadIcon /> },
  { p: '/manage', l: '图片', icon: <PhotoLibraryIcon /> }
]

const SETTINGS_CHILDREN = [
  { p: '/settings/storage', l: '存储', icon: <StorageIcon /> },
  { p: '/settings/appearance', l: '外观', icon: <ImageIcon /> },
  { p: '/settings/security', l: '安全', icon: <SecurityIcon /> },
  { p: '/settings/api', l: '开发者', icon: <ApiIcon /> },
  { p: '/settings/maintenance', l: '维护', icon: <BuildIcon /> }
]

export default function Layout({ children, onLogout }: { children: React.ReactNode; onLogout: () => void }) {
  const nav = useNavigate()
  const loc = useLocation()
  const { cfg } = useContext(AppCtx)
  const [settingsOpen, setSettingsOpen] = usePersistentState<boolean>('danta.settings_open', () => loc.pathname.startsWith('/settings'))
  const [mobileOpen, setMobileOpen] = useState(false)

  const logout = () => {
    clearToken()
    nav('/')
    onLogout()
  }

  const goto = (p: string) => {
    nav(p)
    setMobileOpen(false)
  }

  const drawer = (
    <Box>
      <Toolbar>
        <Box component="img" src="/icon-192.png" alt="蛋挞图床" sx={{ width: 32, height: 32, mr: 1.5, borderRadius: 1 }} />
        <Typography variant="h6" noWrap sx={{ flexGrow: 1 }}>
          蛋挞图床
        </Typography>
      </Toolbar>
      <Divider />
      <List component="nav">
        {TOP_ITEMS.map((i) => (
          <ListItemButton key={i.p} selected={loc.pathname === i.p} onClick={() => goto(i.p)}>
            <ListItemIcon>{i.icon}</ListItemIcon>
            <ListItemText primary={i.l} />
          </ListItemButton>
        ))}

        <ListItemButton onClick={() => setSettingsOpen((v) => !v)}>
          <ListItemIcon>
            <SettingsIcon />
          </ListItemIcon>
          <ListItemText primary="设置" />
          {settingsOpen ? <ExpandLess /> : <ExpandMore />}
        </ListItemButton>
        <Collapse in={settingsOpen} timeout="auto" unmountOnExit>
          <List component="div" disablePadding>
            {SETTINGS_CHILDREN.map((i) => (
              <ListItemButton
                key={i.p}
                selected={loc.pathname === i.p}
                onClick={() => goto(i.p)}
                sx={{ pl: 4 }}
              >
                <ListItemIcon sx={{ minWidth: 32 }}>{i.icon}</ListItemIcon>
                <ListItemText primary={i.l} />
              </ListItemButton>
            ))}
          </List>
        </Collapse>
      </List>
      <Box sx={{ position: 'absolute', bottom: 0, width: '100%', p: 1 }}>
        <Button
          fullWidth
          startIcon={<LogoutIcon />}
          color="inherit"
          onClick={logout}
        >
          退出
        </Button>
      </Box>
    </Box>
  )

  const backgroundUrl = cfg?.background_url ?? ''

  return (
    <Box sx={{ position: 'relative', zIndex: 0, display: 'flex', flexDirection: { xs: 'column', md: 'row' } }}>
      {/* 自定义背景（固定图层，置于内容之下） */}
      {backgroundUrl && (
        <>
          <Box
            sx={{
              position: 'fixed',
              inset: 0,
              zIndex: -1,
              backgroundImage: `url(${backgroundUrl})`,
              backgroundSize: 'cover',
              backgroundPosition: 'center'
            }}
          />
          <Box sx={{ position: 'fixed', inset: 0, zIndex: -1, bgcolor: 'rgba(255,255,255,0.5)' }} />
        </>
      )}
      {/* 移动端顶栏（文档流内，不悬浮；桌面端隐藏） */}
      <AppBar position="static" sx={{ display: { md: 'none' } }}>
        <Toolbar>
          <IconButton color="inherit" edge="start" onClick={() => setMobileOpen(true)} sx={{ mr: 1 }}>
            <MenuIcon />
          </IconButton>
          <Typography variant="h6" noWrap sx={{ flexGrow: 1 }}>
            蛋挞图床
          </Typography>
        </Toolbar>
      </AppBar>

      <Box component="nav" sx={{ width: { md: DRAWER_WIDTH }, flexShrink: { md: 0 } }}>
        <Drawer
          variant="temporary"
          open={mobileOpen}
          onClose={() => setMobileOpen(false)}
          ModalProps={{ keepMounted: true }}
          sx={{ display: { xs: 'block', md: 'none' }, '& .MuiDrawer-paper': { boxSizing: 'border-box', width: DRAWER_WIDTH } }}
        >
          {drawer}
        </Drawer>
        <Drawer
          variant="permanent"
          open
          sx={{ display: { xs: 'none', md: 'block' }, '& .MuiDrawer-paper': { boxSizing: 'border-box', width: DRAWER_WIDTH } }}
        >
          {drawer}
        </Drawer>
      </Box>

      <Box component="main" sx={{ flexGrow: 1, width: '100%', p: backgroundUrl ? { xs: 1, sm: 2 } : { xs: 1.5, sm: 3 }, maxWidth: 1200, mx: 'auto' }}>
        {/* 内容面板：半透明表面 + 毛玻璃，背景图只在边缘露出 */}
        {backgroundUrl ? (
          <Box
            sx={{
              bgcolor: 'rgba(255,255,255,0.85)',
              borderRadius: 2,
              border: '1px solid rgba(0,0,0,0.06)',
              boxShadow: '0 2px 12px rgba(0,0,0,0.06)',
              backdropFilter: 'blur(8px)',
              WebkitBackdropFilter: 'blur(8px)',
              p: { xs: 1.5, sm: 3 }
            }}
          >
            {children}
          </Box>
        ) : (
          children
        )}
      </Box>
    </Box>
  )
}
