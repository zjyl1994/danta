import { useEffect } from 'react'
import Box from '@mui/material/Box'
import Dialog from '@mui/material/Dialog'
import IconButton from '@mui/material/IconButton'
import Tooltip from '@mui/material/Tooltip'
import Typography from '@mui/material/Typography'
import CloseIcon from '@mui/icons-material/Close'
import ChevronLeftIcon from '@mui/icons-material/ChevronLeft'
import ChevronRightIcon from '@mui/icons-material/ChevronRight'

interface LightboxProps {
  images: { url: string; name: string }[]
  index: number
  onClose: () => void
  onNavigate: (i: number) => void
}

export default function Lightbox({ images, index, onClose, onNavigate }: LightboxProps) {
  useEffect(() => {
    if (images.length < 2) return
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'ArrowLeft') onNavigate((index - 1 + images.length) % images.length)
      else if (e.key === 'ArrowRight') onNavigate((index + 1) % images.length)
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [index, images.length, onClose, onNavigate])

  const img = images[index]
  if (!img) return null

  const prev = (index - 1 + images.length) % images.length
  const next = (index + 1) % images.length

  return (
    <Dialog
      fullScreen
      open
      onClose={onClose}
      aria-labelledby="lightbox-title"
      PaperProps={{ sx: { bgcolor: '#111', color: '#fff' } }}
    >
      <Box sx={{ display: 'flex', flexDirection: 'column', height: '100%' }}>
        <Box sx={{ display: 'flex', alignItems: 'center', p: 1 }}>
          <Typography id="lightbox-title" variant="body2" sx={{ flexGrow: 1 }} noWrap>
            {img.name} ({index + 1}/{images.length})
          </Typography>
          <Tooltip title="关闭预览">
            <IconButton aria-label="关闭预览" color="inherit" onClick={onClose}>
              <CloseIcon />
            </IconButton>
          </Tooltip>
        </Box>
        <Box sx={{ flexGrow: 1, display: 'flex', alignItems: 'center', justifyContent: 'center', minHeight: 0, px: 1 }}>
          <Tooltip title="上一张">
            <span>
              <IconButton aria-label="上一张" color="inherit" size="large" disabled={images.length < 2} onClick={() => onNavigate(prev)}>
                <ChevronLeftIcon fontSize="large" />
              </IconButton>
            </span>
          </Tooltip>
          <Box
            component="img"
            src={img.url}
            alt={img.name}
            sx={{ maxWidth: 'calc(100% - 140px)', maxHeight: '100%', objectFit: 'contain', bgcolor: '#111', borderRadius: 1 }}
          />
          <Tooltip title="下一张">
            <span>
              <IconButton aria-label="下一张" color="inherit" size="large" disabled={images.length < 2} onClick={() => onNavigate(next)}>
                <ChevronRightIcon fontSize="large" />
              </IconButton>
            </span>
          </Tooltip>
        </Box>
      </Box>
    </Dialog>
  )
}
