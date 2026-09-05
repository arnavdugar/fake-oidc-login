package config

import (
	"errors"
	"net/url"
	"os"
	"strings"
)

type Config struct {
	ClientID       string
	ClientName     string
	ClientSecret   string
	DatabaseURL    string
	EmailColumn    string
	IssuerURL      string
	ListenAddress  string
	NameColumn     string
	RedirectURIs   []string
	Schema         string
	SubjectColumn  string
	Table          string
	UsersQueryFile string
}

func Load() (Config, error) {
	value := func(key, fallback string) string {
		if result := os.Getenv(key); result != "" {
			return result
		}
		return fallback
	}
	config := Config{
		ClientID:       value("CLIENT_ID", "fake-oidc-client"),
		ClientName:     value("CLIENT_NAME", "Local app"),
		ClientSecret:   value("CLIENT_SECRET", "fake-oidc-secret"),
		DatabaseURL:    os.Getenv("DATABASE_URL"),
		EmailColumn:    os.Getenv("DATABASE_EMAIL_COLUMN"),
		IssuerURL:      value("ISSUER_URL", "http://localhost:8080"),
		ListenAddress:  value("LISTEN_ADDRESS", ":8080"),
		NameColumn:     value("DATABASE_NAME_COLUMN", "name"),
		Schema:         value("DATABASE_SCHEMA", "public"),
		SubjectColumn:  value("DATABASE_SUBJECT_COLUMN", "id"),
		Table:          value("DATABASE_TABLE", "users"),
		UsersQueryFile: os.Getenv("DATABASE_USERS_QUERY_FILE"),
	}
	for _, redirect := range strings.Split(os.Getenv("REDIRECT_URIS"), ",") {
		if redirect = strings.TrimSpace(redirect); redirect != "" {
			config.RedirectURIs = append(config.RedirectURIs, redirect)
		}
	}
	if err := config.Validate(); err != nil {
		return Config{}, err
	}
	return config, nil
}

func (c Config) Validate() error {
	issuer, err := url.Parse(c.IssuerURL)
	if err != nil || issuer.Host == "" || issuer.User != nil || issuer.Path != "" ||
		issuer.RawQuery != "" || issuer.ForceQuery || strings.Contains(c.IssuerURL, "#") ||
		(issuer.Scheme != "http" && issuer.Scheme != "https") {
		return errors.New("ISSUER_URL must be an HTTP(S) origin without a trailing slash")
	}
	if c.ClientID == "" || c.ClientSecret == "" || c.ClientName == "" {
		return errors.New("client ID, name, and secret are required")
	}
	if c.DatabaseURL == "" && c.UsersQueryFile != "" {
		return errors.New("DATABASE_USERS_QUERY_FILE requires DATABASE_URL")
	}
	if len(c.RedirectURIs) == 0 {
		return errors.New("REDIRECT_URIS must contain at least one exact callback URL")
	}
	for _, value := range c.RedirectURIs {
		redirect, err := url.Parse(value)
		if err != nil || redirect.Host == "" || redirect.User != nil ||
			strings.Contains(value, "#") ||
			(redirect.Scheme != "http" && redirect.Scheme != "https") {
			return errors.New("REDIRECT_URIS must contain absolute HTTP(S) URLs without fragments")
		}
		for _, key := range []string{"code", "error", "error_description", "state"} {
			if redirect.Query().Has(key) {
				return errors.New("callback URLs must not contain OAuth response parameters")
			}
		}
	}
	return nil
}
