package config

import (
	"errors"
	"fmt"
	"net/url"
)

type DB struct {
	Host     string `json:"host"`
	Port     int    `json:"port"`
	User     string `json:"user"`
	Password string `json:"password"`
	Database string `json:"database"`

	TLS bool `json:"tls"`
}

func (db *DB) URI() string {
	connect := url.URL{
		Scheme: "postgres",
		Host:   fmt.Sprintf("%s:%d", db.Host, db.Port),
	}

	if db.User != "" {
		connect.User = url.UserPassword(db.User, db.Password)
	}

	if db.Database != "" {
		connect.Path = db.Database
	}

	connect.RawQuery = "sslmode=disable"
	if db.TLS {
		connect.RawQuery = "sslmode=require"
	}

	return connect.String()
}

func (db *DB) Validate() error {
	if db.Host == "" {
		return errors.New("database host is required")
	}

	if db.Port == 0 {
		return errors.New("database port is required")
	}

	return nil
}
