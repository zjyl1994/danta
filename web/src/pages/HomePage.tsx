import { useEffect, useRef, useState } from 'react'
import {
  Box,
  Button,
  Card,
  CardContent,
  Chip,
  FormControlLabel,
  IconButton,
  LinearProgress,
  List,
  ListItem,
  ListItemText,
  Paper,
  Stack,
  Switch,
  Typography
} from '@mui/material'
import DeleteIcon from '@mui/icons-material/Delete'
import UploadFileIcon from '@mui/icons-material/UploadFile'
import { api, ApiError } from '../api'
import type { StatusResp, UploadResp } from '../types'
import CopyButton from '../components/CopyButton'
import { FORMATS, formatLink } from '../format'
import { usePersistentState } from '../usePersistentState'

interface Item {
  id: number
  file: File
  preview: string
  width: number
  height: number
  status: 'idle' | 'uploading' | 'error'
  progress: number
  error?: string
}

interface Result {
  id: number
  name: string
  url: string
  original: boolean
  width: number
  height: number
  size: number
}

async function fileDims(file: File): Promise<{ w: number; h: number } | null> {
  try {
    if ('createImageBitmap' in window) {
      const bmp = await createImageBitmap(file)
      const d = { w: bmp.width, h: bmp.height }
      if (typeof bmp.close === 'function') bmp.close()
      return d
    }
  } catch { /* fallback to <img> */ }
  return await new Promise((resolve) => {
    const img = new Image()
    img.onload = () => resolve({ w: img.naturalWidth, h: img.naturalHeight })
    img.onerror = () => resolve(null)
    img.src = URL.createObjectURL(file)
  })
}

async function compressImage(file: File, maxDim: number, quality: number): Promise<File | null> {
  const ext = (file.name.toLowerCase().split('.').pop() ?? '')
  if (ext === 'gif' || ext === 'avif') return null
  if (!('createImageBitmap' in window)) return null
  try {
    const bmp = await createImageBitmap(file, { imageOrientation: 'from-image' })
    const k = Math.max(bmp.width, bmp.height) > maxDim ? maxDim / Math.max(bmp.width, bmp.height) : 1
    const canvas = document.createElement('canvas')
    canvas.width = Math.max(1, Math.round(bmp.width * k))
    canvas.height = Math.max(1, Math.round(bmp.height * k))
    canvas.getContext('2d')!.drawImage(bmp, 0, 0, canvas.width, canvas.height)
    if (typeof bmp.close === 'function') bmp.close()
    const blob = await new Promise<Blob | null>((res) => canvas.toBlob(res, 'image/webp', quality / 100))
    if (!blob) return null
    const base = file.name.replace(/\.[^.]+$/, '')
    return new File([blob], `${base}.webp`, { type: 'image/webp' })
  } catch {
    return null
  }
}

