import { useState } from 'react'
import { Alert, Button, Stack, TextField, Typography } from '@mui/material'
import { api } from '../api'
import AuthPageLayout from '../components/AuthPageLayout'

export default function SetupPage() {
  const [pw, setPw] = useState('')
  const [pw2, setPw2] = useState('')
  const [err, setErr] = useState('')
  const [done, setDone] = useState(false)

  const submit = async () => {
    setErr('')
    if (pw.length < 8) {
      setErr('密码至少 8 位')
      return
    }
    if (pw !== pw2) {
      setErr('两次密码不一致')
      return
    }
    try {
      await api.post('/setup', { password: pw })
      setDone(true)
    } catch (e: any) {
      setErr(e.message)
    }
  }

  return (
    <AuthPageLayout title="蛋挞图床初始化">
      {done ? (
        <Alert severity="success">初始化完成，请刷新页面登录</Alert>
      ) : (
        <Stack spacing={2}>
          <Typography variant="body2" color="text.secondary">
            设置管理员密码，完成初始化
          </Typography>
          <TextField label="密码" type="password" value={pw} onChange={(e) => setPw(e.target.value)} fullWidth />
          <TextField label="确认密码" type="password" value={pw2} onChange={(e) => setPw2(e.target.value)} fullWidth />
          {err && <Alert severity="error">{err}</Alert>}
          <Button variant="contained" onClick={submit}>
            初始化
          </Button>
        </Stack>
      )}
    </AuthPageLayout>
  )
}
