package login

import (
	"context"
	"log/slog"
	"time"

	"fake-oidc-login/openapi"
	"fake-oidc-login/server/oidc"
)

type LoginHandler struct {
	clientName string
	provider   *oidc.OIDCHandler
	users      oidc.UserSource
}

func NewHandler(clientName string, provider *oidc.OIDCHandler, users oidc.UserSource) *LoginHandler {
	return &LoginHandler{
		clientName: clientName,
		provider:   provider,
		users:      users,
	}
}

func (h *LoginHandler) GetLogin(ctx context.Context, params openapi.GetLoginParams) (openapi.GetLoginRes, error) {
	response := &openapi.Login{
		ClientName: h.clientName,
		Users:      []openapi.User{},
	}
	if params.Request.Set && params.Request.Value != "" {
		response.RedirectHost, response.CanLogin = h.provider.LoginContext(
			params.Request.Value, params.FakeOidcBrowser.Value)
		if !response.CanLogin {
			return &openapi.GetLoginBadRequest{
				Code: openapi.ErrorCodeInvalidRequest,
			}, nil
		}
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	users, err := h.users.ListUsers(ctx)
	if err != nil {
		slog.Error("list users", "error", err)
		return &openapi.GetLoginServiceUnavailable{
			Code: openapi.ErrorCodeAccountsUnavailable,
		}, nil
	}
	for _, user := range users {
		account := openapi.User{
			Email:   user.Email,
			Name:    user.Name,
			Subject: user.Subject,
		}
		if user.Identifier != "" {
			account.Identifier.SetTo(user.Identifier)
		}
		response.Users = append(response.Users, account)
	}
	return response, nil
}
