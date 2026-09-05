package oidc

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"strings"

	"fake-oidc-login/internal/config"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type User struct {
	Email      string `json:"email,omitempty"`
	Identifier string `json:"-"`
	Name       string `json:"name,omitempty"`
	Subject    string `json:"sub"`
}

type UserSource interface {
	ListUsers(context.Context) ([]User, error)
}

// UsersStore only reads an existing schema; the relying app owns account creation.
type UsersStore struct {
	pool  *pgxpool.Pool
	query string
}

func NewUsersStore(ctx context.Context, c config.Config) (*UsersStore, error) {
	if c.DatabaseURL == "" {
		return &UsersStore{}, nil
	}
	poolConfig, err := pgxpool.ParseConfig(c.DatabaseURL)
	if err != nil {
		return nil, errors.New("invalid DATABASE_URL")
	}
	poolConfig.MaxConns = 4
	// Keep custom queries to one prepared statement, including for URLs that request simple mode.
	poolConfig.ConnConfig.DefaultQueryExecMode = pgx.QueryExecModeCacheStatement
	poolConfig.ConnConfig.RuntimeParams["default_transaction_read_only"] = "on"
	poolConfig.ConnConfig.RuntimeParams["statement_timeout"] = "5000"
	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return nil, fmt.Errorf("create database pool: %w", err)
	}
	column := func(name string) string { return pgx.Identifier{name}.Sanitize() + "::text" }
	email := "NULL::text"
	if c.EmailColumn != "" {
		email = column(c.EmailColumn)
	}
	query := fmt.Sprintf("SELECT %s, %s, %s FROM %s ORDER BY 2, 1",
		column(c.SubjectColumn), column(c.NameColumn), email,
		pgx.Identifier{c.Schema, c.Table}.Sanitize())
	if c.UsersQueryFile != "" {
		contents, err := os.ReadFile(c.UsersQueryFile)
		if err != nil || strings.TrimSpace(string(contents)) == "" {
			pool.Close()
			return nil, errors.New("DATABASE_USERS_QUERY_FILE must be a readable, nonempty SQL file")
		}
		query = string(contents)
	}
	return &UsersStore{
		pool:  pool,
		query: query,
	}, nil
}

func (s *UsersStore) Close() {
	if s.pool != nil {
		s.pool.Close()
	}
}

func (s *UsersStore) ListUsers(ctx context.Context) ([]User, error) {
	users := []User{}
	if s.pool == nil {
		return users, nil
	}
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{AccessMode: pgx.ReadOnly})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	rows, err := tx.Query(ctx, s.query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	columns := len(rows.FieldDescriptions())
	if columns != 3 && columns != 4 {
		return nil, errors.New("user query must return subject, name, email, and an optional identifier")
	}
	seen := map[string]bool{}
	for rows.Next() {
		var subject, name, email, identifier *string
		destinations := []any{&subject, &name, &email}
		if columns == 4 {
			destinations = append(destinations, &identifier)
		}
		if err := rows.Scan(destinations...); err != nil {
			return nil, err
		}
		if subject == nil || *subject == "" || len(*subject) > 255 || seen[*subject] {
			return nil, errors.New("user query must return unique, nonempty subjects of at most 255 bytes")
		}
		seen[*subject] = true
		user := User{Subject: *subject}
		if identifier != nil {
			user.Identifier = *identifier
		}
		if name != nil {
			user.Name = *name
		}
		if email != nil {
			user.Email = *email
		}
		if user.Email == "" {
			digest := sha256.Sum256([]byte(user.Subject))
			user.Email = hex.EncodeToString(digest[:12]) + "@users.test"
		}
		if user.Name == "" {
			user.Name = user.Email
		}
		users = append(users, user)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return users, nil
}
