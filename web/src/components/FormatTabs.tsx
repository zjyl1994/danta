import ToggleButtonGroup from '@mui/material/ToggleButtonGroup'
import ToggleButton from '@mui/material/ToggleButton'
import { FORMATS, Fmt } from '../format'

export default function FormatTabs({ value, onChange }: { value: Fmt; onChange: (f: Fmt) => void }) {
  return (
    <ToggleButtonGroup
      size="small"
      value={value}
      exclusive
      onChange={(_, v) => v && onChange(v)}
      sx={{ '& .MuiToggleButton-root': { height: 30 } }}
    >
      {FORMATS.map((f) => (
        <ToggleButton key={f.k} value={f.k}>
          {f.l}
        </ToggleButton>
      ))}
    </ToggleButtonGroup>
  )
}
