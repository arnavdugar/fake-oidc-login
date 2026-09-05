package oidc

import (
	"encoding/base64"
	"math/big"
	"net/http"
)

func (h *OIDCHandler) discovery(w http.ResponseWriter, r *http.Request) {
	issuer := h.config.IssuerURL
	writeJSON(w, http.StatusOK, map[string]any{
		"authorization_endpoint":                issuer + "/o/oauth2/v2/auth",
		"claims_supported":                      []string{"at_hash", "aud", "email", "email_verified", "exp", "iat", "iss", "name", "nonce", "sub"},
		"code_challenge_methods_supported":      []string{"S256"},
		"grant_types_supported":                 []string{"authorization_code"},
		"id_token_signing_alg_values_supported": []string{"RS256"},
		"issuer":                                issuer,
		"jwks_uri":                              issuer + "/oauth2/v3/certs",
		"response_modes_supported":              []string{"query"},
		"response_types_supported":              []string{"code"},
		"scopes_supported":                      []string{"openid", "email", "profile"},
		"subject_types_supported":               []string{"public"},
		"token_endpoint":                        issuer + "/token",
		"token_endpoint_auth_methods_supported": []string{"client_secret_basic", "client_secret_post"},
		"userinfo_endpoint":                     issuer + "/v1/userinfo",
	})
}

func (h *OIDCHandler) certs(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"keys": []map[string]string{{
			"alg": "RS256",
			"e":   base64.RawURLEncoding.EncodeToString(big.NewInt(int64(h.key.E)).Bytes()),
			"kid": h.keyID(),
			"kty": "RSA",
			"n":   base64.RawURLEncoding.EncodeToString(h.key.N.Bytes()),
			"use": "sig",
		}},
	})
}
