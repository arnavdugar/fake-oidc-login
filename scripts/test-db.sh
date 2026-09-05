#!/bin/sh
set -eu

: "${TEST_DATABASE_URL:?Set TEST_DATABASE_URL to a disposable Postgres database}"
(cd api && go test -tags integration "$@" ./...)
