import { useState } from 'react'
import { Dialog, DialogTitle, DialogDescription } from './ui/dialog'
import { Button } from './ui/button'
import { Input, Textarea } from './ui/input'

export function CreateModal({ open, onClose, onSaved, folders }) {
  const [title, setTitle] = useState('')
  const [folder, setFolder] = useState('inbox')
  const [content, setContent] = useState('')
  const [busy, setBusy] = useState(false)

  const reset = () => { setTitle(''); setFolder('inbox'); setContent('') }

  const submit = async (e) => {
    e.preventDefault()
    setBusy(true)
    try {
      await onSaved({ title: title || 'Untitled', folder: folder || 'inbox', body: content })
      reset()
      onClose()
    } finally {
      setBusy(false)
    }
  }

  return (
    <Dialog open={open} onClose={onClose}>
      <DialogTitle>Create note</DialogTitle>
      <DialogDescription className="mb-4">Write a note directly — no model involved.</DialogDescription>
      <form onSubmit={submit} className="space-y-3">
        <Input placeholder="Title" value={title} onChange={(e) => setTitle(e.target.value)} autoFocus />
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
          <Button type="submit" disabled={busy}>{busy ? 'Saving…' : 'Save note'}</Button>
        </div>
      </form>
    </Dialog>
  )
}
