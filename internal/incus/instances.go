package incus

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	incusclient "github.com/lxc/incus/client"
	"github.com/lxc/incus/shared/api"
	log "github.com/sirupsen/logrus"

	"github.com/duskmoon314/incus-traefik-provider/internal/config"
)

// Instance represents a running Incus instance with its Traefik labels and resolved IP.
type Instance struct {
	Name        string
	Labels      map[string]string // key=value pairs with "user." prefix stripped
	IP          string            // first routable IPv4 on the configured network
	DefaultPort string            // lowest port from proxy devices (empty if none)
}

// GetInstances fetches all instances, filters based on traefik.enable and
// exposedByDefault setting, strips the "user." prefix from labels,
// resolves each instance's IP, and detects default ports from proxy devices.
func GetInstances(client incusclient.InstanceServer, cfg config.Config) ([]Instance, error) {
	instances, err := client.GetInstances(api.InstanceTypeAny)
	if err != nil {
		return nil, fmt.Errorf("list instances: %w", err)
	}

	var result []Instance
	for _, inst := range instances {
		labels := extractLabels(inst.ExpandedConfig)

		if !isInstanceEnabled(labels, cfg.Traefik.ExposedByDefault) {
			continue
		}

		ip, err := getInstanceIP(client, inst.Name, cfg.Traefik.Network)
		if err != nil {
			log.WithFields(log.Fields{
				"instance": inst.Name,
				"error":    err,
			}).Warn("failed to resolve IP, skipping instance")
			continue
		}

		result = append(result, Instance{
			Name:        inst.Name,
			Labels:      labels,
			IP:          ip,
			DefaultPort: getDefaultPort(inst.ExpandedDevices),
		})
	}

	return result, nil
}

// isInstanceEnabled checks the traefik.enable label against the exposedByDefault setting.
func isInstanceEnabled(labels map[string]string, exposedByDefault bool) bool {
	enable, ok := labels["traefik.enable"]
	if !ok {
		return exposedByDefault
	}
	return strings.EqualFold(enable, "true")
}

// extractLabels filters config keys starting with "user." and strips the prefix.
func extractLabels(config map[string]string) map[string]string {
	labels := make(map[string]string)
	for k, v := range config {
		if strings.HasPrefix(k, "user.") {
			labels[strings.TrimPrefix(k, "user.")] = v
		}
	}
	return labels
}

// getDefaultPort inspects expanded devices for proxy type devices,
// parses their "connect" property to extract the port, and returns the lowest one.
func getDefaultPort(devices map[string]map[string]string) string {
	var ports []int

	for _, dev := range devices {
		if dev["type"] != "proxy" {
			continue
		}
		connect, ok := dev["connect"]
		if !ok {
			continue
		}
		port := parseProxyPort(connect)
		if port > 0 {
			ports = append(ports, port)
		}
	}

	if len(ports) == 0 {
		return ""
	}

	sort.Ints(ports)
	return strconv.Itoa(ports[0])
}

// parseProxyPort extracts the port number from an Incus proxy connect address.
// Format: "tcp:127.0.0.1:80" or "tcp:0.0.0.0:80" or "unix:/path"
func parseProxyPort(connect string) int {
	parts := strings.Split(connect, ":")
	if len(parts) < 2 {
		return 0
	}
	// The last part is the port for tcp addresses
	portStr := parts[len(parts)-1]
	port, err := strconv.Atoi(portStr)
	if err != nil {
		return 0
	}
	return port
}

// getInstanceIP fetches the instance state and returns the first routable IPv4
// on the specified network interface.
func getInstanceIP(client incusclient.InstanceServer, name, network string) (string, error) {
	state, _, err := client.GetInstanceState(name)
	if err != nil {
		return "", fmt.Errorf("get state: %w", err)
	}

	if state.Network == nil {
		return "", fmt.Errorf("no network interfaces on instance %q", name)
	}

	netInfo, ok := state.Network[network]
	if !ok {
		return "", fmt.Errorf("network %q not found on instance %q (available: %s)",
			network, name, availableNetworks(state.Network))
	}

	for _, addr := range netInfo.Addresses {
		if addr.Family == "inet" && addr.Scope == "global" {
			return addr.Address, nil
		}
	}

	// Fall back to any inet address if no global scope found
	for _, addr := range netInfo.Addresses {
		if addr.Family == "inet" {
			return addr.Address, nil
		}
	}

	return "", fmt.Errorf("no IPv4 address found on %q for instance %q", network, name)
}

func availableNetworks(nets map[string]api.InstanceStateNetwork) string {
	names := make([]string, 0, len(nets))
	for k := range nets {
		names = append(names, k)
	}
	return strings.Join(names, ", ")
}
