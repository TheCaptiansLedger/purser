import type { Tag, ContentType } from '../types'

export function filterTagsForModule(tags: Tag[], contentType: ContentType): Tag[] {
  if (contentType === 'adult' || contentType === 'jav') return tags
  return tags.filter(t => t.key !== 'adult')
}
