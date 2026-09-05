import { style, styleVariants } from '@vanilla-extract/css'
import { tokens } from '../styles.css'

const base = style({
  background: 'transparent',
  border: 0,
  borderRadius: tokens.radius.pill,
  color: tokens.color.blue,
  fontSize: '14px',
  fontWeight: 600,
  padding: '12px 20px',
  selectors: { '&:hover': { background: tokens.color.blueLight } },
})

export const variants = styleVariants({
  primary: [
    base,
    {
      background: tokens.color.blue,
      color: tokens.color.surface,
      selectors: { '&:hover': { background: '#0842a0' } },
    },
  ],
  text: [base],
})
