package config

import "time"

// Config is the top-level configuration for incus-traefik-provider.
type Config struct {
	Incus   IncusConfig   `json:"incus"   yaml:"incus"   toml:"incus"`
	Server  ServerConfig  `json:"server"  yaml:"server"  toml:"server"`
	Traefik TraefikConfig `json:"traefik" yaml:"traefik" toml:"traefik"`
}

// IncusConfig holds connection parameters for the Incus daemon.
type IncusConfig struct {
	Socket string        `json:"socket" yaml:"socket" toml:"socket"`
	Remote *RemoteConfig `json:"remote" yaml:"remote" toml:"remote"`
}

// RemoteConfig holds TLS connection parameters for remote Incus daemons.
type RemoteConfig struct {
	URL  string `json:"url"  yaml:"url"  toml:"url"`
	Cert string `json:"cert" yaml:"cert" toml:"cert"`
	Key  string `json:"key"  yaml:"key"  toml:"key"`
	CA   string `json:"ca"   yaml:"ca"   toml:"ca"`
}

// ServerConfig controls the HTTP server that Traefik polls.
type ServerConfig struct {
	Listen       string        `json:"listen"        yaml:"listen"        toml:"listen"`
	PollInterval time.Duration `json:"pollInterval"  yaml:"pollInterval"  toml:"pollInterval"`
	Path         string        `json:"path"          yaml:"path"          toml:"path"`
	HealthPath   string        `json:"healthPath"    yaml:"healthPath"    toml:"healthPath"`
}

// TraefikConfig holds defaults applied when instance labels omit them.
type TraefikConfig struct {
	ExposedByDefault bool   `json:"exposedByDefault" yaml:"exposedByDefault" toml:"exposedByDefault"`
	DefaultRule      string `json:"defaultRule"      yaml:"defaultRule"      toml:"defaultRule"`
	Network          string `json:"network"          yaml:"network"          toml:"network"`
}

// Defaults returns a Config with sensible default values.
func Defaults() Config {
	return Config{
		Incus: IncusConfig{
			Socket: "/var/lib/incus/unix.socket",
		},
		Server: ServerConfig{
			Listen:       ":9000",
			PollInterval: 10 * time.Second,
			Path:         "/config",
			HealthPath:   "/health",
		},
		Traefik: TraefikConfig{
			ExposedByDefault: false,
			DefaultRule:      "Host(`{{ normalize .Name }}`)",
			Network:          "eth0",
		},
	}
}
