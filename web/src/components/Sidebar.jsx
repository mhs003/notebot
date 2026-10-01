import { Home, Clock, Folder, Settings, FileText, Layers, Hash } from 'lucide-react'
import { cn } from '../lib/utils'

const nav = [
  { id: 'home', label: 'Home', icon: Home },
  { id: 'recent', label: 'Recent', icon: Clock },
  { id: 'folders', label: 'Folders', icon: Folder },
  { id: 'settings', label: 'Settings', icon: Settings },
]

export function Sidebar({ view, setView, activeFolder, setActiveFolder, folders }) {
  return (
    <aside className="flex h-full w-64 shrink-0 flex-col border-r bg-card/50">
      <div className="flex h-16 items-center gap-2 px-5">
        <div className="flex h-8 w-8 items-center justify-center rounded-lg bg-primary text-primary-foreground">
          <Layers className="h-4 w-4" />
        </div>
        <span className="text-lg font-semibold tracking-tight">Notebot</span>
      </div>

      <nav className="space-y-1 px-3">
        {nav.map(({ id, label, icon: Icon }) => (
          <button
            key={id}
            onClick={() => { setView(id); setActiveFolder(null) }}
            className={cn(
              'flex w-full items-center gap-3 rounded-md px-3 py-2 text-sm font-medium transition-colors',
              view === id && !activeFolder
                ? 'bg-primary/10 text-primary'
                : 'text-muted-foreground hover:bg-accent hover:text-foreground'
            )}
          >
            <Icon className="h-4 w-4" />
            {label}
          </button>
        ))}
      </nav>

      <div className="mt-6 px-5 text-xs font-semibold uppercase tracking-wider text-muted-foreground">
        Folders
      </div>
      <div className="mt-2 flex-1 space-y-0.5 overflow-y-auto px-3 pb-4">
        <button
          onClick={() => { setActiveFolder('inbox'); setView('folders') }}
          className={cn(
            'flex w-full items-center gap-3 rounded-md px-3 py-2 text-sm transition-colors',
            activeFolder === 'inbox' ? 'bg-accent font-medium' : 'text-muted-foreground hover:bg-accent'
          )}
        >
          <Hash className="h-4 w-4" /> inbox
        </button>
        {folders.filter((f) => f !== 'inbox').map((f) => (
          <button
            key={f}
            onClick={() => { setActiveFolder(f); setView('folders') }}
            className={cn(
              'flex w-full items-center gap-3 rounded-md px-3 py-2 text-sm transition-colors',
              activeFolder === f ? 'bg-accent font-medium' : 'text-muted-foreground hover:bg-accent'
            )}
          >
            <Folder className="h-4 w-4" /> {f}
          </button>
        ))}
        {folders.length === 0 && (
          <p className="px-3 py-2 text-xs text-muted-foreground">No folders yet</p>
        )}
      </div>

      <div className="border-t px-5 py-3 text-xs text-muted-foreground">
        <div className="flex items-center gap-2">
          <FileText className="h-3.5 w-3.5" />
          Powered by Needle3
        </div>
      </div>
    </aside>
  )
}
