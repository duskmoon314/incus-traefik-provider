package traefik

import (
	"bytes"
	"fmt"
	"net"
	"strings"
	"text/template"
	"unicode"

	log "github.com/sirupsen/logrus"
	"github.com/traefik/traefik/v3/pkg/config/dynamic"
	"github.com/traefik/traefik/v3/pkg/config/label"

	"github.com/duskmoon314/incus-traefik-provider/internal/config"
	"github.com/duskmoon314/incus-traefik-provider/internal/incus"
)

// Build constructs a Traefik dynamic.Configuration from Incus instances.
// Labels are decoded using the same mechanism as Traefik's Docker provider,
// so all standard traefik.http.* label keys are supported.
func Build(instances []incus.Instance, cfg config.Config) *dynamic.Configuration {
	defaultRuleTpl, err := newDefaultRuleTemplate(cfg.Traefik.DefaultRule)
	if err != nil {
		log.WithError(err).Warn("failed to parse default rule template, routers without rules will be skipped")
	}

	configurations := make(map[string]*dynamic.Configuration)

	for _, inst := range instances {
		confFromLabel, err := label.DecodeConfiguration(inst.Labels)
		if err != nil {
			log.WithFields(log.Fields{
				"instance": inst.Name,
				"error":    err,
			}).Warn("failed to decode labels, skipping instance")
			continue
		}

		if confFromLabel.HTTP != nil {
			ensureDefaultService(confFromLabel.HTTP, inst.Name)
			fillServiceServers(confFromLabel.HTTP, inst.IP, inst.DefaultPort)
			buildRouters(confFromLabel.HTTP, inst.Name, inst.Labels, defaultRuleTpl)
		}

		configurations[inst.Name] = confFromLabel
	}

	return mergeConfigurations(configurations)
}

// newDefaultRuleTemplate parses the default rule template string.
func newDefaultRuleTemplate(tplStr string) (*template.Template, error) {
	return template.New("defaultRule").Funcs(template.FuncMap{
		"normalize": normalize,
	}).Parse(tplStr)
}

// normalize replaces all non-alphanumeric characters with hyphens,
// matching Traefik's Normalize function.
func normalize(name string) string {
	fargs := func(c rune) bool {
		return !unicode.IsLetter(c) && !unicode.IsNumber(c)
	}
	return strings.Join(strings.FieldsFunc(name, fargs), "-")
}

// ensureDefaultService creates a default service with the instance name if
// no services were defined via labels.
func ensureDefaultService(http *dynamic.HTTPConfiguration, instanceName string) {
	if len(http.Services) > 0 {
		return
	}
	svcName := normalize(instanceName)
	http.Services = make(map[string]*dynamic.Service)
	lb := &dynamic.ServersLoadBalancer{}
	lb.SetDefaults()
	http.Services[svcName] = &dynamic.Service{
		LoadBalancer: lb,
	}
}

// buildRouters ensures valid router configuration, mirroring the Docker provider's
// BuildRouterConfiguration: create a default router if none exist, apply the default
// rule to routers without a rule, and auto-link routers to the single service.
func buildRouters(http *dynamic.HTTPConfiguration, instanceName string, labels map[string]string, tpl *template.Template) {
	svcName := normalize(instanceName)

	// If no routers defined, create a default one
	if len(http.Routers) == 0 {
		if len(http.Services) > 1 {
			log.WithField("instance", instanceName).Warn("cannot create default router: multiple services defined")
			return
		}
		http.Routers = make(map[string]*dynamic.Router)
		http.Routers[svcName] = &dynamic.Router{}
	}

	for routerName, router := range http.Routers {
		// Apply default rule if none set
		if router.Rule == "" && tpl != nil {
			model := struct {
				Name   string
				Labels map[string]string
			}{
				Name:   normalize(instanceName),
				Labels: labels,
			}
			var buf bytes.Buffer
			if err := tpl.Execute(&buf, model); err != nil {
				log.WithFields(log.Fields{
					"instance": instanceName,
					"router":   routerName,
					"error":    err,
				}).Warn("failed to execute default rule template, removing router")
				delete(http.Routers, routerName)
				continue
			}
			rule := buf.String()
			if rule == "" {
				log.WithFields(log.Fields{
					"instance": instanceName,
					"router":   routerName,
				}).Warn("default rule template produced empty rule, removing router")
				delete(http.Routers, routerName)
				continue
			}
			router.Rule = rule
		}

		// Auto-link to service if not set
		if router.Service == "" {
			if len(http.Services) > 1 {
				log.WithFields(log.Fields{
					"instance": instanceName,
					"router":   routerName,
				}).Warn("cannot auto-link router to service: multiple services defined, removing router")
				delete(http.Routers, routerName)
				continue
			}
			for name := range http.Services {
				router.Service = name
			}
		}
	}
}

// fillServiceServers resolves loadbalancer server.port and server.scheme
// into a full server.URL for each HTTP service. Falls back to defaultPort
// (from proxy devices) when no port label is set.
func fillServiceServers(http *dynamic.HTTPConfiguration, ip, defaultPort string) {
	for _, svc := range http.Services {
		if svc.LoadBalancer == nil {
			continue
		}
		if len(svc.LoadBalancer.Servers) == 0 {
			svc.LoadBalancer.Servers = []dynamic.Server{{}}
		}
		for i, server := range svc.LoadBalancer.Servers {
			if server.URL != "" {
				continue
			}
			port := server.Port
			if port == "" {
				port = defaultPort
			}
			if port == "" {
				continue
			}
			scheme := server.Scheme
			if scheme == "" {
				scheme = "http"
			}
			svc.LoadBalancer.Servers[i].URL = fmt.Sprintf("%s://%s", scheme, net.JoinHostPort(ip, port))
			svc.LoadBalancer.Servers[i].Port = ""
			svc.LoadBalancer.Servers[i].Scheme = ""
		}
	}
}

// mergeConfigurations merges per-instance HTTP configurations into one.
func mergeConfigurations(configs map[string]*dynamic.Configuration) *dynamic.Configuration {
	merged := &dynamic.Configuration{
		HTTP: &dynamic.HTTPConfiguration{
			Routers:     make(map[string]*dynamic.Router),
			Services:    make(map[string]*dynamic.Service),
			Middlewares: make(map[string]*dynamic.Middleware),
		},
	}

	for _, conf := range configs {
		if conf.HTTP == nil {
			continue
		}
		for k, v := range conf.HTTP.Routers {
			merged.HTTP.Routers[k] = v
		}
		for k, v := range conf.HTTP.Services {
			merged.HTTP.Services[k] = v
		}
		for k, v := range conf.HTTP.Middlewares {
			merged.HTTP.Middlewares[k] = v
		}
	}

	return merged
}
