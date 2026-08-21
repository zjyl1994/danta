import { useCallback, useEffect, useState } from 'react'
import {
  Alert,
  Box,
  Button,
  Card,
  CardContent,
  Checkbox,
  IconButton,
  Paper,
  Stack,
  Table,
  TableBody,
  TableCell,
  TableContainer,
  TableHead,
  TablePagination,
  TableRow,
  Tooltip,
  Typography
} from '@mui/material'
import DeleteIcon from '@mui/icons-material/Delete'
import CropOriginalIcon from '@mui/icons-material/CropOriginal'
import { api } from '../api'
import type { ImageItem, StatsResp } from '../types'
import CopyButton from '../components/CopyButton'
import FormatSelect from '../components/FormatSelect'
import Lightbox from '../components/Lightbox'
import { useConfirmDialog } from '../components/useConfirmDialog'
import { copyText } from '../clipboard'
import { formatLink, Fmt, loadFormat, saveFormat } from '../format'
import { usePersistentState } from '../usePersistentState'

function fmtBytes(n: number): string {
  if (n < 1024) return `${n} B`
  if (n < 1024 * 1024) return `${(n / 1024).toFixed(1)} KB`
  return `${(n / 1024 / 1024).toFixed(2)} MB`
}

function fmtTime(s: string): string {
  const d = new Date(s)
  return isNaN(d.getTime()) ? s : d.toLocaleString()
}

