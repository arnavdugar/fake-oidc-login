package server

import (
	"context"
	"log/slog"
	"net/http"
	"net/url"

	"fake-oidc-login/internal/config"
	"fake-oidc-login/openapi"
	"fake-oidc-login/server/login"
	"fake-oidc-login/server/oidc"
	"fake-oidc-login/ui"
	"github.com/ogen-go/ogen/ogenerrors"
)

var _ openapi.Handler = (*login.LoginHandler)(nil)

func NewHandler(c config.Config, users oidc.UserSource) (http.Handler, error) {
	provider, err := oidc.NewHandler(c, users)
	if err != nil {
		return nil, err
	}
	api, err := openapi.NewServer(login.NewHandler(c.ClientName, provider, users),
		openapi.WithErrorHandler(func(ctx context.Context, w http.ResponseWriter, r *http.Request, err error) {
			code := ogenerrors.ErrorCode(err)
			if code >= 500 {
				slog.ErrorContext(ctx, "API request failed", "error", err)
			}
			http.Error(w, http.StatusText(code), code)
		}))
	if err != nil {
		return nil, err
	}
	mux := http.NewServeMux()
	provider.Register(mux)
	mux.Handle("GET /api/", api)
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	mux.Handle("GET /", ui.Handler())
	formPolicy := "form-action 'self'"
	for _, redirect := range c.RedirectURIs {
		target, _ := url.Parse(redirect)
		formPolicy += " " + target.Scheme + "://" + target.Host
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		provider.Cleanup()
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Content-Security-Policy",
			"default-src 'self'; base-uri 'none'; frame-ancestors 'none'; "+formPolicy)
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		mux.ServeHTTP(w, r)
	}), nil
}
