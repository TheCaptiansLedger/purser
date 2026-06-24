import { useNavigate } from 'react-router-dom'
import { TagCloudPage } from '../../components/TagCloudPage'
import type { Tag } from '../../types'

export function MovieTagsPage() {
  const navigate = useNavigate()
  return (
    <TagCloudPage
      contentType="movie"
      accent="#3b82f6"
      onTagClick={(tag: Tag) => navigate(`/movies/genre/${encodeURIComponent(tag.value.toLowerCase().replace(/\s+/g, '-'))}`)}
    />
  )
}
