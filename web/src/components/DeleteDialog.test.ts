import { describe, expect, it } from 'vitest'
import { deleteDialogHeading, deleteButtonLabel } from './DeleteDialog'

describe('deleteDialogHeading', () => {
  it('says Delete for destroy mode', () => {
    expect(deleteDialogHeading('destroy', 'Radiohead')).toBe('Delete Radiohead')
  })

  it('says Remove for unlink mode', () => {
    expect(deleteDialogHeading('unlink', 'Jazz Tag')).toBe('Remove Jazz Tag')
  })
})

describe('deleteButtonLabel', () => {
  it('shows Deleting… while in progress', () => {
    expect(deleteButtonLabel('destroy', true)).toBe('Deleting…')
    expect(deleteButtonLabel('unlink', true)).toBe('Deleting…')
  })

  it('shows Delete for destroy mode when idle', () => {
    expect(deleteButtonLabel('destroy', false)).toBe('Delete')
  })

  it('shows Remove for unlink mode when idle', () => {
    expect(deleteButtonLabel('unlink', false)).toBe('Remove')
  })
})
