interface Props {
  backdropUrl?: string
  accent?: string
  children: React.ReactNode
}

export function Hero({ backdropUrl, accent, children }: Props) {
  return (
    <div className="relative min-h-[28rem] flex items-end overflow-hidden">
      {/* Backdrop */}
      {backdropUrl ? (
        <>
          <div
            className="absolute inset-0 bg-cover bg-center scale-105"
            style={{ backgroundImage: `url(${backdropUrl})`, filter: 'blur(20px) brightness(0.35)' }}
          />
          <div
            className="absolute inset-0"
            style={{
              background: `linear-gradient(to bottom, ${accent ?? '#000'}22 0%, #08080e 100%)`,
            }}
          />
        </>
      ) : (
        <div
          className="absolute inset-0"
          style={{
            background: accent
              ? `linear-gradient(135deg, ${accent}18 0%, #08080e 60%)`
              : 'linear-gradient(to bottom, rgba(255,255,255,0.03) 0%, #08080e 100%)',
          }}
        />
      )}
      {/* Content */}
      <div className="relative z-10 w-full px-8 py-10">{children}</div>
    </div>
  )
}
