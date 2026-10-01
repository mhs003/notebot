import { useState } from 'react'
import { AlertTriangle } from 'lucide-react'
import { Dialog, DialogTitle, DialogDescription } from './ui/dialog'
import { Button } from './ui/button'
import { Input, Textarea } from './ui/input'

export function CreateModal({ open, onClose, onSaved, folders, existingTitles = [] }) {
  const [title, setTitle] = useState('')
  const [folder, setFolder] = useState('inbox')
  const [content, setContent] = useState('')
  const [busy, setBusy] = useState(false)

  const reset = () => { setTitle(''); setFolder('inbox'); setContent('') }
  const duplicate = existingTitles.some((t) => t.trim().toLowerCase() === title.trim().toLowerCase())

  const save = async () => {
    setBusy(true)
    try {
      await onSaved({ title: title || 'Untitled', folder: folder || 'inbox', body: content })
      reset()
      onClose()
    } finally {
      setBusy(false)
    }
  }

  const submit = async (e) => {
    e.preventDefault()
    // Same rule as the CLI: warn before creating a second note with this title.
    if (duplicate && !confirm(`A note titled "${title}" already exists. Create another?`)) return
    await save()
  }

  return (
    <Dialog open={open} onClose={onClose}>
      <DialogTitle>Create note</DialogTitle>
      <DialogDescription className="mb-4">Write a note directly — no model involved.</DialogDescription>
      <form onSubmit={submit} className="space-y-3">
        <Input placeholder="Title" value={title} onChange={(e) => setTitle(e.target.value)} autoFocus />
        {duplicate && (
          <div className="flex items-center gap-2 rounded-md border border-amber-500/50 bg-amber-500/10 px-3 py-2 text-sm text-amber-600 dark:text-amber-400">
            <AlertTriangle className="h-4 w-4 shrink-0" />
            A note with this title already exists.
          </div>
        )}
        <div>
          <Input
            placeholder="Folder"
            value={folder}
            list="folder-list"
            onChange={(e) => setFolder(e.target.value)}
          />
          <datalist id="folder-list">
            {folders.map((f) => <option key={f} value={f} />)}
          </datalist>
        </div>
        <Textarea placeholder="Write your note…" value={content} onChange={(e) => setContent(e.target.value)} />
        <div className="flex justify-end gap-2 pt-2">
          <Button type="button" variant="outline" onClick={onClose}>Cancel</Button>
          <Button type="submit" disabled={busy}>{busy ? 'Saving…' : duplicate ? 'Create anyway' : 'Save note'}</Button>
        </div>
      </form>
    </Dialog>
  )
}
