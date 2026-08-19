import { useState } from 'react'
import { Alert, Box, Button, Card, CardContent, Stack, TextField, Typography } from '@mui/material'
import { api, setToken } from '../api'

export default function LoginPage({ onSuccess }: { onSuccess: () => void }) {
  const [pw, setPw] = useState('')
  const [err, setErr] = useState('')
  const [retry, setRetry] = useState<number>(0)

  const submit = async () => {
    setErr('')
    try {
      const r = await api.post<{ token: string }>('/login', { password: pw })
      setToken(r.data.token)
      onSuccess()
    } catch (e: any) {
      if (e.retry_after) setRetry(e.retry_after)
      setErr(e.message)
    }
  }

  return (
    <Box sx={{ display: 'flex', justifyContent: 'center', alignItems: 'center', minHeight: '100vh' }}>
      <Card sx={{ minWidth: 360 }}>
        <CardContent>
          <Typography variant="h5" gutterBottom>
            登录
          </Typography>
          <Stack spacing={2}>
            <TextField
              label="密码"
              type="password"
              value={pw}
              onChange={(e) => setPw(e.target.value)}
              onKeyDown={(e) => e.key === 'Enter' && submit()}
              fullWidth
            />
            {err && <Alert severity="error">{err}</Alert>}
            {retry > 0 && <Alert severity="warning">已触发封禁，请 {retry} 秒后重试</Alert>}
            <Button variant="contained" onClick={submit}>
              登录
            </Button>
          </Stack>
        </CardContent>
      </Card>
    </Box>
  )
}
