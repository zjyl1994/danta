import Select from '@mui/material/Select'
import MenuItem from '@mui/material/MenuItem'
import { FORMATS, Fmt } from '../format'

// 外链格式下拉选择器（替代切换按钮组）
export default function FormatSelect({ value, onChange }: { value: Fmt; onChange: (f: Fmt) => void }) {
  return (
    <Select size="small" value={value} onChange={(e) => onChange(e.target.value as Fmt)} sx={{ height: 30, minWidth: { xs: 112, sm: 140 } }}>
      {FORMATS.map((f) => (
        <MenuItem key={f.k} value={f.k}>
          {f.l}
        </MenuItem>
      ))}
    </Select>
  )
}
