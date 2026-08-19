import { useEffect } from 'react'
import Box from '@mui/material/Box'
import IconButton from '@mui/material/IconButton'
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
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') onClose()
      else if (e.key === 'ArrowLeft') onNavigate((index - 1 + images.length) % images.length)
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
    <Box
      onClick={onClose}
      sx={{
        position: 'fixed',
        inset: 0,
        zIndex: 1400,
        bgcolor: 'rgba(0,0,0,0.92)',
        display: 'flex',
        flexDirection: 'column'
      }}
    >
      <Box onClick={(e) => e.stopPropagation()} sx={{ display: 'flex', flexDirection: 'column', height: '100%' }}>
        <Box sx={{ display: 'flex', alignItems: 'center', p: 1, color: '#fff' }}>
          <Typography variant="body2" sx={{ flexGrow: 1 }} noWrap>
            {img.name}（{index + 1}/{images.length}）
          </Typography>
          <IconButton color="inherit" onClick={onClose}>
            <CloseIcon />
          </IconButton>
        </Box>
        <Box sx={{ flexGrow: 1, display: 'flex', alignItems: 'center', justifyContent: 'center', minHeight: 0, px: 1 }}>
          <IconButton color="inherit" size="large" onClick={() => onNavigate(prev)}>
            <ChevronLeftIcon fontSize="large" />
          </IconButton>
          <Box
            component="img"
            src={img.url}
            alt={img.name}
            sx={{ maxWidth: 'calc(100% - 140px)', maxHeight: '100%', objectFit: 'contain', bgcolor: '#111', borderRadius: 1 }}
          />
          <IconButton color="inherit" size="large" onClick={() => onNavigate(next)}>
            <ChevronRightIcon fontSize="large" />
          </IconButton>
        </Box>
      </Box>
    </Box>
  )
}
