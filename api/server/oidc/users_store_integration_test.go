//go:build integration

package oidc

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"fake-oidc-login/internal/config"

	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUsersStoreReadsConfiguredSchemaAndRejectsWrites(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	require.NotEmpty(t, databaseURL, "TEST_DATABASE_URL is required with the integration tag")
	ctx := context.Background()
	admin, err := pgx.Connect(ctx, databaseURL)
	require.NoError(t, err)
	t.Cleanup(func() { _ = admin.Close(context.Background()) })
	schema := "test_" + strings.ToLower(randomToken()[:12])
	quoted := pgx.Identifier{schema}.Sanitize()
	_, err = admin.Exec(ctx, "CREATE SCHEMA "+quoted)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, err := admin.Exec(context.Background(), "DROP SCHEMA "+quoted+" CASCADE")
		assert.NoError(t, err)
	})
	_, err = admin.Exec(ctx, "CREATE TABLE "+quoted+`.people (
		subject text PRIMARY KEY, full_name text, email text);
		INSERT INTO `+quoted+`.people VALUES
		('google-subject', 'Alex Example', 'alex@example.test'),
		('without-email', 'No Email', NULL);`)
	require.NoError(t, err)
	c := config.Config{
		DatabaseURL:   databaseURL,
		EmailColumn:   "email",
		NameColumn:    "full_name",
		Schema:        schema,
		SubjectColumn: "subject",
		Table:         "people",
	}
	t.Run("column mapping and null email", func(t *testing.T) {
		store, err := NewUsersStore(ctx, c)
		require.NoError(t, err)
		t.Cleanup(store.Close)
		users, err := store.ListUsers(ctx)
		require.NoError(t, err)
		require.Len(t, users, 2)
		assert.Equal(t, User{
			Email:   "alex@example.test",
			Name:    "Alex Example",
			Subject: "google-subject",
		}, users[0])
		assert.Equal(t, "without-email", users[1].Subject)
		assert.True(t, strings.HasSuffix(users[1].Email, "@users.test"))
	})
	t.Run("custom query joins saved identities", func(t *testing.T) {
		_, err := admin.Exec(ctx, "CREATE TABLE "+quoted+`.identities (user_id text, identifier text);
			INSERT INTO `+quoted+`.identities VALUES ('google-subject', 'external-subject')`)
		require.NoError(t, err)
		queryFile := filepath.Join(t.TempDir(), "users.sql")
		query := "SELECT i.identifier, p.full_name, p.email FROM " + quoted + ".people p JOIN " +
			quoted + ".identities i ON i.user_id = p.subject"
		require.NoError(t, os.WriteFile(queryFile, []byte(query), 0600))
		queryConfig := c
		queryConfig.UsersQueryFile = queryFile
		store, err := NewUsersStore(ctx, queryConfig)
		require.NoError(t, err)
		t.Cleanup(store.Close)
		users, err := store.ListUsers(ctx)
		require.NoError(t, err)
		require.Len(t, users, 1)
		assert.Equal(t, "external-subject", users[0].Subject)
	})
	t.Run("display identifier preserves the subject", func(t *testing.T) {
		queryFile := filepath.Join(t.TempDir(), "labels.sql")
		query := "SELECT p.subject, p.full_name, p.email, i.identifier FROM " + quoted +
			".people p LEFT JOIN " + quoted + ".identities i ON i.user_id = p.subject ORDER BY p.subject"
		require.NoError(t, os.WriteFile(queryFile, []byte(query), 0600))
		queryConfig := c
		queryConfig.UsersQueryFile = queryFile
		store, err := NewUsersStore(ctx, queryConfig)
		require.NoError(t, err)
		t.Cleanup(store.Close)
		users, err := store.ListUsers(ctx)
		require.NoError(t, err)
		require.Len(t, users, 2)
		assert.Equal(t, "google-subject", users[0].Subject)
		assert.Equal(t, "external-subject", users[0].Identifier)
		assert.Equal(t, "alex@example.test", users[0].Email)
		assert.Empty(t, users[1].Identifier)
	})
	t.Run("write query is rejected even with a privileged connection", func(t *testing.T) {
		queryFile := filepath.Join(t.TempDir(), "write.sql")
		query := "DELETE FROM " + quoted + ".people RETURNING subject, full_name, email"
		require.NoError(t, os.WriteFile(queryFile, []byte(query), 0600))
		queryConfig := c
		queryConfig.UsersQueryFile = queryFile
		store, err := NewUsersStore(ctx, queryConfig)
		require.NoError(t, err)
		t.Cleanup(store.Close)
		_, err = store.ListUsers(ctx)
		require.Error(t, err)
		var pgErr interface{ SQLState() string }
		require.ErrorAs(t, err, &pgErr)
		assert.Equal(t, "25006", pgErr.SQLState())
		var count int
		require.NoError(t, admin.QueryRow(ctx, "SELECT count(*) FROM "+quoted+".people").Scan(&count))
		assert.Equal(t, 2, count)
	})
	t.Run("duplicate subjects reject an ambiguous source", func(t *testing.T) {
		queryFile := filepath.Join(t.TempDir(), "duplicates.sql")
		query := "SELECT subject, full_name, email FROM " + quoted + ".people UNION ALL " +
			"SELECT subject, full_name, email FROM " + quoted + ".people"
		require.NoError(t, os.WriteFile(queryFile, []byte(query), 0600))
		queryConfig := c
		queryConfig.UsersQueryFile = queryFile
		store, err := NewUsersStore(ctx, queryConfig)
		require.NoError(t, err)
		t.Cleanup(store.Close)
		_, err = store.ListUsers(ctx)
		assert.ErrorContains(t, err, "unique, nonempty subjects")
	})
	t.Run("multiple statements cannot switch off read-only mode", func(t *testing.T) {
		queryFile := filepath.Join(t.TempDir(), "multiple.sql")
		query := "SET TRANSACTION READ WRITE; DELETE FROM " + quoted +
			".people RETURNING subject, full_name, email"
		require.NoError(t, os.WriteFile(queryFile, []byte(query), 0600))
		queryConfig := c
		separator := "?"
		if strings.Contains(databaseURL, "?") {
			separator = "&"
		}
		queryConfig.DatabaseURL += separator + "default_query_exec_mode=simple_protocol"
		queryConfig.UsersQueryFile = queryFile
		store, err := NewUsersStore(ctx, queryConfig)
		require.NoError(t, err)
		t.Cleanup(store.Close)
		_, err = store.ListUsers(ctx)
		require.Error(t, err)
		var count int
		require.NoError(t, admin.QueryRow(ctx, "SELECT count(*) FROM "+quoted+".people").Scan(&count))
		assert.Equal(t, 2, count)
	})
}
