import { useNavigate } from 'react-router-dom'
import { TagCloudPage } from '../../components/TagCloudPage'
import type { Tag } from '../../types'

export function TVTagsPage() {
  const navigate = useNavigate()
  return (
    <TagCloudPage
      contentType="tv"
      accent="#8b5cf6"
      onTagClick={(tag: Tag) => navigate(`/tv/genre/${encodeURIComponent(tag.value.toLowerCase().replace(/\s+/g, '-'))}`)}
    />
  )
}
