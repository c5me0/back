package config

import "os"

func ListenAddress() string {
	if address := os.Getenv("LISTEN_ADDRESS"); address != "" {
		return address
	}

	return ":80"
}

func MigrationsDir() string {
	if directory := os.Getenv("MIGRATIONS_DIR"); directory != "" {
		return directory
	}

	return "assets/migrations"
}
