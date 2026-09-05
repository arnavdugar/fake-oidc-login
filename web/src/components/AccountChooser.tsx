import type { User } from '../client/types.gen'

import * as styles from './AccountChooser.css'

interface AccountChooserProps {
  canLogin: boolean
  onCreate: () => void
  request: string
  users: User[]
}

const avatarColors = ['blue', 'green', 'yellow', 'red'] as const

export function AccountChooser({ canLogin, onCreate, request, users }: AccountChooserProps) {
  return (
    <>
      {users.length === 0 && (
        <p className={styles.note}>No accounts yet. Create one to get started.</p>
      )}
      <form action="/authorize" method="post">
        <input name="request" type="hidden" value={request} />
        <input name="action" type="hidden" value="select" />
        {users.map((user) => (
          <button
            className={styles.row}
            disabled={!canLogin}
            key={user.subject}
            name="subject"
            type="submit"
            value={user.subject}
          >
            <span
              aria-hidden="true"
              className={
                styles.avatars[
                  avatarColors[user.subject.charCodeAt(0) % avatarColors.length] ?? 'blue'
                ]
              }
            >
              {user.name.slice(0, 1).toUpperCase()}
            </span>
            <span className={styles.identity}>
              <span className={styles.name}>{user.name}</span>
              <span className={styles.email}>{user.identifier || user.email}</span>
            </span>
          </button>
        ))}
      </form>
      <button className={styles.row} disabled={!canLogin} onClick={onCreate} type="button">
        <span className={styles.addIcon} aria-hidden="true">
          +
        </span>
        <span className={styles.name}>Create a new account</span>
      </button>
    </>
  )
}
