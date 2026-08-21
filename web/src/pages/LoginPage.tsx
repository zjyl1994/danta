import { useState } from 'react'
import { Alert, Button, Checkbox, FormControlLabel, Stack, TextField } from '@mui/material'
import { api, isRemember, setAuth, setRemember } from '../api'
import type { LoginResp } from '../types'
import AuthPageLayout from '../components/AuthPageLayout'

export default function LoginPage({ onSuccess }: { onSuccess: () => void }) {
  const [pw, setPw] = useState('')
  const [remember, setRememberState] = useState<boolean>(() => isRemember())
  const [err, setErr] = useState('')
  const [retry, setRetry] = useState<number>(0)

  const submit = async () => {
    setErr('')
    try {
      const r = await api.post<LoginResp>('/login', { password: pw })
      setRemember(remember)
      setAuth(r.data.token, r.data.refresh_token)
      onSuccess()
    } catch (e: any) {
      if (e.retry_after) setRetry(e.retry_after)
      setErr(e.message)
    }
  }

  return (
    <AuthPageLayout title="登录">
      <Stack spacing={2}>
        <TextField
          label="密码"
          type="password"
          value={pw}
          onChange={(e) => setPw(e.target.value)}
          onKeyDown={(e) => e.key === 'Enter' && submit()}
          fullWidth
        />
        <FormControlLabel
          control={<Checkbox checked={remember} onChange={(e) => setRememberState(e.target.checked)} />}
          label="在此设备上保持登录"
        />
        {err && <Alert severity="error">{err}</Alert>}
        {retry > 0 && <Alert severity="warning">尝试次数过多，请 {retry} 秒后再试</Alert>}
        <Button variant="contained" onClick={submit}>
          登录
        </Button>
      </Stack>
    </AuthPageLayout>
  )
}
