export type Fmt = 'url' | 'md' | 'mdlink' | 'bbcode' | 'html'

export const FORMATS: { k: Fmt; l: string }[] = [
  { k: 'url', l: 'URL' },
  { k: 'md', l: 'Markdown' },
  { k: 'mdlink', l: 'MD+链接' },
  { k: 'bbcode', l: 'BBCode' },
  { k: 'html', l: 'HTML' }
]

export function formatLink(f: Fmt, url: string, name: string): string {
  switch (f) {
    case 'url':
      return url
    case 'md':
      return `![${name}](${url})`
    case 'mdlink':
      return `[![${name}](${url})](${url})`
    case 'bbcode':
      return `[img]${url}[/img]`
    case 'html':
      return `<img src="${url}" alt="${name}">`
  }
}

const FORMAT_KEY = 'danta.link_format'

// 从 localStorage 恢复上次选择的外链格式
export function loadFormat(): Fmt {
  const v = localStorage.getItem(FORMAT_KEY)
  return (v && FORMATS.some((f) => f.k === v) ? v : 'url') as Fmt
}

export function saveFormat(f: Fmt) {
  localStorage.setItem(FORMAT_KEY, f)
}
