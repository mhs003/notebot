import { FileText, Folder } from 'lucide-react'
import { Card, Badge } from './ui/card'
import { timeAgo } from '../lib/utils'

export function NoteCard({ note, onClick }) {
  return (
    <Card
      onClick={onClick}
      className="group cursor-pointer p-4 transition-all hover:border-primary/40 hover:shadow-md"
    >
      <div className="mb-2 flex items-start justify-between gap-2">
        <h3 className="line-clamp-2 font-medium leading-snug">{note.title}</h3>
        <FileText className="h-4 w-4 shrink-0 text-muted-foreground" />
      </div>
      <p className="mb-3 line-clamp-3 whitespace-pre-wrap text-sm text-muted-foreground">
        {note.body || 'Empty note'}
      </p>
      <div className="flex items-center justify-between">
        <Badge className="gap-1">
          <Folder className="h-3 w-3" /> {note.folder}
        </Badge>
        <span className="text-xs text-muted-foreground">{timeAgo(note.updated)}</span>
      </div>
    </Card>
  )
}
