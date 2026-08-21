import { useEffect, useState } from 'react'

type Setter<T> = React.Dispatch<React.SetStateAction<T>>

// 持久化到 localStorage 的 useState；init 仅首次求值（含恢复逻辑）
export function usePersistentState<T>(key: string, init: () => T): [T, Setter<T>] {
  const [v, setV] = useState<T>(() => {
    const raw = localStorage.getItem(key)
    if (raw !== null) {
      try {
        return JSON.parse(raw) as T
      } catch {
        /* ignore corrupt value */
      }
    }
    return init()
  })

  useEffect(() => {
    localStorage.setItem(key, JSON.stringify(v))
  }, [key, v])

  return [v, setV]
}
