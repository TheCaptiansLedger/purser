import { Tv2 } from 'lucide-react'
import { GenreFilteredPage } from '../../components/GenreFilteredPage'

export function TVGenrePage() {
  return (
    <GenreFilteredPage
      contentType="tv"
      kind="series"
      accent="#8b5cf6"
      toEntryHref={entry => `/tv/${entry.id}`}
      icon={Tv2}
      emptyNoun="shows"
    />
  )
}
