package oidc

import (
	"context"
	"crypto"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"math/big"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"fake-oidc-login/internal/config"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeUsers struct {
	err   error
	users []User
}

func (f *fakeUsers) ListUsers(context.Context) ([]User, error) { return f.users, f.err }

func testProvider(t *testing.T) (*OIDCHandler, *fakeUsers, http.Handler) {
	t.Helper()
	users := &fakeUsers{users: []User{{
		Email:   "alex@example.test",
		Name:    "Alex Example",
		Subject: "saved-google-subject",
	}}}
	h, err := NewHandler(config.Config{
		ClientID:     "client",
		ClientName:   "Example app",
		ClientSecret: "secret",
		IssuerURL:    "http://localhost:8080",
		RedirectURIs: []string{"http://localhost:3000/callback?existing=value"},
	}, users)
	require.NoError(t, err)
	mux := http.NewServeMux()
	h.Register(mux)
	return h, users, mux
}

type browserLogin struct {
	cookie  *http.Cookie
	request string
}

func startLogin(t *testing.T, handler http.Handler, verifier string) browserLogin {
	t.Helper()
	query := url.Values{
		"client_id":     {"client"},
		"nonce":         {"test-nonce"},
		"redirect_uri":  {"http://localhost:3000/callback?existing=value"},
		"response_type": {"code"},
		"scope":         {"openid email profile"},
		"state":         {"test-state"},
	}
	if verifier != "" {
		digest := sha256.Sum256([]byte(verifier))
		query.Set("code_challenge", base64.RawURLEncoding.EncodeToString(digest[:]))
		query.Set("code_challenge_method", "S256")
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/o/oauth2/v2/auth?"+query.Encode(), nil))
	require.Equal(t, http.StatusFound, response.Code)
	location, err := response.Result().Location()
	require.NoError(t, err)
	require.Len(t, response.Result().Cookies(), 1)
	return browserLogin{
		cookie:  response.Result().Cookies()[0],
		request: location.Query().Get("request"),
	}
}

func submit(t *testing.T, handler http.Handler, login browserLogin, values url.Values) *httptest.ResponseRecorder {
	t.Helper()
	values.Set("request", login.request)
	request := httptest.NewRequest(http.MethodPost, "/authorize", strings.NewReader(values.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if login.cookie != nil {
		request.AddCookie(login.cookie)
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}

func exchange(handler http.Handler, code, verifier, redirect string) *httptest.ResponseRecorder {
	values := url.Values{
		"client_id":     {"client"},
		"client_secret": {"secret"},
		"code":          {code},
		"code_verifier": {verifier},
		"grant_type":    {"authorization_code"},
		"redirect_uri":  {redirect},
	}
	request := httptest.NewRequest(http.MethodPost, "/token", strings.NewReader(values.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}

func TestExistingAccountCodeFlow(t *testing.T) {
	_, _, handler := testProvider(t)
	login := startLogin(t, handler, "")
	response := submit(t, handler, login, url.Values{
		"action": {"select"}, "subject": {"saved-google-subject"},
	})
	require.Equal(t, http.StatusSeeOther, response.Code)
	callback, err := response.Result().Location()
	require.NoError(t, err)
	assert.Equal(t, "test-state", callback.Query().Get("state"))
	assert.Equal(t, "value", callback.Query().Get("existing"))
	code := callback.Query().Get("code")
	require.NotEmpty(t, code)
	tokenResponse := exchange(handler, code, "", "http://localhost:3000/callback?existing=value")
	require.Equal(t, http.StatusOK, tokenResponse.Code)
	var tokens struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
		IDToken     string `json:"id_token"`
		TokenType   string `json:"token_type"`
	}
	require.NoError(t, json.Unmarshal(tokenResponse.Body.Bytes(), &tokens))
	assert.Equal(t, 3600, tokens.ExpiresIn)
	assert.Equal(t, "Bearer", tokens.TokenType)
	parts := strings.Split(tokens.IDToken, ".")
	require.Len(t, parts, 3)
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	require.NoError(t, err)
	var claims map[string]any
	require.NoError(t, json.Unmarshal(payload, &claims))
	assert.Equal(t, "client", claims["aud"])
	assert.Equal(t, "http://localhost:8080", claims["iss"])
	assert.Equal(t, "test-nonce", claims["nonce"])
	assert.Equal(t, "saved-google-subject", claims["sub"])
	assert.Equal(t, "Alex Example", claims["name"])
	assert.Equal(t, "alex@example.test", claims["email"])
	assert.Equal(t, true, claims["email_verified"])
	assert.InDelta(t, time.Now().Add(time.Hour).Unix(), claims["exp"], 2)

	certs := httptest.NewRecorder()
	handler.ServeHTTP(certs, httptest.NewRequest(http.MethodGet, "/oauth2/v3/certs", nil))
	var jwks struct{ Keys []map[string]string }
	require.NoError(t, json.Unmarshal(certs.Body.Bytes(), &jwks))
	require.Len(t, jwks.Keys, 1)
	n, err := base64.RawURLEncoding.DecodeString(jwks.Keys[0]["n"])
	require.NoError(t, err)
	e, err := base64.RawURLEncoding.DecodeString(jwks.Keys[0]["e"])
	require.NoError(t, err)
	signature, err := base64.RawURLEncoding.DecodeString(parts[2])
	require.NoError(t, err)
	digest := sha256.Sum256([]byte(parts[0] + "." + parts[1]))
	publicKey := &rsa.PublicKey{
		E: int(new(big.Int).SetBytes(e).Int64()),
		N: new(big.Int).SetBytes(n),
	}
	assert.NoError(t, rsa.VerifyPKCS1v15(publicKey, crypto.SHA256, digest[:], signature))

	userinfo := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/v1/userinfo", nil)
	request.Header.Set("Authorization", "Bearer "+tokens.AccessToken)
	handler.ServeHTTP(userinfo, request)
	assert.Equal(t, http.StatusOK, userinfo.Code)
	var profile User
	require.NoError(t, json.Unmarshal(userinfo.Body.Bytes(), &profile))
	assert.Equal(t, "saved-google-subject", profile.Subject)
	assert.Equal(t, "Alex Example", profile.Name)

	replay := exchange(handler, code, "", "http://localhost:3000/callback?existing=value")
	assert.Equal(t, http.StatusBadRequest, replay.Code)
	assert.Contains(t, replay.Body.String(), "invalid_grant")
}

func TestPKCEAndCallbackBinding(t *testing.T) {
	_, _, handler := testProvider(t)
	verifier := strings.Repeat("a", 43)
	login := startLogin(t, handler, verifier)
	response := submit(t, handler, login, url.Values{
		"action": {"select"}, "subject": {"saved-google-subject"},
	})
	require.Equal(t, http.StatusSeeOther, response.Code)
	callback, err := response.Result().Location()
	require.NoError(t, err)
	code := callback.Query().Get("code")
	wrongVerifier := exchange(handler, code, strings.Repeat("b", 43), "http://localhost:3000/callback?existing=value")
	assert.Equal(t, http.StatusBadRequest, wrongVerifier.Code)
	wrongCallback := exchange(handler, code, verifier, "http://localhost:3000/callback")
	assert.Equal(t, http.StatusBadRequest, wrongCallback.Code)
	valid := exchange(handler, code, verifier, "http://localhost:3000/callback?existing=value")
	assert.Equal(t, http.StatusOK, valid.Code)
}

func TestBrowserBindingAndFormReplay(t *testing.T) {
	h, _, handler := testProvider(t)
	login := startLogin(t, handler, "")
	_, ok := h.LoginContext(login.request, "another-browser")
	assert.False(t, ok)
	values := url.Values{"action": {"select"}, "subject": {"saved-google-subject"}}
	missingCookie := submit(t, handler, browserLogin{request: login.request}, values)
	assert.Equal(t, http.StatusBadRequest, missingCookie.Code)
	valid := submit(t, handler, login, values)
	assert.Equal(t, http.StatusSeeOther, valid.Code)
	replayed := submit(t, handler, login, values)
	assert.Equal(t, http.StatusBadRequest, replayed.Code)
}

func TestNewAccountOnlyIssuesIdentity(t *testing.T) {
	h, users, handler := testProvider(t)
	login := startLogin(t, handler, "")
	response := submit(t, handler, login, url.Values{
		"action": {"create"}, "email": {"  New@Example.test "}, "name": {"New User"},
	})
	require.Equal(t, http.StatusSeeOther, response.Code)
	callback, err := response.Result().Location()
	require.NoError(t, err)
	issued := h.codes[callback.Query().Get("code")].User
	assert.Equal(t, "new@example.test", issued.Email)
	assert.Equal(t, "New User", issued.Name)
	assert.Len(t, users.users, 1)
	assert.Len(t, issued.Subject, 68)

	second := startLogin(t, handler, "")
	retry := submit(t, handler, second, url.Values{
		"action": {"create"}, "email": {"new@example.test"}, "name": {"New User"},
	})
	retryCallback, err := retry.Result().Location()
	require.NoError(t, err)
	assert.Equal(t, issued.Subject, h.codes[retryCallback.Query().Get("code")].User.Subject)
}

func TestCreatingKnownEmailUsesSavedSubject(t *testing.T) {
	h, _, handler := testProvider(t)
	login := startLogin(t, handler, "")
	response := submit(t, handler, login, url.Values{
		"action": {"create"}, "email": {"alex@example.test"}, "name": {"Different name"},
	})
	require.Equal(t, http.StatusSeeOther, response.Code)
	callback, err := response.Result().Location()
	require.NoError(t, err)
	assert.Equal(t, "saved-google-subject", h.codes[callback.Query().Get("code")].User.Subject)
}

func TestCancelStillWorksWhenDatabaseFails(t *testing.T) {
	_, users, handler := testProvider(t)
	login := startLogin(t, handler, "")
	users.err = errors.New("database unavailable")
	response := submit(t, handler, login, url.Values{"action": {"cancel"}})
	callback, err := response.Result().Location()
	require.NoError(t, err)
	assert.Equal(t, "access_denied", callback.Query().Get("error"))
	assert.Equal(t, "test-state", callback.Query().Get("state"))
}

func TestAuthorizationValidation(t *testing.T) {
	_, _, handler := testProvider(t)
	for _, test := range []struct {
		field     string
		name      string
		redirects bool
		value     string
		wantError string
	}{
		{field: "redirect_uri", name: "unregistered callback", value: "https://evil.test/", wantError: "invalid_request"},
		{field: "client_id", name: "unknown client", value: "other", wantError: "invalid_request"},
		{field: "scope", name: "missing openid", redirects: true, value: "email", wantError: "invalid_scope"},
		{field: "response_type", name: "implicit flow", redirects: true, value: "token", wantError: "unsupported_response_type"},
		{field: "prompt", name: "silent login", redirects: true, value: "none", wantError: "login_required"},
		{field: "code_challenge", name: "invalid PKCE", redirects: true, value: "invalid", wantError: "invalid_request"},
	} {
		t.Run(test.name, func(t *testing.T) {
			query := url.Values{
				"client_id": {"client"}, "redirect_uri": {"http://localhost:3000/callback?existing=value"},
				"response_type": {"code"}, "scope": {"openid"}, "state": {"state"},
			}
			query.Set(test.field, test.value)
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/o/oauth2/v2/auth?"+query.Encode(), nil))
			if test.redirects {
				assert.Equal(t, http.StatusSeeOther, response.Code)
				callback, err := response.Result().Location()
				require.NoError(t, err)
				assert.Equal(t, "state", callback.Query().Get("state"))
				assert.Equal(t, test.wantError, callback.Query().Get("error"))
			} else {
				assert.Equal(t, http.StatusBadRequest, response.Code)
				assert.Empty(t, response.Header().Get("Location"))
				assert.Contains(t, response.Body.String(), test.wantError)
			}
		})
	}
}

func TestExpiredRequestAndCode(t *testing.T) {
	h, _, handler := testProvider(t)
	now := time.Now()
	h.now = func() time.Time { return now }
	login := startLogin(t, handler, "")
	now = now.Add(loginLifetime)
	response := submit(t, handler, login, url.Values{"action": {"cancel"}})
	assert.Equal(t, http.StatusBadRequest, response.Code)
	h.Cleanup()
	assert.Empty(t, h.requests)
	login = startLogin(t, handler, "")
	response = submit(t, handler, login, url.Values{
		"action": {"select"}, "subject": {"saved-google-subject"},
	})
	callback, err := response.Result().Location()
	require.NoError(t, err)
	now = now.Add(time.Minute)
	tokens := exchange(handler, callback.Query().Get("code"), "", "http://localhost:3000/callback?existing=value")
	assert.Equal(t, http.StatusBadRequest, tokens.Code)
	h.Cleanup()
	assert.Empty(t, h.codes)
}

func TestClientAuthenticationAndBasicEncoding(t *testing.T) {
	h, _, handler := testProvider(t)
	h.config.ClientSecret = "secret+with:characters"
	login := startLogin(t, handler, "")
	response := submit(t, handler, login, url.Values{
		"action": {"select"}, "subject": {"saved-google-subject"},
	})
	callback, err := response.Result().Location()
	require.NoError(t, err)
	code := callback.Query().Get("code")
	wrongCredentials := exchange(handler, code, "", "http://localhost:3000/callback?existing=value")
	assert.Equal(t, http.StatusUnauthorized, wrongCredentials.Code)
	assert.Contains(t, wrongCredentials.Body.String(), "invalid_client")
	values := url.Values{
		"code": {code}, "grant_type": {"authorization_code"},
		"redirect_uri": {"http://localhost:3000/callback?existing=value"},
	}
	request := httptest.NewRequest(http.MethodPost, "/token", strings.NewReader(values.Encode()))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.SetBasicAuth("client", url.QueryEscape(h.config.ClientSecret))
	tokens := httptest.NewRecorder()
	handler.ServeHTTP(tokens, request)
	assert.Equal(t, http.StatusOK, tokens.Code)
}

func TestConcurrentCodeExchangeSucceedsOnce(t *testing.T) {
	_, _, handler := testProvider(t)
	login := startLogin(t, handler, "")
	response := submit(t, handler, login, url.Values{
		"action": {"select"}, "subject": {"saved-google-subject"},
	})
	callback, err := response.Result().Location()
	require.NoError(t, err)
	code := callback.Query().Get("code")
	statuses := make(chan int, 8)
	var group sync.WaitGroup
	for range 8 {
		group.Go(func() {
			statuses <- exchange(handler, code, "", "http://localhost:3000/callback?existing=value").Code
		})
	}
	group.Wait()
	close(statuses)
	succeeded := 0
	for status := range statuses {
		if status == http.StatusOK {
			succeeded++
		} else {
			assert.Equal(t, http.StatusBadRequest, status)
		}
	}
	assert.Equal(t, 1, succeeded)
}

func TestUnknownAccountAndInvalidNewAccount(t *testing.T) {
	_, _, handler := testProvider(t)
	for _, test := range []struct {
		name   string
		values url.Values
	}{
		{name: "unknown saved subject", values: url.Values{"action": {"select"}, "subject": {"not-saved"}}},
		{name: "invalid email", values: url.Values{"action": {"create"}, "email": {"invalid"}, "name": {"Name"}}},
		{name: "missing name", values: url.Values{"action": {"create"}, "email": {"new@example.test"}}},
	} {
		t.Run(test.name, func(t *testing.T) {
			login := startLogin(t, handler, "")
			response := submit(t, handler, login, test.values)
			assert.Equal(t, http.StatusBadRequest, response.Code)
			assert.Empty(t, response.Header().Get("Location"))
		})
	}
}

func TestExpiredAccessToken(t *testing.T) {
	h, _, handler := testProvider(t)
	now := time.Now()
	h.now = func() time.Time { return now }
	login := startLogin(t, handler, "")
	response := submit(t, handler, login, url.Values{
		"action": {"select"}, "subject": {"saved-google-subject"},
	})
	callback, err := response.Result().Location()
	require.NoError(t, err)
	tokens := exchange(handler, callback.Query().Get("code"), "", "http://localhost:3000/callback?existing=value")
	var body map[string]any
	require.NoError(t, json.Unmarshal(tokens.Body.Bytes(), &body))
	now = now.Add(tokenLifetime)
	request := httptest.NewRequest(http.MethodGet, "/v1/userinfo", nil)
	request.Header.Set("Authorization", "Bearer "+body["access_token"].(string))
	userinfo := httptest.NewRecorder()
	handler.ServeHTTP(userinfo, request)
	assert.Equal(t, http.StatusUnauthorized, userinfo.Code)
	h.Cleanup()
	assert.Empty(t, h.accessTokens)
}
