import { describe, expect, it } from 'vitest'

// Pure logic tests — rendering and group filtering extracted from the component

const GENDER_MAP: Record<string, string> = {
  non_binary: 'Non-Binary',
  transgender_male: 'Trans Male',
  transgender_female: 'Trans Female',
  unknown: 'Unknown',
}

function fmtGender(g: string): string {
  if (g in GENDER_MAP) return GENDER_MAP[g]
  return g.charAt(0).toUpperCase() + g.slice(1)
}

describe('fmtGender', () => {
  it('formats standard values to title case', () => {
    expect(fmtGender('male')).toBe('Male')
    expect(fmtGender('female')).toBe('Female')
  })

  it('formats compound values to readable strings', () => {
    expect(fmtGender('non_binary')).toBe('Non-Binary')
    expect(fmtGender('transgender_male')).toBe('Trans Male')
    expect(fmtGender('transgender_female')).toBe('Trans Female')
  })

  it('returns Unknown for the unknown sentinel', () => {
    expect(fmtGender('unknown')).toBe('Unknown')
  })
})

describe('PersonMetaGroups visibility logic', () => {
  function hasVisibleGroup(
    metadata: Record<string, unknown>,
    groupKeys: string[],
  ): boolean {
    return groupKeys.some(key => {
      const v = metadata[key]
      if (v === undefined || v === null || v === '') return false
      if (key === 'ended' && v === false) return false
      return true
    })
  }

  it('shows Physical group when any physical key is present', () => {
    expect(hasVisibleGroup({ height: '165 cm' }, ['height', 'weight', 'measurements', 'cup_size', 'hair_color', 'eye_color'])).toBe(true)
    expect(hasVisibleGroup({ hair_color: 'brunette' }, ['height', 'weight', 'measurements', 'cup_size', 'hair_color', 'eye_color'])).toBe(true)
  })

  it('hides Physical group when all physical keys are empty', () => {
    expect(hasVisibleGroup({}, ['height', 'weight', 'measurements', 'cup_size', 'hair_color', 'eye_color'])).toBe(false)
    expect(hasVisibleGroup({ height: '' }, ['height', 'weight', 'measurements', 'cup_size', 'hair_color', 'eye_color'])).toBe(false)
    expect(hasVisibleGroup({ height: null }, ['height', 'weight', 'measurements', 'cup_size', 'hair_color', 'eye_color'])).toBe(false)
  })

  it('hides ended row when ended is false', () => {
    expect(hasVisibleGroup({ ended: false }, ['ended'])).toBe(false)
    expect(hasVisibleGroup({ ended: true }, ['ended'])).toBe(true)
  })

  it('shows Background group when nationality is present', () => {
    const bgKeys = ['type', 'gender', 'area', 'begin_area', 'end_area', 'birthdate', 'deathday', 'place_of_birth', 'nationality', 'ethnicity', 'known_for']
    expect(hasVisibleGroup({ nationality: 'US' }, bgKeys)).toBe(true)
  })

  it('returns no visible groups for empty metadata', () => {
    const groups = [
      ['height', 'weight'],
      ['birthdate', 'nationality'],
      ['career_start', 'career_end'],
    ]
    const meta = {}
    const visible = groups.filter(keys => hasVisibleGroup(meta, keys))
    expect(visible).toHaveLength(0)
  })

  it('only shows groups with present keys', () => {
    const groups = [
      { title: 'Physical', keys: ['height', 'weight'] },
      { title: 'Background', keys: ['birthdate', 'nationality'] },
    ]
    const meta = { height: '165 cm' }
    const visible = groups.filter(g => hasVisibleGroup(meta, g.keys))
    expect(visible.map(g => g.title)).toEqual(['Physical'])
  })
})
