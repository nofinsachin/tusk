package config

import (
	"testing"
)

func TestWithDefaults(t *testing.T) {
	c := Connection{Name: "x"}.WithDefaults()
	if c.Port != DefaultPort {
		t.Errorf("port = %d, want %d", c.Port, DefaultPort)
	}
	if c.SSLMode != DefaultSSLMode {
		t.Errorf("sslmode = %q, want %q", c.SSLMode, DefaultSSLMode)
	}

	// Explicit values are preserved.
	c2 := Connection{Name: "y", Port: 6000, SSLMode: "disable"}.WithDefaults()
	if c2.Port != 6000 || c2.SSLMode != "disable" {
		t.Errorf("WithDefaults overwrote explicit values: %+v", c2)
	}
}

func TestAddUpdateRemove(t *testing.T) {
	s := &Store{}

	if err := s.Add(Connection{Name: "a", Host: "h1"}); err != nil {
		t.Fatalf("Add a: %v", err)
	}
	if err := s.Add(Connection{Name: ""}); err == nil {
		t.Error("Add with empty name should fail")
	}
	if err := s.Add(Connection{Name: "a"}); err == nil {
		t.Error("Add duplicate name should fail")
	}

	// Update existing.
	if err := s.Update("a", Connection{Name: "a", Host: "h2"}); err != nil {
		t.Fatalf("Update a: %v", err)
	}
	if s.Connections[0].Host != "h2" {
		t.Errorf("host = %q, want h2", s.Connections[0].Host)
	}

	// Update missing.
	if err := s.Update("nope", Connection{Name: "nope"}); err == nil {
		t.Error("Update missing should fail")
	}

	// Rename collision.
	_ = s.Add(Connection{Name: "b"})
	if err := s.Update("b", Connection{Name: "a"}); err == nil {
		t.Error("renaming b -> a should collide")
	}

	// Remove.
	s.Remove("a")
	if s.indexByName("a") >= 0 {
		t.Error("a should be removed")
	}
	s.Remove("does-not-exist") // no panic
}

func TestSaveLoadRoundTrip(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)

	s := &Store{}
	if err := s.Add(Connection{
		Name: "local", Host: "localhost", Port: 5432,
		Database: "postgres", User: "postgres", SSLMode: "prefer",
	}); err != nil {
		t.Fatalf("Add: %v", err)
	}
	if err := s.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}

	got, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(got.Connections) != 1 {
		t.Fatalf("got %d connections, want 1", len(got.Connections))
	}
	c := got.Connections[0]
	if c.Name != "local" || c.Host != "localhost" || c.Port != 5432 ||
		c.Database != "postgres" || c.User != "postgres" || c.SSLMode != "prefer" {
		t.Errorf("round-trip mismatch: %+v", c)
	}
}

func TestLoadMissingFileIsEmpty(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)

	s, err := Load()
	if err != nil {
		t.Fatalf("Load on missing file: %v", err)
	}
	if len(s.Connections) != 0 {
		t.Errorf("expected empty store, got %d", len(s.Connections))
	}
}
