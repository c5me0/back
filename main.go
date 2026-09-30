package main

import (
	"context"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"cameo/internal/config"
	"cameo/server"
)

func main() {
	// Every timestamp the API serializes is UTC, regardless of the host time zone.
	time.Local = time.UTC //nolint:reassign // process-wide UTC is intended; set before anything reads the clock

	if len(os.Args) > 1 && os.Args[1] == "healthcheck" {
		os.Exit(runHealthcheck())
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if err := server.NewServer().Run(ctx); err != nil {
		panic(err)
	}
}

func healthcheckAddress(address string) (string, error) {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return "", err
	}

	if host == "" || host == "0.0.0.0" || host == "::" {
		host = "127.0.0.1"
	}

	return net.JoinHostPort(host, port), nil
}

func runHealthcheck() int {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	address, err := healthcheckAddress(config.ListenAddress())
	if err != nil {
		return 1
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://"+address+"/readyz", nil)
	if err != nil {
		return 1
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return 1
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return 1
	}
	return 0
}
