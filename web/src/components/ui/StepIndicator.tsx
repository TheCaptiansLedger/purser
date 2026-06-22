interface StepIndicatorProps {
  current: number
  total: number
}

export function stepLabel(current: number, total: number): string {
  return `Step ${current} of ${total}`
}

export function StepIndicator({ current, total }: StepIndicatorProps) {
  return (
    <div className="flex flex-col items-center gap-3">
      <div className="flex items-center gap-2">
        {Array.from({ length: total }, (_, i) => {
          const n = i + 1
          const isPast    = n < current
          const isCurrent = n === current
          return (
            <div
              key={n}
              className={[
                'rounded-full transition-all duration-300',
                isCurrent ? 'w-6 h-2.5 bg-indigo-500' : 'w-2.5 h-2.5',
                isPast    ? 'bg-indigo-800' : '',
                !isCurrent && !isPast ? 'bg-gray-700' : '',
              ].join(' ')}
            />
          )
        })}
      </div>
      <p className="text-xs font-medium tracking-widest uppercase text-gray-500">
        {stepLabel(current, total)}
      </p>
    </div>
  )
}
