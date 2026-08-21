import { useEffect, useState } from 'react'
import { useParams } from 'react-router-dom'
import { Alert, Box, Button, Dialog, DialogActions, DialogContent, DialogTitle, Divider, Grid, MenuItem, Paper, Stack, Table, TableBody, TableCell, TableContainer, TableHead, TablePagination, TableRow, TextField, Typography } from '@mui/material'
import ContentCopyIcon from '@mui/icons-material/ContentCopy'
import CopyButton from '../../components/CopyButton'
import { api } from '../../api'
import { copyText } from '../../clipboard'
import { useSettingsCtx } from './context'
import type { ImageItem, RunResp, ScanResp } from '../../types'

const CACHE_OPTIONS = [
  { v: 3600, l: '1 小时' },
  { v: 21600, l: '6 小时' },
  { v: 86400, l: '1 天' },
  { v: 604800, l: '7 天' },
  { v: 2592000, l: '30 天' },
  { v: 31536000, l: '1 年' }
]

function Banner() {
  const { msg, err } = useSettingsCtx()
  return (
    <>
      {err && <Alert severity="error">{err}</Alert>}
      {msg && <Alert severity="success">{msg}</Alert>}
    </>
  )
}

// 存储：对象存储与域名 + 上传限制
function StorageCard() {
  const { s, secret, setSecret, update, save, testR2 } = useSettingsCtx()
  if (!s) return null
  return (
    <Box>
      <Box>
        <Typography variant="subtitle1" gutterBottom>
          存储与域名
        </Typography>
        <Grid container spacing={2}>
          <Grid item xs={12} sm={6}>
            <TextField label="访问域名" value={s.cdn_host} onChange={(e) => update('cdn_host', e.target.value)} fullWidth placeholder="img.example.com" />
          </Grid>
          <Grid item xs={12} sm={6}>
            <TextField label="存储服务地址" value={s.r2_endpoint} onChange={(e) => update('r2_endpoint', e.target.value)} fullWidth placeholder="https://<account>.r2.cloudflarestorage.com" />
          </Grid>
          <Grid item xs={12} sm={6}>
            <TextField label="Access Key ID" value={s.r2_access_key_id} onChange={(e) => update('r2_access_key_id', e.target.value)} fullWidth />
          </Grid>
          <Grid item xs={12} sm={6}>
            <TextField label="访问密钥" type="password" value={secret} onChange={(e) => setSecret(e.target.value)} fullWidth placeholder="留空保持不变（已配置）" />
          </Grid>
          <Grid item xs={12} sm={6}>
            <TextField label="存储桶" value={s.r2_bucket} onChange={(e) => update('r2_bucket', e.target.value)} fullWidth />
          </Grid>
        </Grid>

        <Typography variant="subtitle1" gutterBottom sx={{ mt: 3 }}>
          上传限制
        </Typography>
        <Grid container spacing={2}>
          <Grid item xs={12} sm={3}>
            <TextField label="单张上限 (MB)" type="number" value={Math.round(s.max_upload_bytes / 1024 / 1024)} onChange={(e) => update('max_upload_bytes', Math.max(1, parseInt(e.target.value || '1', 10)) * 1024 * 1024)} fullWidth />
          </Grid>
          <Grid item xs={12} sm={3}>
            <TextField label="压缩质量 (0-100)" type="number" value={s.webp_quality} onChange={(e) => update('webp_quality', Math.min(100, Math.max(1, parseInt(e.target.value || '80', 10))))} fullWidth />
          </Grid>
          <Grid item xs={12} sm={3}>
            <TextField label="图片最长边" type="number" value={s.resize_max_dim} onChange={(e) => update('resize_max_dim', Math.max(1, parseInt(e.target.value || '2560', 10)))} fullWidth />
          </Grid>
          <Grid item xs={12} sm={3}>
            <TextField label="CDN 缓存时长" select value={s.cache_max_age} onChange={(e) => update('cache_max_age', parseInt(e.target.value, 10))} fullWidth>
              {CACHE_OPTIONS.map((o) => (
                <MenuItem key={o.v} value={o.v}>{o.l}</MenuItem>
              ))}
            </TextField>
          </Grid>
        </Grid>

        <Stack direction="row" justifyContent="flex-end" spacing={1} sx={{ mt: 2 }}>
          <Button variant="contained" onClick={() => void save()}>保存</Button>
          <Button variant="outlined" onClick={() => void testR2()}>测试连接</Button>
        </Stack>
      </Box>
    </Box>
  )
}

