import { describe, expect, it } from 'vitest'
import { resolveSetupRedirect } from './SetupGuard'

describe('resolveSetupRedirect', () => {
  it('redirects to /setup when setup is incomplete and user is not on /setup', () => {
    expect(resolveSetupRedirect(false, '/')).toBe('/setup')
    expect(resolveSetupRedirect(false, '/movies')).toBe('/setup')
    expect(resolveSetupRedirect(false, '/settings/config')).toBe('/setup')
  })

  it('returns null when setup is incomplete and user is already on /setup', () => {
    expect(resolveSetupRedirect(false, '/setup')).toBeNull()
  })

  it('redirects to / when setup is complete and user is on /setup', () => {
    expect(resolveSetupRedirect(true, '/setup')).toBe('/')
  })

  it('returns null when setup is complete and user is on any other page', () => {
    expect(resolveSetupRedirect(true, '/')).toBeNull()
    expect(resolveSetupRedirect(true, '/movies')).toBeNull()
    expect(resolveSetupRedirect(true, '/settings/config')).toBeNull()
  })
})
