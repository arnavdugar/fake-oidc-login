import type { JSX } from 'preact'

import * as styles from './Button.css'

interface ButtonProps extends JSX.ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: keyof typeof styles.variants
}

export function Button({ className, variant = 'text', ...props }: ButtonProps) {
  return (
    <button
      {...props}
      className={[styles.variants[variant], className].filter(Boolean).join(' ')}
    />
  )
}
