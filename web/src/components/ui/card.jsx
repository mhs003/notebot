import { cn } from '../../lib/utils'

export function Card({ className, ...props }) {
  return <div className={cn('rounded-lg border bg-card text-card-foreground shadow-sm', className)} {...props} />
}

export function Badge({ className, ...props }) {
  return (
    <span
      className={cn(
        'inline-flex items-center rounded-full border px-2.5 py-0.5 text-xs font-medium text-muted-foreground',
        className
      )}
      {...props}
    />
  )
}
