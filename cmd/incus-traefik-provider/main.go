package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	log "github.com/sirupsen/logrus"

	"github.com/duskmoon314/incus-traefik-provider/internal/config"
	"github.com/duskmoon314/incus-traefik-provider/internal/incus"
	"github.com/duskmoon314/incus-traefik-provider/internal/server"
)

func main() {
	configPath := flag.String("config", "", "path to config file (yaml, toml, or json)")
	showHelp := flag.Bool("help", false, "show help")
	flag.Parse()

	if *showHelp {
		fmt.Fprintf(os.Stderr, "incus-traefik-provider — Traefik dynamic config from Incus\n\n")
		fmt.Fprintf(os.Stderr, "Usage:\n")
		fmt.Fprintf(os.Stderr, "  incus-traefik-provider [--config path/to/config.yaml]\n\n")
		fmt.Fprintf(os.Stderr, "Flags:\n")
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nEnvironment variables (highest priority):\n")
		fmt.Fprintf(os.Stderr, "  ITP_INCUS_SOCKET                 Incus socket path\n")
		fmt.Fprintf(os.Stderr, "  ITP_INCUS_REMOTE_URL             Incus remote URL\n")
		fmt.Fprintf(os.Stderr, "  ITP_INCUS_REMOTE_CERT            Client TLS certificate\n")
		fmt.Fprintf(os.Stderr, "  ITP_INCUS_REMOTE_KEY             Client TLS key\n")
		fmt.Fprintf(os.Stderr, "  ITP_INCUS_REMOTE_CA              TLS CA certificate\n")
		fmt.Fprintf(os.Stderr, "  ITP_SERVER_LISTEN                Listen address (e.g. :9000)\n")
		fmt.Fprintf(os.Stderr, "  ITP_SERVER_POLL_INTERVAL         Poll interval (e.g. 10s)\n")
		fmt.Fprintf(os.Stderr, "  ITP_SERVER_PATH                  Config endpoint path\n")
		fmt.Fprintf(os.Stderr, "  ITP_TRAEFIK_EXPOSED_BY_DEFAULT   Expose all instances by default (true/false)\n")
		fmt.Fprintf(os.Stderr, "  ITP_TRAEFIK_DEFAULT_RULE         Default routing rule template\n")
		fmt.Fprintf(os.Stderr, "  ITP_TRAEFIK_NETWORK              Default NIC for IP resolution\n")
		os.Exit(0)
	}

	cfg, err := config.Load(*configPath)
	if err != nil {
		log.WithError(err).Fatal("failed to load config")
	}

	log.SetLevel(log.InfoLevel)
	log.WithFields(log.Fields{
		"socket": cfg.Incus.Socket,
		"listen": cfg.Server.Listen,
		"path":   cfg.Server.Path,
	}).Info("starting incus-traefik-provider")

	client, err := incus.NewClient(cfg)
	if err != nil {
		log.WithError(err).Fatal("failed to connect to Incus")
	}
	defer client.Disconnect()

	// Validate connection
	_, _, err = client.GetServer()
	if err != nil {
		log.WithError(err).Fatal("failed to ping Incus server")
	}
	log.Info("connected to Incus")

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	srv := server.New(cfg, client)
	if err := srv.Run(ctx); err != nil {
		log.WithError(err).Fatal("server error")
	}
}
