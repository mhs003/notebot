import { useEffect, useState } from 'react'
import { Save, LogOut } from 'lucide-react'
import { Button } from '../components/ui/button'
import { Input } from '../components/ui/input'
import { Card } from '../components/ui/card'
import { api } from '../lib/api'

export function Settings({ onLogout }) {
  const [s, setS] = useState(null)
  const [modelPath, setModelPath] = useState('')
  const [port, setPort] = useState('')
  const [msg, setMsg] = useState('')

  useEffect(() => {
    api.settings().then((d) => { setS(d); setModelPath(d.model_path || ''); setPort(String(d.port || '')) })
  }, [])

  if (!s) return <div className="p-8 text-muted-foreground">Loading…</div>

  const save = async () => {
    const res = await api.saveSettings({ model_path: modelPath, port: Number(port) || undefined })
    setMsg(res?.restart_required ? 'Saved — restart notebotd to apply the model path.' : 'Saved.')
  }

  return (
    <div className="mx-auto max-w-2xl p-8">
      <h1 className="mb-6 text-2xl font-semibold tracking-tight">Settings</h1>
      <Card className="space-y-5 p-6">
        <div>
          <label className="mb-1.5 block text-sm font-medium">Data directory</label>
          <Input value={s.data_dir} readOnly className="bg-muted" />
        </div>
        <div>
          <label className="mb-1.5 block text-sm font-medium">Model path</label>
          <Input value={modelPath} onChange={(e) => setModelPath(e.target.value)} />
          <p className="mt-1 text-xs text-muted-foreground">
            Path to needle3.cact. Changing this requires a daemon restart.
          </p>
        </div>
        <div>
          <label className="mb-1.5 block text-sm font-medium">Port</label>
          <Input value={port} onChange={(e) => setPort(e.target.value)} className="max-w-[160px]" />
        </div>
        <div className="flex items-center gap-3">
          <Button onClick={save}><Save className="h-4 w-4" /> Save</Button>
          <Button variant="outline" onClick={onLogout}><LogOut className="h-4 w-4" /> Sign out</Button>
          {msg && <span className="text-sm text-muted-foreground">{msg}</span>}
        </div>
      </Card>
    </div>
  )
}
