package oidc

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
)

func (h *OIDCHandler) token(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 8192)
	if err := r.ParseForm(); err != nil {
		oauthError(w, http.StatusBadRequest, "invalid_request", "Invalid token request.")
		return
	}
	for _, values := range r.PostForm {
		if len(values) != 1 {
			oauthError(w, http.StatusBadRequest, "invalid_request", "Repeated parameters are unsupported.")
			return
		}
	}
	clientID, clientSecret := r.PostForm.Get("client_id"), r.PostForm.Get("client_secret")
	if basicID, basicSecret, ok := r.BasicAuth(); ok {
		if clientID != "" || clientSecret != "" {
			oauthError(w, http.StatusBadRequest, "invalid_request", "Use one client authentication method.")
			return
		}
		var idErr, secretErr error
		clientID, idErr = url.QueryUnescape(basicID)
		clientSecret, secretErr = url.QueryUnescape(basicSecret)
		if idErr != nil || secretErr != nil {
			oauthError(w, http.StatusBadRequest, "invalid_request", "Invalid client credentials encoding.")
			return
		}
	}
	if clientID != h.config.ClientID ||
		subtle.ConstantTimeCompare([]byte(clientSecret), []byte(h.config.ClientSecret)) != 1 {
		w.Header().Set("WWW-Authenticate", `Basic realm="token"`)
		oauthError(w, http.StatusUnauthorized, "invalid_client", "Client authentication failed.")
		return
	}
	if r.PostForm.Get("grant_type") != "authorization_code" {
		oauthError(w, http.StatusBadRequest, "unsupported_grant_type", "Only authorization_code is supported.")
		return
	}
	h.mu.Lock()
	code := r.PostForm.Get("code")
	grant, ok := h.codes[code]
	valid := ok && h.now().Before(grant.ExpiresAt) &&
		grant.Authorization.RedirectURI == r.PostForm.Get("redirect_uri")
	if grant.Authorization.Challenge != "" {
		verifier := r.PostForm.Get("code_verifier")
		digest := sha256.Sum256([]byte(verifier))
		challenge := base64.RawURLEncoding.EncodeToString(digest[:])
		valid = valid && len(verifier) >= 43 && len(verifier) <= 128 &&
			strings.IndexFunc(verifier, func(char rune) bool {
				return !(char >= 'a' && char <= 'z' || char >= 'A' && char <= 'Z' ||
					char >= '0' && char <= '9' || strings.ContainsRune("-._~", char))
			}) == -1 && subtle.ConstantTimeCompare([]byte(challenge),
			[]byte(grant.Authorization.Challenge)) == 1
	}
	if !valid {
		h.mu.Unlock()
		oauthError(w, http.StatusBadRequest, "invalid_grant", "Code is invalid, expired, or already used.")
		return
	}
	delete(h.codes, code)
	h.mu.Unlock()
	now := h.now()
	claims := h.userClaims(grant.User, grant.Authorization.Scope)
	claims["aud"] = h.config.ClientID
	claims["exp"] = now.Add(tokenLifetime).Unix()
	claims["iat"] = now.Unix()
	claims["iss"] = h.config.IssuerURL
	if grant.Authorization.Nonce != "" {
		claims["nonce"] = grant.Authorization.Nonce
	}
	accessToken := randomToken()
	accessHash := sha256.Sum256([]byte(accessToken))
	claims["at_hash"] = base64.RawURLEncoding.EncodeToString(accessHash[:16])
	idToken, err := h.sign(claims)
	if err != nil {
		oauthError(w, http.StatusInternalServerError, "server_error", "Could not sign the ID token.")
		return
	}
	h.mu.Lock()
	h.accessTokens[accessToken] = accessGrant{
		ExpiresAt: now.Add(tokenLifetime),
		Scope:     grant.Authorization.Scope,
		User:      grant.User,
	}
	h.mu.Unlock()
	writeJSON(w, http.StatusOK, map[string]any{
		"access_token": accessToken,
		"expires_in":   int(tokenLifetime.Seconds()),
		"id_token":     idToken,
		"scope":        grant.Authorization.Scope,
		"token_type":   "Bearer",
	})
}

func (h *OIDCHandler) keyID() string {
	digest := sha256.Sum256(h.key.N.Bytes())
	return base64.RawURLEncoding.EncodeToString(digest[:12])
}

func (h *OIDCHandler) sign(claims map[string]any) (string, error) {
	header, err := json.Marshal(map[string]string{"alg": "RS256", "kid": h.keyID(), "typ": "JWT"})
	if err != nil {
		return "", err
	}
	payload, err := json.Marshal(claims)
	if err != nil {
		return "", err
	}
	unsigned := base64.RawURLEncoding.EncodeToString(header) + "." +
		base64.RawURLEncoding.EncodeToString(payload)
	digest := sha256.Sum256([]byte(unsigned))
	signature, err := rsa.SignPKCS1v15(rand.Reader, h.key, crypto.SHA256, digest[:])
	if err != nil {
		return "", err
	}
	return unsigned + "." + base64.RawURLEncoding.EncodeToString(signature), nil
}

func (h *OIDCHandler) userClaims(user User, scope string) map[string]any {
	claims := map[string]any{"sub": user.Subject}
	for _, value := range strings.Fields(scope) {
		switch value {
		case "email":
			claims["email"] = user.Email
			claims["email_verified"] = true
		case "profile":
			claims["name"] = user.Name
		}
	}
	return claims
}

func (h *OIDCHandler) userinfo(w http.ResponseWriter, r *http.Request) {
	scheme, token, ok := strings.Cut(r.Header.Get("Authorization"), " ")
	h.mu.Lock()
	grant, found := h.accessTokens[token]
	h.mu.Unlock()
	if !ok || !strings.EqualFold(scheme, "Bearer") || !found || !h.now().Before(grant.ExpiresAt) {
		w.Header().Set("WWW-Authenticate", `Bearer error="invalid_token"`)
		oauthError(w, http.StatusUnauthorized, "invalid_token", "A valid bearer token is required.")
		return
	}
	writeJSON(w, http.StatusOK, h.userClaims(grant.User, grant.Scope))
}
