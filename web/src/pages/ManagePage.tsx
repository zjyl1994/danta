import { useCallback, useEffect, useState } from 'react'
import {
  Alert,
  Box,
  Button,
  Card,
  CardContent,
  Checkbox,
  Chip,
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
  Typography
} from '@mui/material'
import DeleteIcon from '@mui/icons-material/Delete'
import { api } from '../api'
import type { ImageItem, StatsResp } from '../types'
import CopyButton from '../components/CopyButton'
import FormatTabs from '../components/FormatTabs'
import Lightbox from '../components/Lightbox'
import { formatLink, Fmt } from '../format'

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
  const [size, setSize] = useState(20)
  const [selected, setSelected] = useState<Set<number>>(new Set())
  const [fmt, setFmt] = useState<Fmt>('url')
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(false)
  const [stats, setStats] = useState<StatsResp | null>(null)
  const [lightbox, setLightbox] = useState<number | null>(null)

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
    if (!window.confirm(`确认删除 ${ids.length} 张图片？`)) return
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
  const copyAll = async () => {
    const text = selectedRows.map((r) => formatLink(fmt, r.url, r.name)).join('\n')
    try {
      await navigator.clipboard.writeText(text)
    } catch { /* ignore */ }
  }

  return (
    <Stack spacing={2}>
      <Stack direction="row" spacing={1} alignItems="center" flexWrap="wrap" gap={1}>
        <Typography variant="h6" sx={{ flexGrow: 1 }}>
          图片列表
        </Typography>
        <FormatTabs value={fmt} onChange={setFmt} />
        <Button size="small" variant="outlined" disabled={selectedRows.length === 0} onClick={() => void copyAll()}>
          批量复制
        </Button>
        <Button size="small" color="error" variant="outlined" disabled={selectedRows.length === 0} onClick={() => void remove([...selected])}>
          批量删除
        </Button>
      </Stack>

      {stats && (
        <Stack direction="row" spacing={2} sx={{ flexWrap: 'wrap' }}>
          {[
            { l: '图片总数', v: String(stats.images) },
            { l: '总存储', v: fmtBytes(stats.total_size) },
            { l: '近 24h 新增', v: String(stats.uploads_24h) },
            { l: '原图直存', v: String(stats.originals) }
          ].map((c) => (
            <Card variant="outlined" sx={{ flex: '1 1 160px', minWidth: 140 }}>
              <CardContent sx={{ py: 1.5 }}>
                <Typography variant="body2" color="text.secondary">
                  {c.l}
                </Typography>
                <Typography variant="h6">{c.v}</Typography>
              </CardContent>
            </Card>
          ))}
        </Stack>
      )}

      {error && <Alert severity="error">{error}</Alert>}

      <TableContainer component={Paper}>
        <Table size="small">
          <TableHead>
            <TableRow>
              <TableCell padding="checkbox">
                <Checkbox checked={rows.length > 0 && selected.size === rows.length} indeterminate={selected.size > 0 && selected.size < rows.length} onChange={toggleAll} />
              </TableCell>
              <TableCell>预览</TableCell>
              <TableCell>名称</TableCell>
              <TableCell>模式</TableCell>
              <TableCell>大小</TableCell>
              <TableCell>尺寸</TableCell>
              <TableCell>MIME</TableCell>
              <TableCell>时间</TableCell>
              <TableCell>外链</TableCell>
              <TableCell />
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
                <TableCell sx={{ maxWidth: 160 }}>
                  <Typography variant="body2" noWrap title={r.name}>
                    {r.name}
                  </Typography>
                </TableCell>
                <TableCell>
                  <Chip size="small" color={r.original ? 'default' : 'primary'} label={r.original ? '原图' : 'WebP'} />
                </TableCell>
                <TableCell>{fmtBytes(r.size)}</TableCell>
                <TableCell>{r.width > 0 ? `${r.width}x${r.height}` : '--'}</TableCell>
                <TableCell>{r.mime || '--'}</TableCell>
                <TableCell>{fmtTime(r.created_at)}</TableCell>
                <TableCell>
                  <CopyButton text={formatLink(fmt, r.url, r.name)} />
                </TableCell>
                <TableCell>
                  <IconButton size="small" color="error" onClick={() => void remove([r.id])}>
                    <DeleteIcon fontSize="small" />
                  </IconButton>
                </TableCell>
              </TableRow>
            ))}
            {rows.length === 0 && (
              <TableRow>
                <TableCell colSpan={10} sx={{ textAlign: 'center', color: 'text.secondary' }}>
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
    </Stack>
  )
}