// 安全：反向代理 / 密码 / 登录防爆破 / 上传 Key
function SecurityCard() {
  const { s, update, save, changePassword } = useSettingsCtx()
  const [oldPw, setOldPw] = useState('')
  const [newPw, setNewPw] = useState('')
  const [newPw2, setNewPw2] = useState('')
  const [pwErr, setPwErr] = useState('')
  const [ip, setIp] = useState('')

  // 实时显示当前请求 IP：模式切换时立即刷新，且每 3 秒轮询一次
  const proxyMode = s?.proxy_mode ?? 'none'
  useEffect(() => {
    const load = () => {
      void api
        .get<{ ip: string }>('/admin/client-ip', { params: { mode: proxyMode } })
        .then((r) => setIp(r.data.ip))
        .catch(() => {})
    }
    load()
    const t = setInterval(load, 3000)
    return () => clearInterval(t)
  }, [proxyMode])

  if (!s) return null

  const doChange = async () => {
    setPwErr('')
    if (newPw.length < 8) return setPwErr('新密码至少 8 位')
    if (newPw !== newPw2) return setPwErr('两次新密码不一致')
    await changePassword(oldPw, newPw)
    setOldPw('')
    setNewPw('')
    setNewPw2('')
  }

  return (
    <Box>
      <Box>
        <Stack spacing={3}>
          <Box>
            <Typography variant="subtitle1" gutterBottom>
              访问来源
            </Typography>
            <Grid container spacing={2}>
              <Grid item xs={12} sm={6}>
                <TextField label="代理模式" select value={s.proxy_mode} onChange={(e) => update('proxy_mode', e.target.value)} fullWidth>
                  <MenuItem value="none">直连（无需代理）</MenuItem>
                  <MenuItem value="local">本机反向代理</MenuItem>
                </TextField>
              </Grid>
              <Grid item xs={12} sm={6}>
                <TextField label="当前请求 IP" value={ip} fullWidth disabled />
              </Grid>
            </Grid>
            <Alert severity="warning" sx={{ mt: 2 }}>
              如果通过反向代理访问，请选择对应的代理模式，否则登录安全限制可能误判您的 IP。
            </Alert>
          </Box>

          <Divider />

          <Box>
            <Typography variant="subtitle1" gutterBottom>
              修改密码
            </Typography>
            <Grid container spacing={2}>
              <Grid item xs={12} sm={4}>
                <TextField label="旧密码" type="password" value={oldPw} onChange={(e) => setOldPw(e.target.value)} fullWidth />
              </Grid>
              <Grid item xs={12} sm={4}>
                <TextField label="新密码" type="password" value={newPw} onChange={(e) => setNewPw(e.target.value)} fullWidth />
              </Grid>
              <Grid item xs={12} sm={4}>
                <TextField label="确认新密码" type="password" value={newPw2} onChange={(e) => setNewPw2(e.target.value)} fullWidth />
              </Grid>
              {pwErr && (
                <Grid item xs={12}>
                  <Alert severity="error">{pwErr}</Alert>
                </Grid>
              )}
            </Grid>
            <Stack justifyContent="flex-end" sx={{ mt: 2 }}>
              <Button variant="contained" sx={{ alignSelf: 'flex-end' }} onClick={() => void doChange()}>
                修改密码
              </Button>
            </Stack>
          </Box>

          <Divider />

          <Box>
            <Typography variant="subtitle1" gutterBottom>
              登录安全限制
            </Typography>
            <Grid container spacing={2}>
              <Grid item xs={12} sm={4}>
                <TextField label="连续错误次数" type="number" value={s.login_fail_limit} onChange={(e) => update('login_fail_limit', Math.max(1, parseInt(e.target.value || '5', 10)))} fullWidth />
              </Grid>
              <Grid item xs={12} sm={4}>
                <TextField label="统计窗口（秒）" type="number" value={s.login_fail_window} onChange={(e) => update('login_fail_window', Math.max(1, parseInt(e.target.value || '900', 10)))} fullWidth />
              </Grid>
              <Grid item xs={12} sm={4}>
                <TextField label="暂停时长（秒）" type="number" value={s.login_ban_seconds} onChange={(e) => update('login_ban_seconds', Math.max(1, parseInt(e.target.value || '900', 10)))} fullWidth />
              </Grid>
            </Grid>
            <Stack justifyContent="flex-end" sx={{ mt: 2 }}>
              <Button variant="contained" sx={{ alignSelf: 'flex-end' }} onClick={() => void save()}>保存</Button>
            </Stack>
          </Box>
        </Stack>
      </Box>
    </Box>
  )
}

