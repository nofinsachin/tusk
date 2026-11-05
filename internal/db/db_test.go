package db

import (
	"net/url"
	"strings"
	"testing"

	"github.com/nofinsachin/tusk/internal/config"
)

func TestDSNBasic(t *testing.T) {
	c := config.Connection{
		Name: "x", Host: "localhost", Port: 5432,
		Database: "mydb", User: "alice", SSLMode: "require",
	}
	dsn := DSN(c, "")

	u, err := url.Parse(dsn)
	if err != nil {
		t.Fatalf("parse DSN %q: %v", dsn, err)
	}
	if u.Scheme != "postgres" {
		t.Errorf("scheme = %q, want postgres", u.Scheme)
	}
	if u.Host != "localhost:5432" {
		t.Errorf("host = %q, want localhost:5432", u.Host)
	}
	if u.Path != "/mydb" {
		t.Errorf("path = %q, want /mydb", u.Path)
	}
	if u.User.Username() != "alice" {
		t.Errorf("user = %q, want alice", u.User.Username())
	}
	// No password provided -> none embedded.
	if _, set := u.User.Password(); set {
		t.Error("password should not be set when none provided")
	}
	if got := u.Query().Get("sslmode"); got != "require" {
		t.Errorf("sslmode = %q, want require", got)
	}
}

func TestDSNEmbedsPasswordWhenProvided(t *testing.T) {
	c := config.Connection{Host: "h", Port: 5432, Database: "d", User: "u"}
	dsn := DSN(c, "s3cr3t")
	u, err := url.Parse(dsn)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	pw, set := u.User.Password()
	if !set || pw != "s3cr3t" {
		t.Errorf("password = %q (set=%v), want s3cr3t", pw, set)
	}
}

func TestDSNSpecialCharsEscaped(t *testing.T) {
	c := config.Connection{Host: "h", Port: 5432, Database: "d", User: "a b"}
	dsn := DSN(c, "p@ss/word")
	u, err := url.Parse(dsn)
	if err != nil {
		t.Fatalf("parse %q: %v", dsn, err)
	}
	if u.User.Username() != "a b" {
		t.Errorf("username not decoded correctly: %q", u.User.Username())
	}
	pw, _ := u.User.Password()
	if pw != "p@ss/word" {
		t.Errorf("password not decoded correctly: %q", pw)
	}
}

func TestDSNAppliesDefaults(t *testing.T) {
	c := config.Connection{Host: "h", Database: "d", User: "u"} // no port, no sslmode
	dsn := DSN(c, "")
	if !strings.Contains(dsn, ":5432") {
		t.Errorf("expected default port in DSN: %q", dsn)
	}
	if !strings.Contains(dsn, "sslmode=prefer") {
		t.Errorf("expected default sslmode in DSN: %q", dsn)
	}
}

func TestPasswordAvailableFromEnv(t *testing.T) {
	// Isolate from any ~/.pgpass on the host.
	t.Setenv("PGPASSFILE", "/nonexistent/tusk-test-pgpass")
	c := config.Connection{Host: "h", Port: 5432, Database: "d", User: "u"}

	t.Setenv("PGPASSWORD", "")
	if PasswordAvailable(c) {
		t.Error("expected no password available with empty PGPASSWORD and no pgpass")
	}

	t.Setenv("PGPASSWORD", "fromenv")
	if !PasswordAvailable(c) {
		t.Error("expected password available when PGPASSWORD is set")
	}
}
