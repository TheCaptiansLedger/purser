import { Film } from 'lucide-react'
import { GenreFilteredPage } from '../../components/GenreFilteredPage'

export function MovieGenrePage() {
  return (
    <GenreFilteredPage
      contentType="movie"
      kind="movie"
      accent="#3b82f6"
      toEntryHref={entry => `/movies/${entry.id}`}
      icon={Film}
      emptyNoun="movies"
    />
  )
}
