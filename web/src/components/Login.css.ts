import { style } from '@vanilla-extract/css'
import { tokens } from '../styles.css'

export const note = style({
  color: tokens.color.muted,
  fontSize: '14px',
  lineHeight: 1.5,
})

export const error = style({
  color: tokens.color.red,
  lineHeight: 1.5,
})
