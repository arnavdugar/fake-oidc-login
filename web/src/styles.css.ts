import { createGlobalTheme, globalStyle } from '@vanilla-extract/css'

export const tokens = createGlobalTheme(':root', {
  color: {
    background: '#f0f4f9',
    blue: '#0b57d0',
    blueLight: '#e8f0fe',
    border: '#dce1e7',
    green: '#137333',
    greenLight: '#e6f4ea',
    muted: '#5f6368',
    red: '#a50e0e',
    redLight: '#fce8e6',
    surface: '#ffffff',
    text: '#1f1f1f',
    yellowLight: '#fef7e0',
  },
  font: {
    body: 'Arial, Helvetica, sans-serif',
  },
  radius: {
    card: '28px',
    control: '8px',
    pill: '100px',
  },
  space: {
    large: '32px',
    medium: '24px',
    small: '16px',
    tiny: '8px',
  },
})

export const mobile = 'screen and (max-width: 720px)'

globalStyle('*', { boxSizing: 'border-box' })
globalStyle('body', {
  background: tokens.color.background,
  color: tokens.color.text,
  fontFamily: tokens.font.body,
  margin: 0,
})
globalStyle('button, input', { font: 'inherit' })
globalStyle('button', { cursor: 'pointer' })
globalStyle('button:disabled', { cursor: 'default', opacity: 0.55 })
globalStyle('button:focus-visible, input:focus-visible, a:focus-visible', {
  outline: `3px solid ${tokens.color.blue}`,
  outlineOffset: '3px',
})