export default function HomePage({ cfg }: { cfg: StatusResp }) {
  const [items, setItems] = useState<Item[]>([])
  const [results, setResults] = useState<Result[]>([])
  const [original, setOriginal] = usePersistentState<boolean>('danta.upload_original', () => false)
  const [dragging, setDragging] = useState(false)
  const [globalDrag, setGlobalDrag] = useState(false)
  const [busy, setBusy] = useState(false)
  const inputRef = useRef<HTMLInputElement>(null)

  useEffect(() => {
    return () => items.forEach((i) => URL.revokeObjectURL(i.preview))
  }, [items])

  const addFiles = (files: FileList | File[]) => {
    const list = Array.from(files).filter((f) => f.type.startsWith('image/') || /\.(jpg|jpeg|png|gif|webp|bmp|avif)$/i.test(f.name))
    const now = Date.now()
    const newItems: Item[] = list.map((f, idx) => ({
      id: now + idx,
      file: f,
      preview: URL.createObjectURL(f),
      width: 0,
      height: 0,
      status: 'idle',
      progress: 0
    }))
    setItems((arr) => [...arr, ...newItems])
    newItems.forEach((it) => {
      void fileDims(it.file).then((d) => {
        if (d) setItems((arr) => arr.map((x) => (x.id === it.id ? { ...x, width: d.w, height: d.h } : x)))
      })
    })
  }

  const addFilesRef = useRef<(files: FileList | File[]) => void>(() => {})
  addFilesRef.current = addFiles

  // 全屏粘贴 / 拖拽导入（document 级，任意位置）
  useEffect(() => {
    let depth = 0
    const hasFiles = (e: DragEvent) => e.dataTransfer?.types?.includes('Files')

    const onDragEnter = (e: DragEvent) => {
      if (!hasFiles(e)) return
      e.preventDefault()
      depth++
      setGlobalDrag(true)
    }
    const onDragOver = (e: DragEvent) => {
      if (!hasFiles(e)) return
      e.preventDefault()
    }
    const onDragLeave = (e: DragEvent) => {
      if (!hasFiles(e)) return
      depth = Math.max(0, depth - 1)
      if (depth === 0) setGlobalDrag(false)
    }
    const onDrop = (e: DragEvent) => {
      if (!hasFiles(e)) return
      e.preventDefault()
      depth = 0
      setGlobalDrag(false)
      if (e.dataTransfer?.files?.length) addFilesRef.current(e.dataTransfer.files)
    }
    const onPaste = (e: ClipboardEvent) => {
      const files = e.clipboardData?.files
      if (files && files.length) {
        e.preventDefault()
        addFilesRef.current(files)
      }
    }
    document.addEventListener('dragenter', onDragEnter)
    document.addEventListener('dragover', onDragOver)
    document.addEventListener('dragleave', onDragLeave)
    document.addEventListener('drop', onDrop)
    document.addEventListener('paste', onPaste)
    return () => {
      document.removeEventListener('dragenter', onDragEnter)
      document.removeEventListener('dragover', onDragOver)
      document.removeEventListener('dragleave', onDragLeave)
      document.removeEventListener('drop', onDrop)
      document.removeEventListener('paste', onPaste)
    }
  }, [])

  const removeItem = (id: number) => setItems((arr) => arr.filter((i) => i.id !== id))

  const uploadOne = async (item: Item, originalFlag: boolean) => {
    if (item.file.size > cfg.max_upload_bytes) {
      setItems((arr) => arr.map((i) => (i.id === item.id ? { ...i, status: 'error', error: '图片超过大小限制' } : i)))
      return
    }
    setItems((arr) => arr.map((i) => (i.id === item.id ? { ...i, status: 'uploading', progress: 0, error: undefined } : i)))
    let payload = item.file
    if (!originalFlag) {
      const c = await compressImage(item.file, cfg.resize_max_dim, cfg.webp_quality)
      if (c) payload = c
    }
    const fd = new FormData()
    fd.append('file', payload)
    if (originalFlag) fd.append('original', 'true')
    try {
      const r = await api.post<UploadResp>('/upload', fd, {
        onUploadProgress: (e) => {
          const { loaded, total } = e
          if (total) setItems((arr) => arr.map((i) => (i.id === item.id ? { ...i, progress: loaded / total } : i)))
        }
      })
      // 上传成功：移除待上传项，加入结果区
      setItems((arr) => arr.filter((i) => i.id !== item.id))
      setResults((arr) => [
        {
          id: item.id,
          name: item.file.name,
          url: r.data.url,
          original: r.data.original,
          width: r.data.width,
          height: r.data.height,
          size: r.data.size
        },
        ...arr
      ])
    } catch (err) {
      setItems((arr) =>
        arr.map((i) => (i.id === item.id ? { ...i, status: 'error', error: err instanceof ApiError ? err.message : '上传失败' } : i))
      )
    }
  }

  const uploadAll = async () => {
    if (busy) return
    setBusy(true)
    const pending = [...items]
    let idx = 0
    const workers = Array.from({ length: Math.min(3, pending.length) }, async () => {
      while (idx < pending.length) {
        const item = pending[idx++]
        await uploadOne(item, original)
      }
    })
    await Promise.all(workers)
    setBusy(false)
  }

  return (
    <Stack spacing={2}>
      <Stack direction="row" spacing={2} alignItems="center">
        <Typography variant="h6" sx={{ flexGrow: 1 }}>
          上传
        </Typography>
        <FormControlLabel
          control={<Switch checked={original} onChange={(e) => setOriginal(e.target.checked)} />}
          label="保留原图画质"
          sx={{ m: 0 }}
        />
      </Stack>

      <Paper
        elevation={0}
        sx={{
          p: 3,
          textAlign: 'center',
          cursor: 'pointer',
          bgcolor: dragging ? 'primary.light' : 'inherit',
          border: '2px dashed',
          borderColor: dragging ? 'primary.main' : 'divider'
        }}
        onDragOver={(e) => { e.preventDefault(); setDragging(true) }}
        onDragLeave={() => setDragging(false)}
        onDrop={(e) => { e.preventDefault(); setDragging(false); addFiles(e.dataTransfer.files) }}
        onClick={() => inputRef.current?.click()}
      >
        <input
          ref={inputRef}
          type="file"
          accept="image/*"
          multiple
          hidden
          onChange={(e) => { if (e.target.files) addFiles(e.target.files); e.target.value = '' }}
        />
        <UploadFileIcon sx={{ fontSize: 64, color: 'text.secondary' }} />
        <Typography variant="h6">拖拽图片到这里，点击选择，或直接 Ctrl+V 粘贴</Typography>
        <Typography variant="body2" color="text.secondary">
          单张最大 {Math.round(cfg.max_upload_bytes / 1024 / 1024)}MB
          {!original && ' · 上传时自动压缩，减小图片体积'}
        </Typography>
      </Paper>

      {items.length > 0 && (
        <Card>
          <CardContent>
            <Stack direction="row" spacing={1} alignItems="center" sx={{ mb: 1 }}>
              <Typography variant="subtitle1" sx={{ flexGrow: 1 }}>
                待上传 {items.length} 张
              </Typography>
              <Button variant="contained" disabled={busy} onClick={uploadAll}>
                {busy ? '上传中…' : '上传'}
              </Button>
              <Button variant="outlined" onClick={() => setItems([])}>清空</Button>
            </Stack>
            <List sx={{ '& .MuiListItem-root': { alignItems: 'center', gap: 2 } }}>
              {items.map((it) => (
                <ListItem key={it.id} divider>
                  <Box
                    component="img"
                    src={it.preview}
                    sx={{ width: 48, height: 48, objectFit: 'cover', borderRadius: 1, flexShrink: 0 }}
                  />
                  <ListItemText
                    primary={it.file.name}
                    secondary={
                      <Stack spacing={0.5} sx={{ mt: 0.5 }}>
                        <Typography component="span" variant="caption" color="text.secondary">
                          {(it.file.size / 1024).toFixed(1)} KB
                          {it.width > 0 && ` · ${it.width} × ${it.height} · ${((it.width * it.height) / 1e6).toFixed(2)} MP`}
                        </Typography>
                        {it.status === 'uploading' && <LinearProgress variant="determinate" value={it.progress * 100} />}
                        {it.status === 'error' && (
                          <Typography component="span" variant="caption" color="error">
                            {it.error}
                          </Typography>
                        )}
                      </Stack>
                    }
                  />
                  <IconButton edge="end" onClick={() => removeItem(it.id)}>
                    <DeleteIcon />
                  </IconButton>
                </ListItem>
              ))}
            </List>
          </CardContent>
        </Card>
      )}

      {results.length > 0 && (
        <Card>
          <CardContent>
            <Stack direction="row" spacing={1} alignItems="center" sx={{ mb: 1 }}>
              <Typography variant="subtitle1" sx={{ flexGrow: 1 }}>
                上传结果 {results.length} 张
              </Typography>
              <Button variant="outlined" onClick={() => setResults([])}>清空</Button>
            </Stack>
            <Stack spacing={1.5}>
              {results.map((r) => (
                <Paper variant="outlined" key={r.id} sx={{ p: 1.5, display: 'flex', gap: 2 }}>
                  <Box
                    component="img"
                    src={r.url}
                    sx={{ width: 96, height: 96, objectFit: 'contain', borderRadius: 1, flexShrink: 0 }}
                    loading="lazy"
                  />
                  <Box sx={{ flexGrow: 1, minWidth: 0 }}>
                    <Stack direction="row" spacing={1} alignItems="center">
                      <Chip size="small" color={r.original ? 'default' : 'primary'} label={r.original ? '原图' : '已压缩'} />
                      <Typography variant="body2" noWrap sx={{ flexGrow: 1 }}>
                        {r.name}
                      </Typography>
                      <Typography variant="caption" color="text.secondary">
                        {r.width > 0 ? `${r.width} × ${r.height}` : '--'} · {(r.size / 1024).toFixed(1)} KB
                      </Typography>
                    </Stack>
                    <Stack spacing={0.5} sx={{ mt: 1 }}>
                      {FORMATS.map((f) => (
                        <Stack key={f.k} direction="row" spacing={1} alignItems="center" sx={{ minWidth: 0 }}>
                          <Typography variant="caption" color="text.secondary" sx={{ width: 92, flexShrink: 0 }}>
                            {f.l}
                          </Typography>
                          <Typography variant="body2" noWrap sx={{ flexGrow: 1 }}>
                            {formatLink(f.k, r.url, r.name)}
                          </Typography>
                          <CopyButton text={formatLink(f.k, r.url, r.name)} />
                        </Stack>
                      ))}
                    </Stack>
                  </Box>
                </Paper>
              ))}
            </Stack>
          </CardContent>
        </Card>
      )}

      {globalDrag && (
        <Box
          sx={{
            position: 'fixed',
            inset: 0,
            zIndex: 1300,
            bgcolor: 'rgba(0,0,0,0.25)',
            pointerEvents: 'none',
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center',
            border: '3px dashed',
            borderColor: 'primary.main',
            boxSizing: 'border-box'
          }}
        >
          <Typography variant="h5" sx={{ color: '#fff', bgcolor: 'rgba(0,0,0,0.55)', px: 3, py: 1, borderRadius: 2 }}>
            松开以导入图片
          </Typography>
        </Box>
      )}
    </Stack>
  )
}
