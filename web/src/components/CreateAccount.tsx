import { Button } from './Button'

import * as styles from './CreateAccount.css'

interface CreateAccountProps {
  canLogin: boolean
  onBack: () => void
  request: string
}

export function CreateAccount({ canLogin, onBack, request }: CreateAccountProps) {
  return (
    <form action="/authorize" method="post">
      <input name="request" type="hidden" value={request} />
      <input name="action" type="hidden" value="create" />
      <label className={styles.field}>
        <span className={styles.label}>Full name</span>
        <input autoComplete="name" className={styles.input} maxLength={200} name="name" required />
      </label>
      <label className={styles.field}>
        <span className={styles.label}>Email address</span>
        <input
          autoComplete="email"
          className={styles.input}
          maxLength={254}
          name="email"
          required
          type="email"
        />
      </label>
      <div className={styles.actions}>
        <Button onClick={onBack} type="button">
          Back
        </Button>
        <Button variant="primary" disabled={!canLogin} type="submit">
          Continue
        </Button>
      </div>
    </form>
  )
}
