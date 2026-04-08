package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
	"gopkg.in/yaml.v3"
)

// LoadFile loads configuration from a file, auto-detecting the format by extension.
// Supported extensions: .yaml/.yml, .toml, .json.
func LoadFile(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("read config file: %w", err)
	}

	cfg := Defaults()
	ext := strings.ToLower(filepath.Ext(path))

	switch ext {
	case ".yaml", ".yml":
		if err := yaml.Unmarshal(data, &cfg); err != nil {
			return Config{}, fmt.Errorf("parse YAML config: %w", err)
		}
	case ".toml":
		if err := toml.Unmarshal(data, &cfg); err != nil {
			return Config{}, fmt.Errorf("parse TOML config: %w", err)
		}
	case ".json":
		if err := json.Unmarshal(data, &cfg); err != nil {
			return Config{}, fmt.Errorf("parse JSON config: %w", err)
		}
	default:
		return Config{}, fmt.Errorf("unsupported config format: %s", ext)
	}

	return cfg, nil
}

// AutoDiscover searches for a config file in the standard locations.
// Returns the loaded config, or defaults if none found.
func AutoDiscover() (Config, error) {
	candidates := []string{
		"config.yaml",
		"config.yml",
		"config.toml",
		"config.json",
	}

	for _, path := range candidates {
		if _, err := os.Stat(path); err == nil {
			return LoadFile(path)
		}
	}

	// System-wide fallback
	systemCandidates := []string{
		"/etc/incus-traefik-provider/config.yaml",
		"/etc/incus-traefik-provider/config.yml",
		"/etc/incus-traefik-provider/config.toml",
		"/etc/incus-traefik-provider/config.json",
	}

	for _, path := range systemCandidates {
		if _, err := os.Stat(path); err == nil {
			return LoadFile(path)
		}
	}

	return Defaults(), nil
}

// LoadEnvOverrides applies ITP_ prefixed environment variable overrides to the config.
// Environment variables:
//
//	ITP_INCUS_SOCKET, ITP_INCUS_REMOTE_URL, ITP_INCUS_REMOTE_CERT,
//	ITP_INCUS_REMOTE_KEY, ITP_INCUS_REMOTE_CA,
//	ITP_SERVER_LISTEN, ITP_SERVER_POLL_INTERVAL, ITP_SERVER_PATH,
//	ITP_TRAEFIK_EXPOSED_BY_DEFAULT, ITP_TRAEFIK_DEFAULT_RULE,
//	ITP_TRAEFIK_NETWORK
func LoadEnvOverrides(cfg Config) Config {
	if v := os.Getenv("ITP_INCUS_SOCKET"); v != "" {
		cfg.Incus.Socket = v
	}
	if v := os.Getenv("ITP_INCUS_REMOTE_URL"); v != "" {
		if cfg.Incus.Remote == nil {
			cfg.Incus.Remote = &RemoteConfig{}
		}
		cfg.Incus.Remote.URL = v
	}
	if v := os.Getenv("ITP_INCUS_REMOTE_CERT"); v != "" {
		if cfg.Incus.Remote == nil {
			cfg.Incus.Remote = &RemoteConfig{}
		}
		cfg.Incus.Remote.Cert = v
	}
	if v := os.Getenv("ITP_INCUS_REMOTE_KEY"); v != "" {
		if cfg.Incus.Remote == nil {
			cfg.Incus.Remote = &RemoteConfig{}
		}
		cfg.Incus.Remote.Key = v
	}
	if v := os.Getenv("ITP_INCUS_REMOTE_CA"); v != "" {
		if cfg.Incus.Remote == nil {
			cfg.Incus.Remote = &RemoteConfig{}
		}
		cfg.Incus.Remote.CA = v
	}
	if v := os.Getenv("ITP_SERVER_LISTEN"); v != "" {
		cfg.Server.Listen = v
	}
	if v := os.Getenv("ITP_SERVER_POLL_INTERVAL"); v != "" {
		if d, err := parseDuration(v); err == nil {
			cfg.Server.PollInterval = d
		}
	}
	if v := os.Getenv("ITP_SERVER_PATH"); v != "" {
		cfg.Server.Path = v
	}
	if v := os.Getenv("ITP_SERVER_HEALTH_PATH"); v != "" {
		cfg.Server.HealthPath = v
	}
	if v := os.Getenv("ITP_TRAEFIK_EXPOSED_BY_DEFAULT"); v != "" {
		cfg.Traefik.ExposedByDefault = strings.EqualFold(v, "true") || v == "1"
	}
	if v := os.Getenv("ITP_TRAEFIK_DEFAULT_RULE"); v != "" {
		cfg.Traefik.DefaultRule = v
	}
	if v := os.Getenv("ITP_TRAEFIK_NETWORK"); v != "" {
		cfg.Traefik.Network = v
	}

	return cfg
}

// Load is the main entry point for loading configuration.
// If configPath is non-empty, it loads that file directly.
// Otherwise it auto-discovers a config file.
// Environment overrides are always applied last (highest priority).
func Load(configPath string) (Config, error) {
	var cfg Config
	var err error

	if configPath != "" {
		cfg, err = LoadFile(configPath)
		if err != nil {
			return Config{}, err
		}
	} else {
		cfg, err = AutoDiscover()
		if err != nil {
			return Config{}, err
		}
	}

	cfg = LoadEnvOverrides(cfg)
	return cfg, nil
}

// parseDuration tries Go duration format first, then falls back to plain seconds.
func parseDuration(s string) (time.Duration, error) {
	if d, err := time.ParseDuration(s); err == nil {
		return d, nil
	}
	var secs int
	if _, err := fmt.Sscanf(s, "%d", &secs); err == nil {
		return time.Duration(secs) * time.Second, nil
	}
	return 0, fmt.Errorf("cannot parse duration: %s", s)
}
