# Fake OIDC Login

A passwordless OIDC provider for local development. Choose an existing account or enter a name and
email, then complete the application's normal login callback. The provider reads an existing
PostgreSQL database and never creates or updates records. The relying application owns persistence.

The Go binary embeds the Preact UI. There is one server, one port, and no separate UI process. This
project starts from boilerplate's initial commit, `7372ca94`.

## Run

Install Go 1.25 or later, Node.js 24, and pnpm 11.15.1. Run commands from the repository root:

```sh
pnpm --dir web install --frozen-lockfile
./scripts/build.sh
REDIRECT_URIS=http://localhost:3000/callback ./bin/fake-oidc-login
```

Start login from an OIDC client configured with these values:

- Issuer: `http://localhost:8080`
- Discovery: `http://localhost:8080/.well-known/openid-configuration`
- Client ID: `fake-oidc-client`
- Client secret: `fake-oidc-secret`
- Scopes: `openid email profile`

Opening the provider directly lists accounts. Choosing or adding an account requires an active
client authorization request. Without `DATABASE_URL`, the account list is empty and new identities
can still complete the flow. No account is retained by the provider after sign-in.

To inspect the chooser without an OIDC client, open this development authorization request. Its
callback requires an application running on port 3000 to complete login:

```text
http://localhost:8080/o/oauth2/v2/auth?client_id=fake-oidc-client&redirect_uri=http%3A%2F%2Flocalhost%3A3000%2Fcallback&response_type=code&scope=openid%20email%20profile&state=example-state&nonce=example-nonce
```

## Configuration

Configure the process with environment variables. `REDIRECT_URIS` is required. Callback matching is
exact, including scheme, port, path, and query string; fragments and OAuth response parameters are
not allowed in registered URLs.

| Variable                    | Default                 | Purpose                                                                   |
| --------------------------- | ----------------------- | ------------------------------------------------------------------------- |
| `CLIENT_ID`                 | `fake-oidc-client`      | One registered application's client ID.                                   |
| `CLIENT_NAME`               | `Local app`             | Application name shown in the chooser.                                    |
| `CLIENT_SECRET`             | `fake-oidc-secret`      | Development client secret for token exchange.                             |
| `DATABASE_EMAIL_COLUMN`     | Unset                   | Optional email column; missing emails use stable `@users.test` addresses. |
| `DATABASE_NAME_COLUMN`      | `name`                  | Display name column.                                                      |
| `DATABASE_SCHEMA`           | `public`                | Schema containing the user table.                                         |
| `DATABASE_SUBJECT_COLUMN`   | `id`                    | Stable OIDC subject column.                                               |
| `DATABASE_TABLE`            | `users`                 | Existing user table.                                                      |
| `DATABASE_URL`              | Unset                   | PostgreSQL connection URL, including database name.                       |
| `DATABASE_USERS_QUERY_FILE` | Unset                   | Optional SQL file replacing the table and column mapping.                 |
| `ISSUER_URL`                | `http://localhost:8080` | Browser-accessible HTTP(S) origin, without a trailing slash.              |
| `LISTEN_ADDRESS`            | `:8080`                 | HTTP bind address.                                                        |
| `REDIRECT_URIS`             | Required                | Comma-separated exact callback URLs.                                      |

The default query lists every row in the configured table, sorted by name and subject. Configured
identifiers are quoted and selected values are cast to text. Subjects must be unique, nonempty, and
at most 255 bytes. Null names fall back to email. Nothing is written or migrated at startup.

For joins or filtering, mount a SQL file returning three text-compatible columns in this order:
subject, name, email. An optional fourth column supplies an identifier to display below the name
instead of email; it does not affect OIDC claims or account selection. Name, email, and identifier
may be null. Use `NULL::text` if there is no email field. All queries run in read-only transactions,
with a five-second statement timeout and up to four connections. Use a database role with only
`SELECT` permission as well.

```sh
DATABASE_URL=postgres://reader:password@localhost:5432/app?sslmode=disable \
DATABASE_TABLE=accounts \
DATABASE_SUBJECT_COLUMN=oidc_subject \
DATABASE_NAME_COLUMN=display_name \
DATABASE_EMAIL_COLUMN=email \
REDIRECT_URIS=http://localhost:3000/callback \
./bin/fake-oidc-login
```

A failed database read produces a retryable error in the chooser. Submitting an account rereads the
source, so deleted accounts cannot be selected from stale UI data. Canceling does not require the
database.

## OIDC behavior