// API：上传 Key 管理 + 上传接口示例
function ApiCard() {
  const { uploadKey, resetUploadKey } = useSettingsCtx()
  const origin = window.location.origin
  const key = uploadKey || '<upload_key>'

  const examples = [
    {
      title: '自动压缩后上传（默认）',
      cmd: `curl -X POST ${origin}/api/upload -H "Authorization: Bearer ${key}" -F "file=@image.png"`
    },
    {
      title: '保留原图上传',
      cmd: `curl -X POST ${origin}/api/upload -H "Authorization: Bearer ${key}" -F "file=@image.png" -F "original=true"`
    }
  ]

  return (
    <Box>
      <Box>
        <Stack spacing={3}>
          <Box>
            <Typography variant="subtitle1" gutterBottom>
              上传密钥
            </Typography>
            <Typography variant="body2" color="text.secondary" gutterBottom>
              用于通过命令行等工具上传图片，等同于您的上传权限，请妥善保管。
            </Typography>
            <Stack spacing={1}>
              <TextField value={uploadKey} fullWidth disabled />
              <Stack direction="row" justifyContent="flex-end" spacing={1}>
                <Button
                  variant="outlined"
                  startIcon={<ContentCopyIcon />}
                  onClick={() => void copyText(uploadKey)}
                >
                  复制密钥
                </Button>
                <Button variant="outlined" color="error" onClick={() => void resetUploadKey()}>重置</Button>
              </Stack>
            </Stack>
          </Box>

          <Divider />

          <Box>
            <Typography variant="subtitle1" gutterBottom>
              命令行上传示例
            </Typography>
            <Typography variant="body2" color="text.secondary" gutterBottom>
              使用上方密钥作为口令；命令行仅需 <code>file</code> 一个字段。
            </Typography>
            <Stack spacing={2}>
              {examples.map((ex) => (
                <Box key={ex.title}>
                  <Stack direction="row" spacing={1} alignItems="center" sx={{ mb: 0.5 }}>
                    <Typography variant="body2" sx={{ flexGrow: 1 }}>
                      {ex.title}
                    </Typography>
                    <CopyButton text={ex.cmd} label="复制命令" />
                  </Stack>
                  <Paper variant="outlined" sx={{ p: 1.5, bgcolor: '#0d1117' }}>
                    <Typography
                      component="pre"
                      sx={{ m: 0, color: '#e6edf3', fontSize: 13, whiteSpace: 'pre-wrap', wordBreak: 'break-all', fontFamily: 'monospace' }}
                    >
                      {ex.cmd}
                    </Typography>
                  </Paper>
                </Box>
              ))}
            </Stack>
          </Box>
        </Stack>
      </Box>
    </Box>
  )
}

