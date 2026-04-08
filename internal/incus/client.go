package incus

import (
	"fmt"

	incusclient "github.com/lxc/incus/client"

	"github.com/duskmoon314/incus-traefik-provider/internal/config"
)

// NewClient creates an Incus client from the given config.
// It connects via Unix socket if cfg.Incus.Socket is set,
// or via TLS if cfg.Incus.Remote is configured.
func NewClient(cfg config.Config) (incusclient.InstanceServer, error) {
	if cfg.Incus.Remote != nil && cfg.Incus.Remote.URL != "" {
		return connectRemote(cfg)
	}
	return connectSocket(cfg.Incus.Socket)
}

func connectSocket(path string) (incusclient.InstanceServer, error) {
	client, err := incusclient.ConnectIncusUnix(path, &incusclient.ConnectionArgs{
		UserAgent: "incus-traefik-provider",
	})
	if err != nil {
		return nil, fmt.Errorf("connect to Incus socket %s: %w", path, err)
	}
	return client, nil
}

func connectRemote(cfg config.Config) (incusclient.InstanceServer, error) {
	args := &incusclient.ConnectionArgs{
		UserAgent:     "incus-traefik-provider",
		TLSClientCert: cfg.Incus.Remote.Cert,
		TLSClientKey:  cfg.Incus.Remote.Key,
	}
	if cfg.Incus.Remote.CA != "" {
		args.TLSCA = cfg.Incus.Remote.CA
	}

	client, err := incusclient.ConnectIncus(cfg.Incus.Remote.URL, args)
	if err != nil {
		return nil, fmt.Errorf("connect to Incus remote %s: %w", cfg.Incus.Remote.URL, err)
	}
	return client, nil
}
