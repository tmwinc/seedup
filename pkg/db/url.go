package db

import (
	"fmt"
	"net/url"
	"strings"
)

// DBConfig holds parsed database connection details
type DBConfig struct {
	User     string
	Password string
	Host     string
	Port     string
	Database string
	SSLMode  string
}

// ParseDatabaseURL parses a PostgreSQL connection URL
func ParseDatabaseURL(rawURL string) (*DBConfig, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("invalid database URL: %w", err)
	}

	if u.Scheme != "postgres" && u.Scheme != "postgresql" {
		return nil, fmt.Errorf("invalid scheme: expected postgres or postgresql, got %s", u.Scheme)
	}

	password, _ := u.User.Password()

	port := u.Port()
	if port == "" {
		port = "5432"
	}

	database := strings.TrimPrefix(u.Path, "/")

	sslMode := u.Query().Get("sslmode")
	if sslMode == "" {
		sslMode = "disable"
	}

	return &DBConfig{
		User:     u.User.Username(),
		Password: password,
		Host:     u.Hostname(),
		Port:     port,
		Database: database,
		SSLMode:  sslMode,
	}, nil
}

// AdminURL returns a URL pointing to the 'postgres' database for admin operations
func (c *DBConfig) AdminURL() string {
	if c.Password != "" {
		return fmt.Sprintf("postgres://%s:%s@%s:%s/postgres?sslmode=%s",
			c.User, url.QueryEscape(c.Password), c.Host, c.Port, c.SSLMode)
	}
	return fmt.Sprintf("postgres://%s@%s:%s/postgres?sslmode=%s",
		c.User, c.Host, c.Port, c.SSLMode)
}

// URLWithDatabase returns a URL with a different database name
func (c *DBConfig) URLWithDatabase(dbName string) string {
	if c.Password != "" {
		return fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=%s",
			c.User, url.QueryEscape(c.Password), c.Host, c.Port, dbName, c.SSLMode)
	}
	return fmt.Sprintf("postgres://%s@%s:%s/%s?sslmode=%s",
		c.User, c.Host, c.Port, dbName, c.SSLMode)
}
