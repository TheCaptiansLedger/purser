import type { Tag } from '../types'

export function filterTagsForModule(tags: Tag[], module: string): Tag[] {
  if (module === 'afterdark') return tags
  return tags.filter(t => t.key !== 'adult')
}
