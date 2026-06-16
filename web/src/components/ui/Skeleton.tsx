interface Props {
  className?: string
  aspect?: string
}

export function Skeleton({ className = '', aspect }: Props) {
  return (
    <div
      className={['animate-pulse rounded-xl bg-white/5', className].join(' ')}
      style={aspect ? { aspectRatio: aspect } : {}}
    />
  )
}

export function CardSkeleton({ aspect = '2/3' }: { aspect?: string }) {
  return (
    <div className="flex flex-col gap-2">
      <Skeleton aspect={aspect} className="w-full rounded-xl" />
      <Skeleton className="h-4 w-3/4" />
      <Skeleton className="h-3 w-1/2" />
    </div>
  )
}

export function SkeletonGrid({ count = 20, aspect }: { count?: number; aspect?: string }) {
  return (
    <div className="grid grid-cols-2 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-5 xl:grid-cols-6 gap-4">
      {Array.from({ length: count }).map((_, i) => (
        <CardSkeleton key={i} aspect={aspect} />
      ))}
    </div>
  )
}
