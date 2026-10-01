import { useCallback, useEffect, useMemo, useState } from 'react'
import { Search, Plus, Bot, RefreshCw } from 'lucide-react'
import { Sidebar } from './components/Sidebar'
import { NoteCard, FolderCard } from './components/NoteCard'
import { CreateModal } from './components/CreateModal'
import { AgentSheet } from './components/AgentSheet'
import { NoteView } from './components/NoteView'
import { Login } from './pages/Login'
import { Settings } from './pages/Settings'
import { Button } from './components/ui/button'
import { Input } from './components/ui/input'
import { api, Unauthorized } from './lib/api'

export default function App() {
  const [authed, setAuthed] = useState(null)
  const [view, setView] = useState('home')
  const [activeFolder, setActiveFolder] = useState(null)
  const [notes, setNotes] = useState([])
  const [folders, setFolders] = useState([])
  const [query, setQuery] = useState('')
  const [loading, setLoading] = useState(false)
  const [openNote, setOpenNote] = useState(null)
  const [showCreate, setShowCreate] = useState(false)
  const [showAgent, setShowAgent] = useState(false)

  useEffect(() => {
    api.status()
      .then((s) => setAuthed(s.authed || !s.password_set))
      .catch(() => setAuthed(false))
  }, [])

  const load = useCallback(async () => {
    setLoading(true)
    try {
      const [n, f] = await Promise.all([api.notes(), api.folders()])
      setNotes(n || [])
      setFolders(f || [])
    } catch (e) {
      if (e instanceof Unauthorized) setAuthed(false)
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => { if (authed) load() }, [authed, load])

  const visible = useMemo(() => {
    let list = notes
    if (view === 'recent') list = [...list].sort((a, b) => new Date(b.updated) - new Date(a.updated)).slice(0, 30)
    if (view === 'folders' && activeFolder) list = list.filter((n) => n.folder === activeFolder)
    if (query.trim()) {
      const q = query.toLowerCase()
      list = list.filter((n) => n.title.toLowerCase().includes(q) || (n.body || '').toLowerCase().includes(q))
    }
    return list
  }, [notes, view, activeFolder, query])

  const folderCounts = useMemo(() => {
    const m = {}
    for (const n of notes) m[n.folder] = (m[n.folder] || 0) + 1
    return m
  }, [notes])

  const logout = async () => { await api.logout().catch(() => {}); setAuthed(false) }

  if (authed === null) return <div className="flex min-h-screen items-center justify-center text-muted-foreground">Loading…</div>
  if (!authed) return <Login onDone={() => setAuthed(true)} />

  const heading = openNote ? '' : view === 'home' ? 'Home' : view === 'recent' ? 'Recent' : view === 'folders' ? (activeFolder ? `#${activeFolder}` : 'Folders') : 'Settings'

  return (
    <div className="flex h-screen overflow-hidden">
      <Sidebar
        view={view}
        setView={setView}
        activeFolder={activeFolder}
        setActiveFolder={setActiveFolder}
        folders={folders}
      />

      <main className="flex flex-1 flex-col overflow-hidden">
        {openNote ? (
          <div className="flex-1 overflow-y-auto">
            <NoteView id={openNote} onBack={() => setOpenNote(null)} onChanged={load} />
          </div>
        ) : view === 'settings' ? (
          <div className="flex-1 overflow-y-auto">
            <Settings onLogout={logout} />
          </div>
        ) : (
          <>
            <header className="flex h-16 shrink-0 items-center gap-4 border-b px-6">
              <h1 className="text-xl font-semibold tracking-tight">{heading}</h1>
              <div className="relative ml-auto w-full max-w-sm">
                <Search className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
                <Input
                  placeholder="Search notes…"
                  value={query}
                  onChange={(e) => setQuery(e.target.value)}
                  className="pl-9"
                />
              </div>
              <Button variant="ghost" size="icon" onClick={load} aria-label="Refresh">
                <RefreshCw className={loading ? 'h-4 w-4 animate-spin' : 'h-4 w-4'} />
              </Button>
            </header>

            <div className="flex-1 overflow-y-auto p-6">
              {view === 'folders' && !activeFolder ? (
                <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4">
                  {folders.length === 0 && (
                    <p className="col-span-full py-16 text-center text-muted-foreground">No folders yet. Capture a note to create one.</p>
                  )}
                  {folders.map((f) => (
                    <FolderCard key={f} name={f} count={folderCounts[f] || 0} onClick={() => setActiveFolder(f)} />
                  ))}
                </div>
              ) : visible.length === 0 ? (
                <div className="flex flex-col items-center justify-center py-24 text-center">
                  <p className="text-lg font-medium">No notes yet</p>
                  <p className="mt-1 text-sm text-muted-foreground">
                    Run <code className="rounded bg-muted px-1.5 py-0.5">nb "your note"</code> or click Create.
                  </p>
                </div>
              ) : (
                <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4">
                  {visible.map((n) => <NoteCard key={n.id} note={n} onClick={() => setOpenNote(n.id)} />)}
                </div>
              )}
            </div>
          </>
        )}
      </main>

      <div className="fixed bottom-6 right-6 z-40 flex gap-3">
        <Button className="shadow-lg" onClick={() => setShowCreate(true)}>
          <Plus className="h-4 w-4" /> Create
        </Button>
        <Button className="shadow-lg" variant="secondary" onClick={() => setShowAgent(true)}>
          <Bot className="h-4 w-4" /> Agent
        </Button>
      </div>

      <CreateModal
        open={showCreate}
        onClose={() => setShowCreate(false)}
        folders={folders}
        onSaved={async (note) => { await api.createNote(note); load() }}
      />
      <AgentSheet open={showAgent} onClose={() => setShowAgent(false)} onChanged={load} />
    </div>
  )
}
