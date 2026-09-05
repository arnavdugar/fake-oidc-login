// Package oidc implements a deliberately passwordless provider for local development.
package oidc

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"fake-oidc-login/internal/config"
)

const (
	browserCookie = "fake_oidc_browser"
	loginLifetime = 10 * time.Minute
	tokenLifetime = time.Hour
)

type authorization struct {
	BrowserID   string
	Challenge   string
	ExpiresAt   time.Time
	Nonce       string
	RedirectURI string
	Scope       string
	State       string
}

type grant struct {
	Authorization authorization
	ExpiresAt     time.Time
	User          User
}

type accessGrant struct {
	ExpiresAt time.Time
	Scope     string
	User      User
}

type OIDCHandler struct {
	accessTokens map[string]accessGrant
	codes        map[string]grant
	config       config.Config
	key          *rsa.PrivateKey
	mu           sync.Mutex
	now          func() time.Time
	requests     map[string]authorization
	users        UserSource
}

func NewHandler(c config.Config, users UserSource) (*OIDCHandler, error) {
	if err := c.Validate(); err != nil {
		return nil, err
	}
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, err
	}
	return &OIDCHandler{
		accessTokens: map[string]accessGrant{},
		codes:        map[string]grant{},
		config:       c,
		key:          key,
		now:          time.Now,
		requests:     map[string]authorization{},
		users:        users,
	}, nil
}

func (h *OIDCHandler) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /.well-known/openid-configuration", h.discovery)
	mux.HandleFunc("GET /o/oauth2/v2/auth", h.authorize)
	mux.HandleFunc("GET /oauth2/v3/certs", h.certs)
	mux.HandleFunc("POST /authorize", h.finish)
	mux.HandleFunc("POST /token", h.token)
	mux.HandleFunc("GET /v1/userinfo", h.userinfo)
	mux.HandleFunc("POST /v1/userinfo", h.userinfo)
}

// Cleanup runs before requests so expired state never accumulates across active use.
func (h *OIDCHandler) Cleanup() {
	h.mu.Lock()
	defer h.mu.Unlock()
	now := h.now()
	for key, value := range h.requests {
		if !now.Before(value.ExpiresAt) {
			delete(h.requests, key)
		}
	}
	for key, value := range h.codes {
		if !now.Before(value.ExpiresAt) {
			delete(h.codes, key)
		}
	}
	for key, value := range h.accessTokens {
		if !now.Before(value.ExpiresAt) {
			delete(h.accessTokens, key)
		}
	}
}

func randomToken() string {
	var value [32]byte
	// crypto/rand.Read cannot fail on supported Go versions.
	_, _ = rand.Read(value[:])
	return base64.RawURLEncoding.EncodeToString(value[:])
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(value); err != nil {
		slog.Error("write response", "error", err)
	}
}

func oauthError(w http.ResponseWriter, status int, code, description string) {
	writeJSON(w, status, map[string]string{
		"error":             code,
		"error_description": description,
	})
}
