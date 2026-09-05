import { style } from '@vanilla-extract/css'
import { mobile, tokens } from '../styles.css'

export const page = style({
  alignItems: 'center',
  display: 'flex',
  justifyContent: 'center',
  minHeight: '100svh',
  padding: tokens.space.medium,
  '@media': {
    [mobile]: {
      background: tokens.color.surface,
      padding: tokens.space.medium,
    },
  },
})

export const container = style({
  width: '100%',
  maxWidth: '1040px',
})

export const card = style({
  background: tokens.color.surface,
  borderRadius: tokens.radius.card,
  display: 'grid',
  gap: '64px',
  gridTemplateColumns: '1fr 1fr',
  minHeight: '400px',
  padding: '40px',
  '@media': {
    [mobile]: {
      gap: tokens.space.large,
      gridTemplateColumns: '1fr',
      padding: '16px 0',
    },
  },
})

export const heading = style({
  fontSize: '36px',
  fontWeight: 400,
  lineHeight: 1.2,
  margin: '0 0 20px',
})

export const content = style({
  alignSelf: 'center',
  minWidth: 0,
})

export const footer = style({
  padding: '16px 20px',
  '@media': { [mobile]: { padding: '24px 0' } },
})