export default function ManagePage() {
  const [rows, setRows] = useState<ImageItem[]>([])
  const [total, setTotal] = useState(0)
  const [page, setPage] = useState(0)
  const [size, setSize] = usePersistentState<number>('danta.list_page_size', () => 20)
  const [selected, setSelected] = useState<Set<number>>(new Set())
  const [fmt, setFmt] = useState<Fmt>(() => loadFormat())
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(false)
  const [stats, setStats] = useState<StatsResp | null>(null)
  const [lightbox, setLightbox] = useState<number | null>(null)
  const { confirm, dialog } = useConfirmDialog()

  const loadStats = useCallback(async () => {
    try {
      const r = await api.get<StatsResp>('/admin/stats')
      setStats(r.data)
    } catch {
      /* ignore */
    }
  }, [])

  const load = useCallback(async () => {
    setLoading(true)
    try {
      const r = await api.get<{ items: ImageItem[]; total: number }>('/admin/images', { params: { page: page + 1, size } })
      setRows(r.data.items)
      setTotal(r.data.total)
      setError('')
    } catch (e: any) {
      setError(e.message)
    } finally {
      setLoading(false)
    }
  }, [page, size])

  useEffect(() => {
    void load()
    void loadStats()
  }, [load, loadStats])

  const toggle = (id: number) =>
    setSelected((s) => {
      const n = new Set(s)
      if (n.has(id)) n.delete(id)
      else n.add(id)
      return n
    })

  const toggleAll = () =>
    setSelected((s) => {
      if (s.size === rows.length) return new Set()
      return new Set(rows.map((r) => r.id))
    })

  const remove = async (ids: number[]) => {
    if (!(await confirm({ title: '删除图片', description: `确认删除 ${ids.length} 张图片？此操作无法撤销。`, confirmLabel: '删除' }))) return
    try {
      await api.post('/admin/images/delete', { ids })
      setSelected((s) => {
        const n = new Set(s)
        ids.forEach((i) => n.delete(i))
        return n
      })
      void load()
    } catch (e: any) {
      setError(e.message)
    }
  }

  const selectedRows = rows.filter((r) => selected.has(r.id))
  const changeFmt = (f: Fmt) => {
    setFmt(f)
    saveFormat(f)
  }
  const copyAll = async () => {
    const text = selectedRows.map((r) => formatLink(fmt, r.url, r.name)).join('\n')
    await copyText(text)
  }

  return (
    <Stack spacing={2}>
      <Stack direction="row" spacing={1} alignItems="center" sx={{ flexWrap: 'nowrap' }}>
        <Typography variant="h6" sx={{ flexGrow: 1 }}>
          图片
        </Typography>
        <FormatSelect value={fmt} onChange={changeFmt} />
        <Button size="small" variant="outlined" sx={{ height: 30 }} disabled={selectedRows.length === 0} onClick={() => void copyAll()}>
          批量复制
        </Button>
        <Button size="small" color="error" variant="outlined" sx={{ height: 30 }} disabled={selectedRows.length === 0} onClick={() => void remove([...selected])}>
          批量删除
        </Button>
      </Stack>

      {stats && (
        <Box sx={{ display: 'grid', gridTemplateColumns: { xs: 'repeat(2, 1fr)', sm: 'repeat(4, 1fr)' }, gap: 2 }}>
          {[
            { k: 'images', l: '图片', v: String(stats.images) },
            { k: 'size', l: '已用存储', v: fmtBytes(stats.total_size) },
            { k: 'uploads', l: '近 24 小时新增', v: String(stats.uploads_24h) },
            { k: 'originals', l: '原图', v: String(stats.originals) }
          ].map((c) => (
            <Card key={c.k} variant="outlined" sx={{ minWidth: 0, height: 84, display: 'flex', flexDirection: 'column' }}>
              <CardContent sx={{ flexGrow: 1, display: 'flex', flexDirection: 'column', py: 1, '&:last-child': { pb: 1 } }}>
                <Typography variant="body2" color="text.secondary" noWrap>
                  {c.l}
                </Typography>
                <Typography variant="h5" fontWeight={700} noWrap sx={{ mt: 'auto' }}>
                  {c.v}
                </Typography>
              </CardContent>
            </Card>
          ))}
        </Box>
      )}

      {error && <Alert severity="error">{error}</Alert>}

      <TableContainer component={Paper}>
        <Table size="small">
          <TableHead>
            <TableRow>
              <TableCell padding="checkbox">
                <Checkbox checked={rows.length > 0 && selected.size === rows.length} indeterminate={selected.size > 0 && selected.size < rows.length} onChange={toggleAll} />
              </TableCell>
              <TableCell sx={{ width: 64 }}>预览</TableCell>
              <TableCell sx={{ width: '100%', minWidth: 120 }}>名称</TableCell>
              <TableCell sx={{ width: 88 }}>大小</TableCell>
              <TableCell sx={{ width: 100 }}>尺寸</TableCell>
              <TableCell sx={{ width: 100 }}>类型</TableCell>
              <TableCell sx={{ width: 170 }}>时间</TableCell>
              <TableCell sx={{ width: 100 }}>操作</TableCell>
            </TableRow>
          </TableHead>
          <TableBody>
            {rows.map((r) => (
              <TableRow key={r.id} hover>
                <TableCell padding="checkbox">
                  <Checkbox checked={selected.has(r.id)} onChange={() => toggle(r.id)} />
                </TableCell>
                <TableCell>
                  <Box
                    component="img"
                    src={r.url}
                    alt={r.name}
                    loading="lazy"
                    sx={{ width: 56, height: 56, objectFit: 'cover', borderRadius: 1, cursor: 'pointer' }}
                    onClick={() => setLightbox(rows.findIndex((x) => x.id === r.id))}
                    onError={(e) => ((e.target as HTMLImageElement).style.visibility = 'hidden')}
                  />
                </TableCell>
                <TableCell sx={{ width: '100%', minWidth: 120 }}>
                  <Stack direction="row" spacing={0.5} alignItems="center" sx={{ minWidth: 0 }}>
                    <Typography variant="body2" noWrap title={r.name} sx={{ minWidth: 0, flexGrow: 1 }}>
                      {r.name}
                    </Typography>
                    {r.original && (
                      <Tooltip title="原图">
                        <CropOriginalIcon fontSize="small" color="primary" sx={{ flexShrink: 0 }} />
                      </Tooltip>
                    )}
                  </Stack>
                </TableCell>
                <TableCell sx={{ whiteSpace: 'nowrap' }}>{fmtBytes(r.size)}</TableCell>
                <TableCell sx={{ whiteSpace: 'nowrap' }}>{r.width > 0 ? `${r.width}x${r.height}` : '--'}</TableCell>
                <TableCell sx={{ whiteSpace: 'nowrap' }}>{r.mime ? r.mime.replace(/^image\//, '') : '--'}</TableCell>
                <TableCell sx={{ whiteSpace: 'nowrap' }}>{fmtTime(r.created_at)}</TableCell>
                <TableCell sx={{ whiteSpace: 'nowrap' }}>
                  <Stack direction="row" spacing={0.5} alignItems="center">
                    <CopyButton text={formatLink(fmt, r.url, r.name)} />
                    <IconButton size="small" color="error" onClick={() => void remove([r.id])}>
                      <DeleteIcon fontSize="small" />
                    </IconButton>
                  </Stack>
                </TableCell>
              </TableRow>
            ))}
            {rows.length === 0 && (
              <TableRow>
                <TableCell colSpan={8} sx={{ textAlign: 'center', color: 'text.secondary' }}>
                  {loading ? '加载中…' : '暂无图片'}
                </TableCell>
              </TableRow>
            )}
          </TableBody>
        </Table>
      </TableContainer>

      <TablePagination
        component="div"
        count={total}
        page={page}
        rowsPerPage={size}
        rowsPerPageOptions={[10, 20, 50, 100]}
        onPageChange={(_, p) => setPage(p)}
        onRowsPerPageChange={(e) => { setSize(parseInt(e.target.value, 10)); setPage(0) }}
      />

      {lightbox !== null && (
        <Lightbox
          images={rows.map((r) => ({ url: r.url, name: r.name }))}
          index={lightbox}
          onClose={() => setLightbox(null)}
          onNavigate={setLightbox}
        />
      )}
      {dialog}
    </Stack>
  )
}
