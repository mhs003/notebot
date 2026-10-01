import { useEffect } from 'react'
import { X } from 'lucide-react'
import { cn } from '../../lib/utils'
import { Button } from './button'

export function Dialog({ open, onClose, children, className }) {
  useEffect(() => {
    if (!open) return
    const onKey = (e) => e.key === 'Escape' && onClose()
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [open, onClose])

  if (!open) return null
  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center">
      <div className="absolute inset-0 bg-black/50 backdrop-blur-sm" onClick={onClose} />
      <div className={cn('relative z-10 w-full max-w-lg rounded-lg border bg-card p-6 shadow-lg', className)}>
        {children}
      </div>
    </div>
  )
}

export function Sheet({ open, onClose, children, className }) {
  useEffect(() => {
    if (!open) return
    const onKey = (e) => e.key === 'Escape' && onClose()
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [open, onClose])

  if (!open) return null
  return (
    <div className="fixed inset-0 z-50">
      <div className="absolute inset-0 bg-black/40" onClick={onClose} />
      <div className={cn('absolute right-0 top-0 flex h-full w-full max-w-md flex-col border-l bg-card shadow-xl', className)}>
        {children}
      </div>
    </div>
  )
}

export function SheetHeader({ children, onClose }) {
  return (
    <div className="flex items-center justify-between border-b px-6 py-4">
      <div>{children}</div>
      <Button variant="ghost" size="icon" onClick={onClose} aria-label="Close">
        <X className="h-4 w-4" />
      </Button>
    </div>
  )
}

export function DialogTitle({ children }) {
  return <h2 className="text-lg font-semibold tracking-tight">{children}</h2>
}

export function DialogDescription({ children }) {
  return <p className="text-sm text-muted-foreground">{children}</p>
}