The provider implements Google's server authorization-code request and response shape, based on
[Google's OIDC documentation](https://developers.google.com/identity/openid-connect/openid-connect).
It uses its own configured issuer. Configure the client to trust that issuer and use these
endpoints; it does not impersonate Google's issuer or replace Google's browser SDK.

| Endpoint                            | Method      | Purpose                                             |
| ----------------------------------- | ----------- | --------------------------------------------------- |
| `/.well-known/openid-configuration` | GET         | Discovery metadata.                                 |
| `/o/oauth2/v2/auth`                 | GET         | Authorization and account chooser.                  |
| `/oauth2/v3/certs`                  | GET         | RSA public signing key as JWKS.                     |
| `/token`                            | POST        | Form-encoded code exchange with client credentials. |
| `/v1/userinfo`                      | GET or POST | User claims with a bearer access token.             |

Authorization preserves `state` and `nonce`, supports `openid`, `email`, and `profile`, and
optionally accepts PKCE with `code_challenge_method=S256`. Token exchange accepts
`client_secret_post` or `client_secret_basic`. Codes expire after one minute and can be exchanged
once, using the original callback URL and PKCE verifier. Browser requests expire after ten minutes
and are bound to an HTTP-only cookie. Cancel returns `error=access_denied` and the original state to
the client.

ID tokens use RS256 and contain `iss`, `aud`, `sub`, `iat`, `exp`, `at_hash`, and the requested
nonce. The email scope adds `email` and `email_verified: true`; the profile scope adds `name`.
Access tokens and ID tokens expire after one hour. The chooser always asks for an account;
`prompt=none` returns `login_required`. Implicit flow, refresh tokens, real passwords, external
Google APIs, and logout sessions are not implemented. `access_type=offline` does not issue a refresh
token.

New identities use a stable `dev-` subject derived from the trimmed, lowercased email. Reusing an
existing email or previously persisted development subject selects that saved identity. An account
appears on later visits only if the application's callback persists it in the configured source. The
signing key and all requests and grants are in memory. Restarting drops outstanding requests and
access tokens and rotates the signing key. Run one instance.

This service deliberately allows choosing any listed user and marks development emails verified
without verification. Expose it only in a development environment.

## Container

Build the UI and Go binary together in a container. The final image contains the binary and CA
certificates, runs as a non-root user, and needs no writable filesystem:

```sh
docker build -t fake-oidc-login .
docker run --rm -p 127.0.0.1:8080:8080 \
  -e REDIRECT_URIS=http://localhost:3000/callback \
  fake-oidc-login
```

Pass `DATABASE_URL` and the column mappings to connect to an existing database. Mount a custom query
file read-only and set `DATABASE_USERS_QUERY_FILE` to its path inside the container. Set
`ISSUER_URL` to the origin reachable by the browser. The relying app must also be able to reach the
discovery, token, and JWKS endpoints; `localhost` inside its container names that container, not the
provider.

[CI](.github/workflows/ci.yml) builds and tests the project and container. After this repository is
published to GitHub, successful pushes to `main` publish Linux amd64 and arm64 images to
`ghcr.io/<owner>/<repository>` with `latest` and full commit-SHA tags, using `GITHUB_TOKEN`. Pull
requests build the image without publishing it. The workflow has not been run on GitHub yet.

## Client integration

Configure the application's OIDC client with this provider's issuer, endpoints, client credentials,
and registered callback. Use the same authorization-code flow and callback persistence as any other
OIDC provider. The provider works with any PostgreSQL database and user table through its column
mapping or a custom query.

The subject returned for an existing user must match the identifier the application resolves in its
OIDC callback. Use `DATABASE_SUBJECT_COLUMN` for a subject stored on the user record. If identity
subjects live in another table, use `DATABASE_USERS_QUERY_FILE` to join them to the profile table.
[examples/users.sql](examples/users.sql) shows the three-column query shape. The provider cannot
link unassociated fixtures or create identities in the database; the application owns that work.

New accounts appear in the chooser once the callback saves them in the configured source. Set
`CLIENT_NAME` to the application name and register its callback URLs with `REDIRECT_URIS`. GitHub
publication and configuration in a consuming application's local stack are subsequent work.

## Development

Regenerate the chooser's API after editing `schema/openapi.yaml`:

```sh
./scripts/generate.sh
```

Build the UI before running Go checks because the binary embeds its output:

```sh
./scripts/build.sh
(cd api && go test -race ./...)
(cd api && go vet ./...)
pnpm --dir web format:check
```

Run integration tests against a disposable PostgreSQL database:

```sh
TEST_DATABASE_URL=postgres://postgres:password@localhost:5432/test?sslmode=disable \
./scripts/test-db.sh -race
```

Tests create and clean up isolated schemas and verify that database writes are rejected even with
privileged connection credentials. No database is needed for ordinary Go tests. The integration tag
requires `TEST_DATABASE_URL`.
