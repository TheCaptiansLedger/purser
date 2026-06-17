import { countryFlag, countryName, isKnownCountry } from '../../utils/countryFlag'

interface Props {
  value?: string | null
}

export function CountryChip({ value }: Props) {
  const text = value?.trim()
  if (!text) return null

  if (!isKnownCountry(text)) {
    return <span>{text}</span>
  }

  return (
    <span className="inline-flex items-center gap-1 rounded-md bg-white/8 px-2 py-0.5 text-sm text-white/75">
      <span aria-hidden="true">{countryFlag(text)}</span>
      <span>{countryName(text)}</span>
    </span>
  )
}
