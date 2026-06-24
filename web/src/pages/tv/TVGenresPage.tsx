import { Clapperboard } from 'lucide-react'
import { GenreListPage } from '../../components/GenreListPage'

export function TVGenresPage() {
  return (
    <GenreListPage
      contentType="tv"
      accent="#8b5cf6"
      toEntryHref={slug => `/tv/genre/${slug}`}
      icon={Clapperboard}
      emptyDescription="Tag TV series with key=genre to see them here."
    />
  )
}
