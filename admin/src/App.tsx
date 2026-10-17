import { useCallback, useEffect, useState } from 'react'

import { api, type Admin } from './api'
import { Login } from './pages/Login'
import { Shell } from './Shell'

function App() {
  const [me, setMe] = useState<Admin | null | undefined>(undefined)

  const load = useCallback(() => {
    api<Admin>('GET', '/me')
      .then(setMe)
      .catch(() => setMe(null))
  }, [])

  useEffect(() => {
    load()
    const out = () => setMe(null)
    window.addEventListener('admin:logout', out)
    return () => window.removeEventListener('admin:logout', out)
  }, [load])

  if (me === undefined) return null
  if (me === null) return <Login onDone={setMe} />
  return <Shell me={me} onLogout={() => api('POST', '/logout', {}).finally(() => setMe(null))} />
}

export default App
