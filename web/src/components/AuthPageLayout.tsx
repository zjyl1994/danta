import { useContext } from 'react'
import type { ReactNode } from 'react'
import { Box, Card, CardContent, Typography } from '@mui/material'
import { AppCtx } from '../App'

export default function AuthPageLayout({ title, children }: { title: string; children: ReactNode }) {
  const { cfg } = useContext(AppCtx)
  const backgroundUrl = cfg?.background_url ?? ''
  return (
    <Box
      sx={{
        display: 'flex',
        minHeight: '100vh',
        alignItems: 'center',
        justifyContent: 'center',
        p: 2,
        position: 'relative',
        overflow: 'hidden'
      }}
    >
      {backgroundUrl && (
        <>
          <Box
            sx={{
              position: 'absolute',
              inset: 0,
              backgroundImage: `url(${backgroundUrl})`,
              backgroundSize: 'cover',
              backgroundPosition: 'center'
            }}
          />
          <Box sx={{ position: 'absolute', inset: 0, bgcolor: 'rgba(0,0,0,0.35)' }} />
        </>
      )}
      <Card sx={{ width: '100%', maxWidth: 420, position: 'relative', zIndex: 1 }}>
        <CardContent>
          <Typography variant="h5" gutterBottom>
            {title}
          </Typography>
          {children}
        </CardContent>
      </Card>
    </Box>
  )
}
