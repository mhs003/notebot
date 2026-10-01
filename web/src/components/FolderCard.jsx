import { useState } from 'react'
import { Folder, Pencil, Trash2, Check, X } from 'lucide-react'
import { Card } from './ui/card'
import { Button } from './ui/button'
import { Input } from './ui/input'

export function FolderCard({ name, count, onClick, onRename, onDelete }) {
  const [editing, setEditing] = useState(false)
  const [value, setValue] = useState(name)
  const [confirming, setConfirming] = useState(false)

  const stop = (e) => e.stopPropagation()

  const commit = async (e) => {
    e?.stopPropagation()
    if (value.trim() && value !== name) await onRename(name, value.trim())
    setEditing(false)
  }

  if (editing) {
    return (
      <Card className="flex items-center gap-2 p-4">
        <Input
          value={value}
          autoFocus
          onClick={stop}
          onChange={(e) => setValue(e.target.value)}
          onKeyDown={(e) => { if (e.key === 'Enter') commit(e); if (e.key === 'Escape') setEditing(false) }}
          className="h-9"
        />
        <Button size="icon" variant="ghost" onClick={commit} aria-label="Rename">
          <Check className="h-4 w-4" />
        </Button>
        <Button size="icon" variant="ghost" onClick={(e) => { stop(e); setEditing(false) }} aria-label="Cancel">
          <X className="h-4 w-4" />
        </Button>
      </Card>
    )
  }

  return (
    <Card
      onClick={onClick}
      className="group flex cursor-pointer items-center gap-3 p-4 transition-all hover:border-primary/40 hover:shadow-md"
    >
      <div className="flex h-10 w-10 items-center justify-center rounded-lg bg-primary/10 text-primary">
        <Folder className="h-5 w-5" />
      </div>
      <div className="min-w-0 flex-1">
        <div className="truncate font-medium">{name}</div>
        <div className="text-xs text-muted-foreground">{count} note{count === 1 ? '' : 's'}</div>
      </div>
      {confirming ? (
        <div className="flex items-center gap-1" onClick={stop}>
          <span className="text-xs text-destructive">delete?</span>
          <Button size="icon" variant="ghost" onClick={(e) => { stop(e); onDelete(name) }} aria-label="Confirm delete">
            <Check className="h-4 w-4" />
          </Button>
          <Button size="icon" variant="ghost" onClick={(e) => { stop(e); setConfirming(false) }} aria-label="Cancel delete">
            <X className="h-4 w-4" />
          </Button>
        </div>
      ) : (
        <div className="flex items-center gap-1 opacity-0 transition-opacity group-hover:opacity-100">
          <Button size="icon" variant="ghost" onClick={(e) => { stop(e); setValue(name); setEditing(true) }} aria-label="Rename folder">
            <Pencil className="h-4 w-4" />
          </Button>
          <Button size="icon" variant="ghost" onClick={(e) => { stop(e); setConfirming(true) }} aria-label="Delete folder">
            <Trash2 className="h-4 w-4" />
          </Button>
        </div>
      )}
    </Card>
  )
}
