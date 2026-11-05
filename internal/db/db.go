// Package db builds connection strings and opens/tests Postgres connections
// using pgx. Passwords are never persisted by tusk; they are resolved from an
// explicit override (e.g. an interactive prompt), the PGPASSWORD environment
// variable, or ~/.pgpass — the last two handled natively by pgx.
package db

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/nofinsachin/tusk/internal/config"
)

// pingTimeout bounds how long a connection test may take.
const pingTimeout = 8 * time.Second

// DSN builds a libpq-style connection URL from a saved connection.
//
// If password is non-empty it is embedded in the DSN (used for the prompt
// flow). If it is empty, no password is set and pgx falls back to PGPASSWORD
// or ~/.pgpass at connect time.
func DSN(c config.Connection, password string) string {
	c = c.WithDefaults()

	u := url.URL{
		Scheme: "postgres",
		Host:   net.JoinHostPort(c.Host, strconv.Itoa(c.Port)),
		Path:   "/" + c.Database,
	}
	if c.User != "" {
		if password != "" {
			u.User = url.UserPassword(c.User, password)
		} else {
			u.User = url.User(c.User)
		}
	}
	q := url.Values{}
	if c.SSLMode != "" {
		q.Set("sslmode", c.SSLMode)
	}
	u.RawQuery = q.Encode()
	return u.String()
}

// Ping opens a connection described by c and verifies it with a ping, then
// closes it. It returns nil if the database is reachable and authentication
// succeeds. password may be empty to defer to PGPASSWORD/.pgpass.
func Ping(ctx context.Context, c config.Connection, password string) error {
	cfg, err := pgx.ParseConfig(DSN(c, password))
	if err != nil {
		return fmt.Errorf("invalid connection settings: %w", err)
	}

	ctx, cancel := context.WithTimeout(ctx, pingTimeout)
	defer cancel()

	conn, err := pgx.ConnectConfig(ctx, cfg)
	if err != nil {
		return fmt.Errorf("connect failed: %w", err)
	}
	defer conn.Close(ctx)

	if err := conn.Ping(ctx); err != nil {
		return fmt.Errorf("ping failed: %w", err)
	}
	return nil
}

// PasswordAvailable reports whether a password can be resolved for c without
// prompting — i.e. pgx finds one via PGPASSWORD or ~/.pgpass. It is used to
// decide whether the UI needs to show a password prompt.
//
// It never makes a network connection; it only parses configuration.
func PasswordAvailable(c config.Connection) bool {
	cfg, err := pgx.ParseConfig(DSN(c, ""))
	if err != nil {
		return false
	}
	return cfg.Password != ""
}

// Connect opens and returns a live connection. The caller owns it and must
// Close it. Used once we move beyond testing into real browsing/querying.
func Connect(ctx context.Context, c config.Connection, password string) (*pgx.Conn, error) {
	cfg, err := pgx.ParseConfig(DSN(c, password))
	if err != nil {
		return nil, fmt.Errorf("invalid connection settings: %w", err)
	}
	conn, err := pgx.ConnectConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("connect failed: %w", err)
	}
	return conn, nil
}
