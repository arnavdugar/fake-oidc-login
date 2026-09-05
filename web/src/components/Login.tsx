import { useQuery } from '@tanstack/preact-query'
import { useState } from 'preact/hooks'
import { getLoginOptions } from '../client/@tanstack/preact-query.gen'
import type { ErrorCode } from '../client/types.gen'
import { AccountChooser } from './AccountChooser'
import { Button } from './Button'
import { CreateAccount } from './CreateAccount'
import { LoginFrame } from './LoginFrame'

import * as styles from './Login.css'

const errorMessages: Record<ErrorCode, string> = {
  accounts_unavailable: 'Accounts are unavailable. Check the database configuration and try again.',
  invalid_request: 'This sign-in has expired. Start again from your app.',
}

export function Login() {
  const [creating, setCreating] = useState(false)
  const request = new URLSearchParams(window.location.search).get('request') ?? ''
  const login = useQuery(getLoginOptions({ query: request ? { request } : undefined }))
  const data = login.data

  return (
    <LoginFrame
      heading={creating ? 'Create an account' : 'Choose an account'}
      description={
        data &&
        !data.can_login && (
          <p className={styles.note}>Start sign-in from your app to choose or add an account.</p>
        )
      }
      footer={
        data?.can_login && (
          <form action="/authorize" method="post">
            <input name="request" type="hidden" value={request} />
            <Button name="action" type="submit" value="cancel">
              Cancel
            </Button>
          </form>
        )
      }
    >
      {login.isPending && <p role="status">Loading accounts…</p>}
      {login.isError && (
        <div role="alert">
          <p className={styles.error}>
            {(login.error?.code && errorMessages[login.error.code]) ?? 'Could not load accounts.'}
          </p>
          <Button
            onClick={async () => {
              await login.refetch()
            }}
            type="button"
          >
            Try again
          </Button>
        </div>
      )}
      {data &&
        (creating ? (
          <CreateAccount
            canLogin={data.can_login}
            onBack={() => setCreating(false)}
            request={request}
          />
        ) : (
          <AccountChooser
            canLogin={data.can_login}
            onCreate={() => setCreating(true)}
            request={request}
            users={data.users}
          />
        ))}
    </LoginFrame>
  )
}
