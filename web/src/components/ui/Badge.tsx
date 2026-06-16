interface Props {
  children: React.ReactNode
  color?: string
  className?: string
}

export function Badge({ children, color, className = '' }: Props) {
  return (
    <span
      className={['inline-flex items-center px-2 py-0.5 rounded-md text-xs font-medium', className].join(' ')}
      style={color ? { background: color + '22', color } : { background: 'rgba(255,255,255,0.08)', color: 'rgba(255,255,255,0.55)' }}
    >
      {children}
    </span>
  )
}
