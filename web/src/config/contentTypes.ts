import { Film, Tv, Music2, BookOpen, Video, Layers } from 'lucide-react'
import type { LucideIcon } from 'lucide-react'
import type { ContentType, LibraryEntry, Item, Group } from '../types'

export interface ContentTypeRouting {
  entrySectionLabel: string
  entryTypeLabel:    string
  itemSectionLabel:  string
  itemTypeLabel:     string
  icon:              LucideIcon
  entryPath:  (entry: LibraryEntry) => string
  itemPath:   (item: Item)          => string
  groupPath?: (group: Group)        => string
}

export const contentTypeConfig: Record<ContentType, ContentTypeRouting> = {
  movie: {
    entrySectionLabel: 'Movies',
    entryTypeLabel:    'Movie',
    itemSectionLabel:  'Movies',
    itemTypeLabel:     'Movie',
    icon:              Film,
    entryPath:  (e) => `/movies/${e.id}`,
    itemPath:   (i) => `/movies/${i.libraryEntryId}`,
  },
  tv: {
    entrySectionLabel: 'TV Shows',
    entryTypeLabel:    'TV Series',
    itemSectionLabel:  'Episodes',
    itemTypeLabel:     'Episode',
    icon:              Tv,
    entryPath:  (e) => `/tv/${e.id}`,
    itemPath:   (i) => `/tv/${i.libraryEntryId}`,
    groupPath:  (g) => `/tv/${g.libraryEntryId}/seasons/${g.number}`,
  },
  music: {
    entrySectionLabel: 'Bands & Artists',
    entryTypeLabel:    'Artist',
    itemSectionLabel:  'Tracks',
    itemTypeLabel:     'Track',
    icon:              Music2,
    entryPath:  (e) => `/music/${e.id}`,
    itemPath:   (i) => `/music/${i.libraryEntryId}`,
    groupPath:  (g) => `/music/${g.libraryEntryId}/albums/${g.id}`,
  },
  adult: {
    entrySectionLabel: 'Studios & Networks',
    entryTypeLabel:    'Studio',
    itemSectionLabel:  'Scenes',
    itemTypeLabel:     'Scene',
    icon:              Video,
    entryPath:  (e) => e.kind === 'network' ? `/afterdark/networks/${e.id}` : `/afterdark/studios/${e.id}`,
    itemPath:   (i) => `/afterdark/scenes/${i.id}`,
  },
  jav: {
    entrySectionLabel: 'Studios',
    entryTypeLabel:    'Studio',
    itemSectionLabel:  'JAV Titles',
    itemTypeLabel:     'Scene',
    icon:              Video,
    entryPath:  (e) => `/afterdark/studios/${e.id}`,
    itemPath:   (i) => `/afterdark/scenes/${i.id}`,
  },
  book: {
    entrySectionLabel: 'Publishers & Books',
    entryTypeLabel:    'Book',
    itemSectionLabel:  'Books',
    itemTypeLabel:     'Book',
    icon:              BookOpen,
    entryPath:  (e) => `/books/${e.id}`,
    itemPath:   (i) => `/books/${i.libraryEntryId}`,
  },
}

export const defaultIcon = Layers
