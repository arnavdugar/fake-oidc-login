# Repository guidance

## Architecture

- `api/` is the Go module. `api/cmd/fake-oidc-login` runs the server.
- `web/` uses Preact, Vite, and vanilla-extract. Vite builds into `api/ui/dist`, which Go embeds and
  serves. Run `./scripts/build.sh` before Go checks on a fresh checkout.
- `schema/openapi.yaml` owns the account chooser API under `/api/v1/`. Run `./scripts/generate.sh`
  after changing it; do not edit generated Go or TypeScript files.
- `api/server/oidc` owns the standard OIDC routes, browser authorization requests, and token grants.
  These protocol routes use `net/http` directly to preserve OAuth form and redirect semantics.
- `UsersStore` reads a caller-owned PostgreSQL schema. Configurable identifiers and an optional
  administrator-supplied query require runtime SQL here; sqlc and migrations do not apply. Keep SQL
  in this store or example query files, never in HTTP handlers.
- The provider never writes users. The relying app creates them through its normal OIDC callback.
  Never add a provider-side insert, migration, seed runner, or persistent account cache.
- New subjects are external identifiers derived from normalized email addresses. Authorization
  requests, grants, access tokens, and the signing key exist only in memory.

## Conventions

- Keep changes focused and implementations simple. Alphabetize items when order has no meaning.
- Use `log/slog` with structured fields. Never log credentials, codes, tokens, or user profiles.
- Use keyed Go struct literals with one field per line. Keep a 100-character ruler as a guide.
- Define narrow dependency interfaces where they are consumed. Inline one-use helpers unless they
  create a real abstraction boundary.
- Put components in `web/src/components` and hooks in `web/src/hooks`. Import from `preact` and
  `preact/hooks` directly.
- Put component styles in adjacent `.css.ts` files. Keep `globalStyle` in `web/src/styles.css.ts`
  and reuse its tokens. Avoid inline styles except for runtime values.
- Use the generated SDK and Preact Query options for chooser requests. Use native forms for the
  browser authorization flow. Preserve loading, error, and empty states.
- Use semantic controls, stable user subject keys, visible keyboard focus, and useful labels.
- Keep database transactions explicitly read-only, quote configured identifiers, and reject
  ambiguous or empty subjects. Do not accept SQL from browser requests.
- Match registered callback URLs exactly. Keep authorization codes single-use, browser requests
  bound to their cookie, and tokens scoped to their client, callback, nonce, and PKCE challenge.
- Do not commit real credentials or personal data. This passwordless provider is for development.

## Verification

Run scripts from the repository root. Use Go 1.25 or later and the pnpm version in
`web/package.json`.

```sh
pnpm --dir web install --frozen-lockfile
./scripts/build.sh
(cd api && go test -race ./...)
(cd api && go vet ./...)
pnpm --dir web format:check
```

Run database tests against a disposable Postgres database. Tests create and remove their own
schemas; missing `TEST_DATABASE_URL` fails when the integration tag is enabled.

```sh
TEST_DATABASE_URL=postgres://... ./scripts/test-db.sh -race
```

Use ordinary Go tests, `httptest`, and small fakes. Use testify `require` for prerequisites and
`assert` for independent results; call `require` only from the test goroutine. Keep database tests
in `*_integration_test.go` with the `integration` build tag.

Format Markdown and YAML with Prettier using `web/.prettierrc.json` and `--prose-wrap always`.