// 外观：自定义背景图片（从已上传图片中选择）
function AppearanceCard() {
  const { s, update, save } = useSettingsCtx()
  const [bgUrl, setBgUrl] = useState('')
  const [open, setOpen] = useState(false)
  const [rows, setRows] = useState<ImageItem[]>([])
  const [total, setTotal] = useState(0)
  const [page, setPage] = useState(0)
  const [size, setSize] = useState(20)
  const [loading, setLoading] = useState(false)

  if (!s) return null

  const loadImages = async (p: number, z: number) => {
    setLoading(true)
    try {
      const r = await api.get<{ items: ImageItem[]; total: number }>('/admin/images', { params: { page: p + 1, size: z } })
      setRows(r.data.items)
      setTotal(r.data.total)
    } catch {
      /* ignore */
    } finally {
      setLoading(false)
    }
  }

  const openDialog = () => {
    setPage(0)
    setOpen(true)
    void loadImages(0, size)
  }

  const pick = (item: ImageItem) => {
    update('background_image', item.objectkey)
    setBgUrl(item.url)
    setOpen(false)
  }

  const removeBg = () => {
    update('background_image', '')
    setBgUrl('')
  }

  const preview = bgUrl || s.background_url

  return (
    <Box>
      <Box>
        <Typography variant="subtitle1" gutterBottom>
          自定义背景
        </Typography>
        <Typography variant="body2" color="text.secondary" gutterBottom>
          从已上传的图片中选择一张，作为登录页与应用界面的背景。
        </Typography>
        <Box sx={{ mt: 2 }}>
          {preview ? (
            <Box component="img" src={preview} alt="当前背景" sx={{ width: '100%', maxWidth: 480, height: 160, objectFit: 'cover', borderRadius: 1 }} />
          ) : (
            <Paper variant="outlined" sx={{ width: '100%', maxWidth: 480, height: 160, display: 'flex', alignItems: 'center', justifyContent: 'center', color: 'text.secondary' }}>
              未设置背景
            </Paper>
          )}
        </Box>
        <Stack direction="row" justifyContent="flex-end" spacing={1} sx={{ mt: 2 }}>
          <Button variant="outlined" color="error" disabled={!preview} onClick={removeBg}>
            移除背景
          </Button>
          <Button variant="contained" onClick={openDialog}>
            选择图片
          </Button>
          <Button variant="outlined" onClick={() => void save()}>
            保存
          </Button>
        </Stack>
      </Box>

      <Dialog open={open} onClose={() => setOpen(false)} maxWidth="sm" fullWidth>
        <DialogTitle>选择背景图片</DialogTitle>
        <DialogContent dividers>
          <Box sx={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fill, minmax(96px, 1fr))', gap: 1 }}>
            {rows.map((r) => (
              <Box
                key={r.id}
                component="img"
                src={r.url}
                alt={r.name}
                loading="lazy"
                onClick={() => pick(r)}
                onError={(e) => ((e.target as HTMLImageElement).style.visibility = 'hidden')}
                sx={{ width: '100%', aspectRatio: '1', objectFit: 'cover', borderRadius: 1, cursor: 'pointer', '&:hover': { opacity: 0.8 } }}
              />
            ))}
          </Box>
          {rows.length === 0 && (
            <Typography variant="body2" color="text.secondary">
              {loading ? '加载中…' : '暂无图片'}
            </Typography>
          )}
        </DialogContent>
        <DialogActions>
          <TablePagination
            component="div"
            count={total}
            page={page}
            rowsPerPage={size}
            rowsPerPageOptions={[20, 50]}
            onPageChange={(_, p) => {
              setPage(p)
              void loadImages(p, size)
            }}
            onRowsPerPageChange={(e) => {
              const z = parseInt(e.target.value, 10)
              setSize(z)
              setPage(0)
              void loadImages(0, z)
            }}
          />
        </DialogActions>
      </Dialog>
    </Box>
  )
}

