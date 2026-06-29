interface Props {
  backdropUrl?: string
  backdropBlur?: boolean
  accent?: string
  children: React.ReactNode
}

export function Hero({ backdropUrl, backdropBlur = true, accent, children }: Props) {
  const backdropFilter = backdropBlur
    ? 'blur(20px) brightness(0.35)'
    : 'blur(6px) brightness(0.55)'

  return (
    <div className="relative min-h-[36rem] flex items-end overflow-hidden">
      {/* Backdrop */}
      {backdropUrl ? (
        <>
          <div
            className="absolute inset-0 bg-cover bg-center scale-105"
            style={{ backgroundImage: `url(${backdropUrl})`, filter: backdropFilter }}
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
