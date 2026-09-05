#!/bin/sh
set -eu

(cd web && pnpm build)
mkdir -p bin
(cd api && go build -trimpath -o ../bin/fake-oidc-login ./cmd/fake-oidc-login)
