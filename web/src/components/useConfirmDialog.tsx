import { useState } from 'react'
import type { ButtonProps } from '@mui/material/Button'
import Button from '@mui/material/Button'
import Dialog from '@mui/material/Dialog'
import DialogActions from '@mui/material/DialogActions'
import DialogContent from '@mui/material/DialogContent'
import DialogContentText from '@mui/material/DialogContentText'
import DialogTitle from '@mui/material/DialogTitle'

interface ConfirmOptions {
  title: string
  description: string
  confirmLabel?: string
  confirmColor?: ButtonProps['color']
}

interface ConfirmRequest {
  options: ConfirmOptions
  resolve: (confirmed: boolean) => void
}

export function useConfirmDialog() {
  const [request, setRequest] = useState<ConfirmRequest | null>(null)

  const close = (confirmed: boolean) => {
    request?.resolve(confirmed)
    setRequest(null)
  }

  const confirm = (options: ConfirmOptions) =>
    new Promise<boolean>((resolve) => {
      if (request) {
        resolve(false)
        return
      }
      setRequest({ options, resolve })
    })

  const dialog = (
    <Dialog
      open={request !== null}
      onClose={() => close(false)}
      aria-labelledby="confirm-dialog-title"
      aria-describedby="confirm-dialog-description"
      maxWidth="xs"
      fullWidth
    >
      <DialogTitle id="confirm-dialog-title">{request?.options.title}</DialogTitle>
      <DialogContent>
        <DialogContentText id="confirm-dialog-description">{request?.options.description}</DialogContentText>
      </DialogContent>
      <DialogActions>
        <Button onClick={() => close(false)} autoFocus>取消</Button>
        <Button
          variant="contained"
          color={request?.options.confirmColor ?? 'error'}
          onClick={() => close(true)}
        >
          {request?.options.confirmLabel ?? '确认'}
        </Button>
      </DialogActions>
    </Dialog>
  )

  return { confirm, dialog }
}
