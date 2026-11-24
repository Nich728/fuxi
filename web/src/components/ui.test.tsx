import { render, screen } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import { SealWeek } from './ui'

describe('SealWeek', () => {
  it('shows Monday to Sunday: sealed days, today, and days to come', () => {
    // Today is Wednesday 25 November 2026.
    const days = [
      { date: '2026-11-19', reviews: 3 },
      { date: '2026-11-20', reviews: 0 },
      { date: '2026-11-21', reviews: 0 },
      { date: '2026-11-22', reviews: 0 },
      { date: '2026-11-23', reviews: 12 },
      { date: '2026-11-24', reviews: 0 },
      { date: '2026-11-25', reviews: 0 },
    ]
    const { container } = render(<SealWeek days={days} today="2026-11-25" />)
    const seals = [...container.querySelectorAll('.seal')]
    expect(seals.map((s) => s.textContent?.[0])).toEqual(['一', '二', '三', '四', '五', '六', '日'])
    expect(seals.map((s) => s.className.replace('seal seal-', ''))).toEqual([
      'done',
      'missed',
      'today',
      'future',
      'future',
      'future',
      'future',
    ])
    expect(screen.getByRole('list', { name: 'This week: studied on 1 of 7 days' })).toBeInTheDocument()
  })
})
