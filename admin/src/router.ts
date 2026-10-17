import { useEffect, useState } from 'react'

const BASE = '/admin'

export function navigate(to: string) {
  history.pushState(null, '', BASE + to)
  window.dispatchEvent(new PopStateEvent('popstate'))
}

export function usePath() {
  const read = () => location.pathname.replace(/^\/admin/, '') || '/'
  const [path, setPath] = useState(read)
  useEffect(() => {
    const on = () => setPath(read())
    window.addEventListener('popstate', on)
    return () => window.removeEventListener('popstate', on)
  }, [])
  return path
}
