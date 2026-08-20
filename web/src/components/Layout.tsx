import React, { useState } from 'react'
import { useLocation, useNavigate } from 'react-router-dom'
import Box from '@mui/material/Box'
import Button from '@mui/material/Button'
import Collapse from '@mui/material/Collapse'
import Divider from '@mui/material/Divider'
import Drawer from '@mui/material/Drawer'
import Fab from '@mui/material/Fab'
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
import DownloadIcon from '@mui/icons-material/Download'
import { clearToken } from '../api'

const DRAWER_WIDTH = 220

const TOP_ITEMS = [
  { p: '/', l: '上传', icon: <CloudUploadIcon /> },
  { p: '/manage', l: '图片', icon: <PhotoLibraryIcon /> }
]

const SETTINGS_CHILDREN = [
  { p: '/settings/storage', l: '存储', icon: <StorageIcon /> },
  { p: '/settings/security', l: '安全', icon: <SecurityIcon /> },
  { p: '/settings/api', l: 'API', icon: <ApiIcon /> },
  { p: '/settings/migrate', l: '迁移', icon: <DownloadIcon /> }
]

export default function Layout({ children, onLogout }: { children: React.ReactNode; onLogout: () => void }) {
  const nav = useNavigate()
  const loc = useLocation()
  const [settingsOpen, setSettingsOpen] = useState(loc.pathname.startsWith('/settings'))
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
        <Typography variant="h6" sx={{ flexGrow: 1 }}>
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

  return (
    <Box sx={{ display: 'flex' }}>
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

      {/* 移动端浮动菜单按钮（无顶部标题栏） */}
      <Fab
        size="small"
        color="primary"
        aria-label="menu"
        sx={{ position: 'fixed', top: 12, left: 12, zIndex: 1200, display: { md: 'none' } }}
        onClick={() => setMobileOpen(true)}
      >
        <MenuIcon />
      </Fab>

      <Box component="main" sx={{ flexGrow: 1, width: '100%', pt: { xs: 6, sm: 3 }, px: { xs: 1, sm: 3 }, maxWidth: 1200, mx: 'auto' }}>
        {children}
      </Box>
    </Box>
  )
}
