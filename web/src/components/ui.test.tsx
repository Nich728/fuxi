import { act, render, screen } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import { Roll, WeekDots } from './ui'

describe('WeekDots', () => {
  it('shows Monday to Sunday: done days, today, and days to come', () => {
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
    const { container } = render(<WeekDots days={days} today="2026-11-25" />)
    const items = [...container.querySelectorAll('.day')]
    expect(items.map((d) => d.querySelector('.day-glyph')?.textContent)).toEqual(['一', '二', '三', '四', '五', '六', '日'])
    expect(items.map((d) => d.className.trim().replace('day day-', ''))).toEqual([
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

  it('fills today with a ripple only when asked to celebrate a finished day', () => {
    const days = [{ date: '2026-11-25', reviews: 20 }]
    const { container, rerender } = render(<WeekDots days={days} today="2026-11-25" />)
    expect(container.querySelector('.day-celebrate')).toBeNull()
    rerender(<WeekDots days={days} today="2026-11-25" celebrate />)
    expect(container.querySelector('.day-celebrate .day-glyph')?.textContent).toBe('三')
  })
})

describe('Roll', () => {
  it('rolls from the old value to the new one, and reads as the new one', () => {
    const { container, rerender } = render(<Roll value={41} />)
    expect(container.querySelector('.roll')).not.toHaveClass('is-rolling')
    act(() => rerender(<Roll value={42} />))
    const roll = container.querySelector('.roll')!
    expect(roll).toHaveClass('is-rolling')
    expect(roll.querySelector('[aria-hidden]')?.textContent).toBe('4142')
    expect(roll.querySelector('.visually-hidden')?.textContent).toBe('42')
  })
})
