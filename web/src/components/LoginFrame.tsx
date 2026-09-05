import type { ComponentChildren } from 'preact'
import { useId } from 'preact/hooks'

import * as styles from './LoginFrame.css'

interface LoginFrameProps {
  children: ComponentChildren
  description?: ComponentChildren
  footer?: ComponentChildren
  heading: string
}

export function LoginFrame({ children, description, footer, heading }: LoginFrameProps) {
  const headingId = useId()

  return (
    <main className={styles.page}>
      <div className={styles.container}>
        <section className={styles.card} aria-labelledby={headingId}>
          <div>
            <h1 id={headingId} className={styles.heading}>
              {heading}
            </h1>
            {description}
          </div>
          <div className={styles.content}>{children}</div>
        </section>
        <footer className={styles.footer}>{footer}</footer>
      </div>
    </main>
  )
}
