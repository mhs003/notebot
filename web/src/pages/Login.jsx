import { useState } from 'react'
import { Layers, Loader2 } from 'lucide-react'
import { Button } from '../components/ui/button'
import { Input } from '../components/ui/input'
import { api } from '../lib/api'

export function Login({ onDone }) {
  const [pw, setPw] = useState('')
  const [err, setErr] = useState('')
  const [busy, setBusy] = useState(false)

  const submit = async (e) => {
    e.preventDefault()
    setBusy(true)
    setErr('')
    try {
      await api.login(pw)
      onDone()
    } catch (e) {
      setErr(e.message === 'unauthorized' ? 'Incorrect password' : e.message)
    } finally {
      setBusy(false)
    }
  }

  return (
    <div className="flex min-h-screen items-center justify-center bg-gradient-to-br from-primary/10 via-background to-background p-4">
      <div className="w-full max-w-sm rounded-xl border bg-card p-8 shadow-lg">
        <div className="mb-6 flex flex-col items-center gap-3 text-center">
          <div className="flex h-12 w-12 items-center justify-center rounded-xl bg-primary text-primary-foreground">
            <Layers className="h-6 w-6" />
          </div>
          <div>
            <h1 className="text-xl font-semibold tracking-tight">Notebot</h1>
            <p className="text-sm text-muted-foreground">Sign in to your notes</p>
          </div>
        </div>
        <form onSubmit={submit} className="space-y-3">
          <Input
            type="password"
            placeholder="Password"
            value={pw}
            onChange={(e) => setPw(e.target.value)}
            autoFocus
          />
          {err && <p className="text-sm text-destructive">{err}</p>}
          <Button type="submit" className="w-full" disabled={busy}>
            {busy ? <Loader2 className="h-4 w-4 animate-spin" /> : 'Sign in'}
          </Button>
        </form>
      </div>
    </div>
  )
}
