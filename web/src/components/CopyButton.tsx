import { useState } from 'react'
import IconButton from '@mui/material/IconButton'
import Tooltip from '@mui/material/Tooltip'
import Snackbar from '@mui/material/Snackbar'
import ContentCopyIcon from '@mui/icons-material/ContentCopy'
import { copyText } from '../clipboard'

export default function CopyButton({ text, label }: { text: string; label?: string }) {
  const [open, setOpen] = useState(false)

  const copy = async () => {
    if (await copyText(text)) setOpen(true)
  }

  return (
    <>
      <Tooltip title={label ?? '复制'}>
        <IconButton size="small" onClick={copy}>
          <ContentCopyIcon fontSize="small" />
        </IconButton>
      </Tooltip>
      <Snackbar open={open} autoHideDuration={1200} onClose={() => setOpen(false)} message="已复制" />
    </>
  )
}
