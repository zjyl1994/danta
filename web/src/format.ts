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
