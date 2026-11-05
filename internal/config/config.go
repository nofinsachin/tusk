// Package config handles loading and saving tusk's saved connections.
//
// Connections are stored as YAML at ~/.config/tusk/connections.yml. Passwords
// are never written to this file; they are resolved at connect time from
// PGPASSWORD, ~/.pgpass, or an interactive prompt.
package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// Connection is a single saved Postgres connection's metadata.
type Connection struct {
	Name    string `yaml:"name"`
	Host    string `yaml:"host"`
	Port    int    `yaml:"port"`
	Database string `yaml:"database"`
	User    string `yaml:"user"`
	SSLMode string `yaml:"sslmode"`
}

// Store is the on-disk collection of connections.
type Store struct {
	Connections []Connection `yaml:"connections"`
}

// DefaultPort is the standard Postgres port.
const DefaultPort = 5432

// DefaultSSLMode mirrors libpq's default.
const DefaultSSLMode = "prefer"

// configDir returns the directory tusk stores config in, honoring
// XDG_CONFIG_HOME when set.
func configDir() (string, error) {
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return filepath.Join(xdg, "tusk"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home dir: %w", err)
	}
	return filepath.Join(home, ".config", "tusk"), nil
}

// Path returns the full path to connections.yml.
func Path() (string, error) {
	dir, err := configDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "connections.yml"), nil
}

// Load reads the store from disk. A missing file is not an error; it yields an
// empty store so first-run works cleanly.
func Load() (*Store, error) {
	path, err := Path()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return &Store{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	var s Store
	if err := yaml.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	return &s, nil
}

// Save writes the store to disk, creating the config directory if needed.
// The file is written with 0600 permissions since it describes connections.
func (s *Store) Save() error {
	dir, err := configDir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create config dir %s: %w", dir, err)
	}
	path := filepath.Join(dir, "connections.yml")

	data, err := yaml.Marshal(s)
	if err != nil {
		return fmt.Errorf("encode connections: %w", err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}

// Add appends a connection. It returns an error if the name is empty or a
// connection with the same name already exists.
func (s *Store) Add(c Connection) error {
	if strings.TrimSpace(c.Name) == "" {
		return errors.New("connection name is required")
	}
	if s.indexByName(c.Name) >= 0 {
		return fmt.Errorf("a connection named %q already exists", c.Name)
	}
	s.Connections = append(s.Connections, c)
	return nil
}

// Update replaces the connection stored under origName with c. If the name is
// changed, the new name must not collide with another existing connection.
func (s *Store) Update(origName string, c Connection) error {
	if strings.TrimSpace(c.Name) == "" {
		return errors.New("connection name is required")
	}
	i := s.indexByName(origName)
	if i < 0 {
		return fmt.Errorf("no connection named %q", origName)
	}
	if c.Name != origName {
		if j := s.indexByName(c.Name); j >= 0 {
			return fmt.Errorf("a connection named %q already exists", c.Name)
		}
	}
	s.Connections[i] = c
	return nil
}

// Remove deletes the connection with the given name. It is not an error to
// remove a name that does not exist.
func (s *Store) Remove(name string) {
	i := s.indexByName(name)
	if i < 0 {
		return
	}
	s.Connections = append(s.Connections[:i], s.Connections[i+1:]...)
}

func (s *Store) indexByName(name string) int {
	for i := range s.Connections {
		if s.Connections[i].Name == name {
			return i
		}
	}
	return -1
}

// WithDefaults returns a copy of c with empty optional fields filled in.
func (c Connection) WithDefaults() Connection {
	if c.Port == 0 {
		c.Port = DefaultPort
	}
	if strings.TrimSpace(c.SSLMode) == "" {
		c.SSLMode = DefaultSSLMode
	}
	return c
}
