import { useState } from 'react'
import { Bot, Send, AlertTriangle, Check, Loader2 } from 'lucide-react'
import { Sheet, SheetHeader } from './ui/dialog'
import { Button } from './ui/button'
import { Input } from './ui/input'
import { api } from '../lib/api'

const examples = [
  'create a note about the Nilkhet book trip',
  'move the poem note to poems',
  'delete the deadline note',
  'create a folder called recipes',
]

export function AgentSheet({ open, onClose, onChanged }) {
  const [prompt, setPrompt] = useState('')
  const [busy, setBusy] = useState(false)
  const [log, setLog] = useState([])
  const [pending, setPending] = useState(null)
  const [error, setError] = useState('')

  const run = async (text, confirmId) => {
    setBusy(true)
    setError('')
    try {
      const res = await api.agent(confirmId ? prompt : text, confirmId)
      setLog((l) => [...l, { kind: 'result', text: res.summary, calls: res.calls }])
      if (res.needs_confirm) setPending(res)
      else { setPending(null); onChanged?.() }
    } catch (e) {
      setError(e.message)
    } finally {
      setBusy(false)
    }
  }

  const submit = (e) => {
    e.preventDefault()
    if (!prompt.trim()) return
    setLog((l) => [...l, { kind: 'user', text: prompt }])
    run(prompt)
  }

  return (
    <Sheet open={open} onClose={onClose}>
      <SheetHeader onClose={onClose}>
        <div className="flex items-center gap-2">
          <div className="flex h-8 w-8 items-center justify-center rounded-lg bg-primary text-primary-foreground">
            <Bot className="h-4 w-4" />
          </div>
          <div>
            <div className="font-semibold leading-none">Agent</div>
            <div className="text-xs text-muted-foreground">Needle3 · tool calling</div>
          </div>
        </div>
      </SheetHeader>

      <div className="flex-1 space-y-3 overflow-y-auto px-6 py-4">
        {log.length === 0 && (
          <div className="space-y-3">
            <p className="text-sm text-muted-foreground">
              Ask the agent to manage notes and folders. Destructive actions ask for confirmation.
            </p>
            <div className="space-y-1.5">
              {examples.map((ex) => (
                <button
                  key={ex}
                  onClick={() => setPrompt(ex)}
                  className="w-full rounded-md border border-dashed px-3 py-2 text-left text-sm text-muted-foreground hover:bg-accent hover:text-foreground"
                >
                  {ex}
                </button>
              ))}
            </div>
          </div>
        )}
        {log.map((e, i) => (
          <div
            key={i}
            className={
              e.kind === 'user'
                ? 'ml-8 rounded-lg bg-primary px-3 py-2 text-sm text-primary-foreground'
                : 'mr-2 rounded-lg bg-muted px-3 py-2 text-sm'
            }
          >
            {e.text}
            {e.calls?.length > 0 && (
              <div className="mt-1 flex flex-wrap gap-1">
                {e.calls.map((c, j) => (
                  <span key={j} className="rounded bg-background px-1.5 py-0.5 text-xs text-muted-foreground">{c}</span>
                ))}
              </div>
            )}
          </div>
        ))}
        {pending && (
          <div className="rounded-lg border border-amber-500/50 bg-amber-500/10 p-3 text-sm">
            <div className="mb-2 flex items-center gap-2 font-medium text-amber-600 dark:text-amber-400">
              <AlertTriangle className="h-4 w-4" /> Confirmation required
            </div>
            <p className="mb-3 text-muted-foreground">{pending.summary}</p>
            <div className="flex gap-2">
              <Button size="sm" variant="destructive" onClick={() => run(null, pending.confirm_id)} disabled={busy}>
                <Check className="h-4 w-4" /> Confirm
              </Button>
              <Button size="sm" variant="outline" onClick={() => setPending(null)}>Cancel</Button>
            </div>
          </div>
        )}
        {error && <div className="rounded-lg bg-destructive/10 px-3 py-2 text-sm text-destructive">{error}</div>}
      </div>

      <form onSubmit={submit} className="flex items-center gap-2 border-t px-4 py-3">
        <Input
          placeholder="Tell the agent what to do…"
          value={prompt}
          onChange={(e) => setPrompt(e.target.value)}
          disabled={busy}
        />
        <Button type="submit" size="icon" disabled={busy || !prompt.trim()}>
          {busy ? <Loader2 className="h-4 w-4 animate-spin" /> : <Send className="h-4 w-4" />}
        </Button>
      </form>
    </Sheet>
  )
}
