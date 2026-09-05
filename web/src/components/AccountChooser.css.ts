import { style, styleVariants } from '@vanilla-extract/css'
import { tokens } from '../styles.css'

export const note = style({
  color: tokens.color.muted,
  fontSize: '14px',
  lineHeight: 1.5,
})

export const row = style({
  alignItems: 'center',
  background: 'transparent',
  border: 0,
  borderBottom: `1px solid ${tokens.color.border}`,
  color: tokens.color.text,
  display: 'flex',
  gap: tokens.space.small,
  padding: '16px 12px',
  textAlign: 'left',
  width: '100%',
  selectors: {
    '&:hover:not(:disabled)': {
      background: tokens.color.background,
    },
  },
})

const avatar = style({
  alignItems: 'center',
  borderRadius: '50%',
  display: 'flex',
  flexShrink: 0,
  fontSize: '18px',
  height: '36px',
  justifyContent: 'center',
  width: '36px',
})

export const avatars = styleVariants({
  blue: [
    avatar,
    {
      background: tokens.color.blueLight,
      color: tokens.color.blue,
    },
  ],
  green: [
    avatar,
    {
      background: tokens.color.greenLight,
      color: tokens.color.green,
    },
  ],
  red: [
    avatar,
    {
      background: tokens.color.redLight,
      color: tokens.color.red,
    },
  ],
  yellow: [
    avatar,
    {
      background: tokens.color.yellowLight,
      color: tokens.color.text,
    },
  ],
})

export const identity = style({
  display: 'grid',
  gap: '5px',
  minWidth: 0,
})

export const name = style({
  fontSize: '15px',
  fontWeight: 600,
  overflowWrap: 'anywhere',
})

export const email = style({
  color: tokens.color.muted,
  fontSize: '13px',
  overflowWrap: 'anywhere',
})

export const addIcon = style([
  avatar,
  {
    color: tokens.color.muted,
    fontSize: '24px',
  },
])
