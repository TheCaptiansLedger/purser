import { describe, expect, it } from 'vitest'
import { stepLabel } from './StepIndicator'

describe('stepLabel', () => {
  it('formats the current step and total', () => {
    expect(stepLabel(1, 5)).toBe('Step 1 of 5')
    expect(stepLabel(2, 5)).toBe('Step 2 of 5')
    expect(stepLabel(5, 5)).toBe('Step 5 of 5')
  })

  it('works for any total', () => {
    expect(stepLabel(3, 10)).toBe('Step 3 of 10')
    expect(stepLabel(1, 1)).toBe('Step 1 of 1')
  })
})
