FROM --platform=$BUILDPLATFORM node:24-alpine AS web
WORKDIR /src/web
RUN npm install --global pnpm@11.15.1
COPY web/package.json web/pnpm-lock.yaml web/pnpm-workspace.yaml ./
RUN pnpm install --frozen-lockfile
COPY web/ ./
RUN pnpm build

FROM --platform=$BUILDPLATFORM golang:1.25-alpine AS api
WORKDIR /src/api
COPY api/go.mod api/go.sum ./
RUN go mod download
COPY api/ ./
COPY --from=web /src/api/ui/dist ./ui/dist
ARG TARGETARCH
ARG TARGETOS
RUN CGO_ENABLED=0 GOARCH=$TARGETARCH GOOS=$TARGETOS \
    go build -trimpath -ldflags="-s -w" -o /fake-oidc-login ./cmd/fake-oidc-login

FROM scratch
COPY --from=api /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=api /fake-oidc-login /fake-oidc-login
USER 65532:65532
EXPOSE 8080
ENTRYPOINT ["/fake-oidc-login"]
