import { useEffect, useState } from 'react'
import { ArrowLeft, Trash2, Save, Folder } from 'lucide-react'
import { Button } from './ui/button'
import { Input, Textarea } from './ui/input'
import { Badge } from './ui/card'
import { api } from '../lib/api'

export function NoteView({ id, folders, onBack, onChanged }) {
  const [note, setNote] = useState(null)
  const [title, setTitle] = useState('')
  const [body, setBody] = useState('')
  const [folder, setFolder] = useState('')
  const [saving, setSaving] = useState(false)
  const [confirmDelete, setConfirmDelete] = useState(false)

  useEffect(() => {
    if (!id) return
    api.note(id).then((n) => {
      setNote(n); setTitle(n.title); setBody(n.body); setFolder(n.folder)
    })
  }, [id])

  if (!note) return <div className="p-8 text-muted-foreground">Loading…</div>

  const save = async () => {
    setSaving(true)
    try {
      await api.updateNote(id, { title, content: body, folder })
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

      <div className="mb-4 flex items-center gap-2">
        <Folder className="h-4 w-4 text-muted-foreground" />
        <Input
          value={folder}
          list="note-folder-list"
          onChange={(e) => setFolder(e.target.value)}
          className="h-8 w-48"
        />
        <datalist id="note-folder-list">
          {folders.map((f) => <option key={f} value={f} />)}
        </datalist>
        <span className="text-xs text-muted-foreground">
          created {new Date(note.created).toLocaleString()} · updated {new Date(note.updated).toLocaleString()}
        </span>
      </div>

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
