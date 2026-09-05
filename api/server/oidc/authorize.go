package oidc

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"log/slog"
	"net/http"
	"net/mail"
	"net/url"
	"strings"
	"time"
)

func (h *OIDCHandler) authorize(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	if len(r.URL.RawQuery) > 8192 {
		oauthError(w, http.StatusBadRequest, "invalid_request", "Authorization request is too large.")
		return
	}
	for _, values := range query {
		if len(values) != 1 {
			oauthError(w, http.StatusBadRequest, "invalid_request", "Repeated parameters are unsupported.")
			return
		}
	}
	redirectURI := query.Get("redirect_uri")
	allowed := false
	for _, registered := range h.config.RedirectURIs {
		allowed = allowed || redirectURI == registered
	}
	if query.Get("client_id") != h.config.ClientID || !allowed {
		oauthError(w, http.StatusBadRequest, "invalid_request", "Unknown client or callback URL.")
		return
	}
	request := authorization{
		Challenge:   query.Get("code_challenge"),
		ExpiresAt:   h.now().Add(loginLifetime),
		Nonce:       query.Get("nonce"),
		RedirectURI: redirectURI,
		Scope:       strings.Join(strings.Fields(query.Get("scope")), " "),
		State:       query.Get("state"),
	}
	if query.Get("response_type") != "code" {
		h.redirect(w, r, request, "error", "unsupported_response_type")
		return
	}
	if query.Get("response_mode") != "" && query.Get("response_mode") != "query" {
		h.redirect(w, r, request, "error", "invalid_request")
		return
	}
	openid := false
	for _, scope := range strings.Fields(request.Scope) {
		if scope != "openid" && scope != "email" && scope != "profile" {
			h.redirect(w, r, request, "error", "invalid_scope")
			return
		}
		openid = openid || scope == "openid"
	}
	if !openid {
		h.redirect(w, r, request, "error", "invalid_scope")
		return
	}
	method := query.Get("code_challenge_method")
	challenge, err := base64.RawURLEncoding.Strict().DecodeString(request.Challenge)
	if (request.Challenge != "" && (method != "S256" || err != nil || len(challenge) != 32)) ||
		(request.Challenge == "" && method != "") {
		h.redirect(w, r, request, "error", "invalid_request")
		return
	}
	for _, prompt := range strings.Fields(query.Get("prompt")) {
		if prompt == "none" {
			h.redirect(w, r, request, "error", "login_required")
			return
		}
		if prompt != "consent" && prompt != "select_account" && prompt != "login" {
			h.redirect(w, r, request, "error", "invalid_request")
			return
		}
	}
	if cookie, err := r.Cookie(browserCookie); err == nil && len(cookie.Value) == 43 {
		request.BrowserID = cookie.Value
	} else {
		request.BrowserID = randomToken()
	}
	http.SetCookie(w, &http.Cookie{
		HttpOnly: true,
		Name:     browserCookie,
		Path:     "/",
		SameSite: http.SameSiteLaxMode,
		Secure:   strings.HasPrefix(h.config.IssuerURL, "https://"),
		Value:    request.BrowserID,
	})
	requestToken := randomToken()
	h.mu.Lock()
	h.requests[requestToken] = request
	h.mu.Unlock()
	http.Redirect(w, r, "/?"+url.Values{"request": {requestToken}}.Encode(), http.StatusFound)
}

func (h *OIDCHandler) redirect(w http.ResponseWriter, r *http.Request,
	request authorization, key, value string) {
	target, _ := url.Parse(request.RedirectURI)
	query := target.Query()
	query.Set(key, value)
	if request.State != "" {
		query.Set("state", request.State)
	}
	target.RawQuery = query.Encode()
	http.Redirect(w, r, target.String(), http.StatusSeeOther)
}

func (h *OIDCHandler) browserRequest(token, browserID string) (authorization, bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	request, ok := h.requests[token]
	return request, ok && h.now().Before(request.ExpiresAt) && browserID == request.BrowserID
}

func (h *OIDCHandler) LoginContext(token, browserID string) (string, bool) {
	request, ok := h.browserRequest(token, browserID)
	if !ok {
		return "", false
	}
	redirect, _ := url.Parse(request.RedirectURI)
	return redirect.Host, true
}

func (h *OIDCHandler) finish(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 8192)
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid form.", http.StatusBadRequest)
		return
	}
	for _, values := range r.PostForm {
		if len(values) != 1 {
			http.Error(w, "Repeated form fields are unsupported.", http.StatusBadRequest)
			return
		}
	}
	requestToken := r.PostForm.Get("request")
	browserID := ""
	if cookie, err := r.Cookie(browserCookie); err == nil {
		browserID = cookie.Value
	}
	request, ok := h.browserRequest(requestToken, browserID)
	if !ok {
		http.Error(w, "This sign-in has expired. Start again from your app.", http.StatusBadRequest)
		return
	}
	var user User
	if r.PostForm.Get("action") != "cancel" {
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()
		users, err := h.users.ListUsers(ctx)
		if err != nil {
			slog.Error("list users", "error", err)
			http.Error(w, "Accounts are unavailable. Try again.", http.StatusServiceUnavailable)
			return
		}
		switch r.PostForm.Get("action") {
		case "select":
			for _, candidate := range users {
				if candidate.Subject == r.PostForm.Get("subject") {
					user = candidate
					break
				}
			}
			if user.Subject == "" {
				http.Error(w, "Account no longer exists. Start again.", http.StatusBadRequest)
				return
			}
		case "create":
			email := strings.ToLower(strings.TrimSpace(r.PostForm.Get("email")))
			name := strings.TrimSpace(r.PostForm.Get("name"))
			address, err := mail.ParseAddress(email)
			if err != nil || address.Address != email || len(email) > 254 ||
				name == "" || len(name) > 200 || strings.ContainsAny(name, "\r\n") {
				http.Error(w, "Enter a name and a valid email address.", http.StatusBadRequest)
				return
			}
			// This external identifier is stable across retries and provider restarts.
			// Only the relying app persists the new account through its callback.
			digest := sha256.Sum256([]byte(email))
			user = User{
				Email:   email,
				Name:    name,
				Subject: "dev-" + hex.EncodeToString(digest[:]),
			}
			for _, candidate := range users {
				if strings.EqualFold(candidate.Email, email) || candidate.Subject == user.Subject {
					user = candidate
					break
				}
			}
		default:
			http.Error(w, "Invalid sign-in action.", http.StatusBadRequest)
			return
		}
	}
	h.mu.Lock()
	// Concurrent form submissions can only consume the request once.
	_, ok = h.requests[requestToken]
	if ok && h.now().Before(request.ExpiresAt) {
		delete(h.requests, requestToken)
	} else {
		h.mu.Unlock()
		http.Error(w, "This sign-in is already used or expired.", http.StatusBadRequest)
		return
	}
	if r.PostForm.Get("action") == "cancel" {
		h.mu.Unlock()
		h.redirect(w, r, request, "error", "access_denied")
		return
	}
	code := randomToken()
	h.codes[code] = grant{
		Authorization: request,
		ExpiresAt:     h.now().Add(time.Minute),
		User:          user,
	}
	h.mu.Unlock()
	h.redirect(w, r, request, "code", code)
}
