import { Clapperboard } from 'lucide-react'
import { GenreListPage } from '../../components/GenreListPage'

export function GenresPage() {
  return (
    <GenreListPage
      contentType="movie"
      accent="#3b82f6"
      toEntryHref={slug => `/movies/genre/${slug}`}
      icon={Clapperboard}
      emptyDescription="Tag movies with key=genre to see them here."
    />
  )
}
