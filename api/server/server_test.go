package server

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"fake-oidc-login/internal/config"
	"fake-oidc-login/server/oidc"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type unavailableUsers struct{}

func (unavailableUsers) ListUsers(context.Context) ([]oidc.User, error) {
	return nil, errors.New("private database detail")
}

func TestEmbeddedUIAndDatabaseFailure(t *testing.T) {
	handler, err := NewHandler(config.Config{
		ClientID:     "client",
		ClientName:   "Example app",
		ClientSecret: "secret",
		IssuerURL:    "http://localhost:8080",
		RedirectURIs: []string{"http://localhost:3000/callback"},
	}, unavailableUsers{})
	require.NoError(t, err)
	t.Run("binary serves built UI", func(t *testing.T) {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/", nil))
		assert.Equal(t, http.StatusOK, response.Code)
		assert.Contains(t, response.Body.String(), `id="app"`)
		assert.Contains(t, response.Body.String(), "/assets/")
		assert.Equal(t, "no-store", response.Header().Get("Cache-Control"))
	})
	t.Run("source failure is safe and retryable", func(t *testing.T) {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/login", nil))
		assert.Equal(t, http.StatusServiceUnavailable, response.Code)
		assert.JSONEq(t, `{"code":"accounts_unavailable"}`, response.Body.String())
		assert.NotContains(t, response.Body.String(), "private database detail")
	})
	t.Run("invalid request is rejected before loading accounts", func(t *testing.T) {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/api/v1/login?request=unknown", nil))
		assert.Equal(t, http.StatusBadRequest, response.Code)
		assert.JSONEq(t, `{"code":"invalid_request"}`, response.Body.String())
	})
}
