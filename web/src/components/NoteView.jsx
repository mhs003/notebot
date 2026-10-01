import { useEffect, useState } from 'react'
import { ArrowLeft, Trash2, Save, Folder } from 'lucide-react'
import { Button } from './ui/button'
import { Input, Textarea } from './ui/input'
import { Badge } from './ui/card'
import { api } from '../lib/api'

export function NoteView({ id, onBack, onChanged }) {
  const [note, setNote] = useState(null)
  const [title, setTitle] = useState('')
  const [body, setBody] = useState('')
  const [saving, setSaving] = useState(false)
  const [confirmDelete, setConfirmDelete] = useState(false)

  useEffect(() => {
    if (!id) return
    api.note(id).then((n) => { setNote(n); setTitle(n.title); setBody(n.body) })
  }, [id])

  if (!note) return <div className="p-8 text-muted-foreground">Loading…</div>

  const save = async () => {
    setSaving(true)
    try {
      await api.updateNote(id, { title, content: body })
      onChanged?.()
      onBack()
    } finally {
      setSaving(false)
    }
  }

  return (
    <div className="mx-auto max-w-3xl p-8">
      <div className="mb-6 flex items-center justify-between">
        <Button variant="ghost" size="sm" onClick={onBack}>
          <ArrowLeft className="h-4 w-4" /> Back
        </Button>
        <div className="flex items-center gap-2">
          {confirmDelete ? (
            <>
              <span className="text-sm text-destructive">Delete this note?</span>
              <Button size="sm" variant="destructive" onClick={async () => { await api.deleteNote(id); onChanged?.(); onBack() }}>
                Yes, delete
              </Button>
              <Button size="sm" variant="outline" onClick={() => setConfirmDelete(false)}>Cancel</Button>
            </>
          ) : (
            <>
              <Button size="sm" variant="outline" onClick={() => setConfirmDelete(true)}>
                <Trash2 className="h-4 w-4" /> Delete
              </Button>
              <Button size="sm" onClick={save} disabled={saving}>
                <Save className="h-4 w-4" /> {saving ? 'Saving…' : 'Save'}
              </Button>
            </>
          )}
        </div>
      </div>

      <Badge className="mb-4 gap-1"><Folder className="h-3 w-3" /> {note.folder}</Badge>
      <Input
        value={title}
        onChange={(e) => setTitle(e.target.value)}
        className="mb-4 border-0 px-0 text-3xl font-semibold shadow-none focus-visible:ring-0"
      />
      <Textarea
        value={body}
        onChange={(e) => setBody(e.target.value)}
        className="min-h-[400px] resize-none border-0 px-0 text-base leading-relaxed shadow-none focus-visible:ring-0"
      />
    </div>
  )
}