// 维护：R2 迁移导入 + 孤儿清理
function MigrateCard() {
  const [prefix, setPrefix] = useState('')
  const [scan, setScan] = useState<ScanResp | null>(null)
  const [run, setRun] = useState<RunResp | null>(null)
  const [err, setErr] = useState('')
  const [cleanup, setCleanup] = useState<{ deleted: number; skipped_grace: number } | null>(null)
  const [dialogOpen, setDialogOpen] = useState(false)
  const [busy, setBusy] = useState(false)

  // 扫描（dry-run）→ 弹确认框 → 确认后导入
  const doScanAndConfirm = async () => {
    setErr('')
    setRun(null)
    setBusy(true)
    try {
      const r = await api.get<ScanResp>('/admin/import/scan', { params: { prefix } })
      setScan(r.data)
      setDialogOpen(true)
    } catch (e: any) {
      setErr(e.message)
    } finally {
      setBusy(false)
    }
  }

  const doImport = async () => {
    if (!scan) return
    setDialogOpen(false)
    setErr('')
    try {
      const r = await api.post<RunResp>('/admin/import/run', { prefix })
      setRun(r.data)
    } catch (e: any) {
      setErr(e.message)
    }
  }

  const doCleanup = async () => {
    setErr('')
    if (!window.confirm('确认执行清理？将删除系统中没有记录、且不是近期上传的图片。')) return
    try {
      const r = await api.post<{ deleted: number; skipped_grace: number }>('/admin/cleanup')
      setCleanup(r.data)
    } catch (e: any) {
      setErr(e.message)
    }
  }

  return (
    <Box>
      <Box>
        <Stack spacing={3}>
          <Box>
            <Typography variant="subtitle1" gutterBottom>
              导入历史图片
            </Typography>
            <Typography variant="body2" color="text.secondary" gutterBottom>
              把存储桶中已有的图片导入到本系统（仅查看文件列表，不会改动原有数据）。
            </Typography>
            <Stack spacing={1} sx={{ mb: 2 }}>
              <TextField label="目录前缀（可选）" value={prefix} onChange={(e) => setPrefix(e.target.value)} placeholder="例如 2023/08/" fullWidth />
              <Stack direction="row" justifyContent="flex-end">
                <Button variant="contained" disabled={busy} onClick={() => void doScanAndConfirm()}>
                  {busy ? '扫描中…' : '扫描并导入'}
                </Button>
              </Stack>
            </Stack>
            {err && <Alert severity="error">{err}</Alert>}
            {run && (
              <Alert severity="success" sx={{ mt: 1 }}>
                导入完成：新增 {run.imported} · 跳过 {run.skipped} · 忽略 {run.ignored}
                {run.errors.length > 0 && ` · 错误 ${run.errors.length}`}
              </Alert>
            )}
          </Box>

          <Divider />

          <Box>
            <Typography variant="subtitle1" gutterBottom>
              清理无用图片
            </Typography>
            <Typography variant="body2" color="text.secondary" gutterBottom>
              删除存储桶中不再被系统记录的图片，释放空间。建议先完成导入再清理。
            </Typography>
            <Stack justifyContent="flex-end">
              <Button variant="contained" color="error" sx={{ alignSelf: 'flex-end' }} onClick={() => void doCleanup()}>立即清理</Button>
            </Stack>
            {cleanup && (
              <Alert severity="success" sx={{ mt: 1 }}>
                已清理 {cleanup.deleted} 张无用图片，{cleanup.skipped_grace} 张近期上传已保留。
              </Alert>
            )}
          </Box>
        </Stack>
      </Box>

      <Dialog open={dialogOpen} onClose={() => setDialogOpen(false)} maxWidth="sm" fullWidth>
        <DialogTitle>确认导入</DialogTitle>
        <DialogContent dividers>
          {scan && (
            <Stack spacing={1}>
              <Typography variant="body2">
                共 {scan.total} 个对象 · 新增 <b>{scan.new}</b> · 已存在 {scan.existing} · 忽略 {scan.ignored}
              </Typography>
              <Typography variant="body2" color="text.secondary">
                仅查看存储桶中的文件列表，不会下载或改动内容；确认后将新增 {scan.new} 条记录。
              </Typography>
              {scan.items.length > 0 && (
                <TableContainer component={Box} sx={{ maxHeight: 280, border: '1px solid', borderColor: 'divider', borderRadius: 1 }}>
                  <Table size="small" stickyHeader>
                    <TableHead>
                      <TableRow>
                        <TableCell>Key</TableCell>
                        <TableCell>Size</TableCell>
                        <TableCell>LastModified</TableCell>
                      </TableRow>
                    </TableHead>
                    <TableBody>
                      {scan.items.map((i) => (
                        <TableRow key={i.key}>
                          <TableCell sx={{ maxWidth: 280 }}>
                            <Typography noWrap variant="body2">{i.key}</Typography>
                          </TableCell>
                          <TableCell>{i.size}</TableCell>
                          <TableCell>{i.last_modified}</TableCell>
                        </TableRow>
                      ))}
                    </TableBody>
                  </Table>
                </TableContainer>
              )}
            </Stack>
          )}
        </DialogContent>
        <DialogActions>
          <Button onClick={() => setDialogOpen(false)}>取消</Button>
          <Button variant="contained" onClick={() => void doImport()}>导入 {scan?.new ?? 0} 个</Button>
        </DialogActions>
      </Dialog>
    </Box>
  )
}

export default function SettingsPage() {
  const { section } = useParams()
  const { s } = useSettingsCtx()
  if (!s) {
    return <Typography>加载中…</Typography>
  }
  return (
    <Stack spacing={3}>
      <Typography variant="h6">设置</Typography>
      <Banner />
      {section === 'storage' && <StorageCard />}
      {section === 'appearance' && <AppearanceCard />}
      {section === 'security' && <SecurityCard />}
      {section === 'api' && <ApiCard />}
      {section === 'maintenance' && <MigrateCard />}
    </Stack>
  )
}
