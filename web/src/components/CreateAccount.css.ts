import { style } from '@vanilla-extract/css'
import { tokens } from '../styles.css'

export const field = style({
  display: 'grid',
  gap: tokens.space.tiny,
  marginBottom: tokens.space.medium,
})

export const label = style({
  color: tokens.color.muted,
  fontSize: '14px',
})

export const input = style({
  background: tokens.color.surface,
  border: `1px solid ${tokens.color.muted}`,
  borderRadius: tokens.radius.control,
  color: tokens.color.text,
  padding: '15px',
  width: '100%',
})

export const actions = style({
  display: 'flex',
  gap: tokens.space.small,
  justifyContent: 'flex-end',
})
