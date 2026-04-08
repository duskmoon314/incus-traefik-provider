# incus-traefik-provider

A standalone HTTP provider for [Traefik](https://traefik.io/) that discovers [Incus](https://linuxcontainers.org/incus/) instances' properties (`user.traefik.*`) and generates Traefik configuration accordingly.

> *Note:* This repo is a vibe coding side project. Though I will try to review all agents' codes, no guarantee is made on the quality of the code. Use at your own risk.

## Prerequisites

- [Incus](https://linuxcontainers.org/incus/)
  - Latest version is recommended to ensure working with OCI images.
- [skopeo](https://github.com/containers/skopeo)
  - Incus relies on `skopeo` to work with OCI registries.
- [OpenTofu](https://opentofu.org/) **(Optional)**
  - I use OpenTofu to manage Incus, and currently I recommend using OpenTofu (or Terraform) to get a similar experience for docker users.

## Quick Start

### Using OpenTofu

First, create a `main.tf` file with the following content:

```hcl
terraform {
  required_providers {
    incus = {
      source  = "lxc/incus"
      version = "1.0.2"
    }
  }
}

provider "incus" {}

resource "incus_instance" "traefik" {
  image = "docker:traefik:v3.6"
  name  = "traefik"

  device {
    name = "http"
    type = "proxy"
    properties = {
      listen  = "tcp:0.0.0.0:80"
      connect = "tcp:127.0.0.1:80"
    }
  }

  device {
    name = "api"
    type = "proxy"
    properties = {
      listen  = "tcp:0.0.0.0:8080"
      connect = "tcp:127.0.0.1:8080"
    }
  }

  device {
    name = "config"
    type = "disk"
    properties = {
      source = abspath("${path.root}/traefik.yml")
      path   = "/etc/traefik/traefik.yml"
    }
  }
}

resource "incus_instance" "whoami" {
  image = "docker:traefik/whoami"
  name  = "whoami"

  config = {
    "user.traefik.enable"                                        = "true"
    "user.traefik.http.routers.whoami.rule"                      = "Host(`whoami.localhost`)"
    "user.traefik.http.routers.whoami.entrypoints"               = "web"
    "user.traefik.http.services.whoami.loadbalancer.server.port" = "80"
  }
}

resource "incus_instance" "itp" {
  image = "ghcr:duskmoon314/incus-traefik-provider:latest"
  name  = "itp"

  device {
    name = "incus-socket"
    type = "disk"
    properties = {
      source   = "/var/lib/incus/unix.socket"
      path     = "/var/lib/incus/unix.socket"
      readonly = "true"
      shift    = "true"
    }
  }
}
```

Then, create a `traefik.yml` file with the following content:

```yaml
api:
  insecure: true
entryPoints:
  web:
    address: ":80"
providers:
  http:
    endpoint: "http://itp:9000/config"
```

Finally, apply the configuration:

```bash
tofu apply
```

### Using Incus CLI

```bash
incus config set myapp \
  user.traefik.enable=true \
  "user.traefik.http.routers.myapp.rule=Host(\`app.example.com\`)" \
  user.traefik.http.routers.myapp.entrypoints=websecure \
  user.traefik.http.routers.myapp.tls=true \
  user.traefik.http.services.myapp.loadbalancer.server.port=8080
```

## Configuration

Config is loaded in this priority order (highest wins):

1. **Environment variables** — `ITP_` prefix
2. **Explicit file** — `--config path/to/file.yaml|toml|json`
3. **Auto-discovered file** — searches `./config.yaml`, `./config.toml`, `./config.json`, `/etc/incus-traefik-provider/config.*`
4. **Defaults** — sensible built-in values

### Config file (TOML example)

```toml
[incus]
socket = "/var/lib/incus/unix.socket"

[server]
listen = ":9000"
pollInterval = "10s"
path = "/config"

[traefik]
exposedByDefault = false
defaultRule = "Host(`{{ normalize .Name }}`)"
network = "eth0"
```

### TLS remote connection

```toml
[incus.remote]
url = "https://incus.example.com:8443"
cert = "/certs/client.crt"
key = "/certs/client.key"
ca = "/certs/ca.crt"          # omit to use system CAs

[server]
listen = ":9000"
```

### Environment variables

| Variable                         | Description                              |
| -------------------------------- | ---------------------------------------- |
| `ITP_INCUS_SOCKET`               | Incus Unix socket path                   |
| `ITP_INCUS_REMOTE_URL`           | Incus remote HTTPS URL                   |
| `ITP_INCUS_REMOTE_CERT`          | Client TLS certificate path              |
| `ITP_INCUS_REMOTE_KEY`           | Client TLS key path                      |
| `ITP_INCUS_REMOTE_CA`            | TLS CA certificate path                  |
| `ITP_SERVER_LISTEN`              | HTTP listen address (e.g. `:9000`)       |
| `ITP_SERVER_POLL_INTERVAL`       | Poll interval (e.g. `10s`)               |
| `ITP_SERVER_PATH`                | Config endpoint path (default `/config`) |
| `ITP_TRAEFIK_EXPOSED_BY_DEFAULT` | Expose all instances by default          |
| `ITP_TRAEFIK_DEFAULT_RULE`       | Default routing rule template            |
| `ITP_TRAEFIK_NETWORK`            | Default NIC for IP resolution            |

## Label Reference

Labels follow the same dot-notation as Traefik's Docker provider, prefixed with `user.traefik.`:

Labels are decoded using Traefik's built-in label parser, so all standard HTTP router, service, and middleware label keys are supported. See the [Traefik Docker routing documentation](https://doc.traefik.io/traefik/reference/install-configuration/providers/docker/) for the full list.

### Enable the instance

With `exposedByDefault = false` (default), instances must be explicitly enabled:

```
user.traefik.enable=true
```

With `exposedByDefault = true`, all instances are exposed unless explicitly disabled with `user.traefik.enable=false`.

### Routers

```
user.traefik.http.routers.<name>.rule=Host(`app.example.com`)
user.traefik.http.routers.<name>.entrypoints=websecure
user.traefik.http.routers.<name>.tls=true
user.traefik.http.routers.<name>.tls.certresolver=myresolver
user.traefik.http.routers.<name>.tls.options=default
user.traefik.http.routers.<name>.tls.domains[0].main=example.com
user.traefik.http.routers.<name>.tls.domains[0].sans=*.example.com
user.traefik.http.routers.<name>.middlewares=<mw1>,<mw2>
user.traefik.http.routers.<name>.service=<name>
user.traefik.http.routers.<name>.priority=10
```

### Services

If no `server.port` is specified, the provider auto-detects the port from the instance's proxy devices (lowest port wins).

```
user.traefik.http.services.<name>.loadbalancer.server.port=8080
user.traefik.http.services.<name>.loadbalancer.server.scheme=https
user.traefik.http.services.<name>.loadbalancer.server.url=http://10.0.0.5:8080
user.traefik.http.services.<name>.loadbalancer.passhostheader=true
user.traefik.http.services.<name>.loadbalancer.healthcheck.path=/health
user.traefik.http.services.<name>.loadbalancer.healthcheck.interval=10s
user.traefik.http.services.<name>.loadbalancer.sticky.cookie=true
user.traefik.http.services.<name>.loadbalancer.sticky.cookie.name=mysession
```

### Middlewares

All standard Traefik HTTP middlewares are supported via label decoding:

```
user.traefik.http.middlewares.<name>.headers.accesscontrolalloworigin=*
user.traefik.http.middlewares.<name>.headers.accesscontrolallowmethods=GET,POST
user.traefik.http.middlewares.<name>.headers.accesscontrolallowheaders=Content-Type
user.traefik.http.middlewares.<name>.stripprefix.prefixes=/api
user.traefik.http.middlewares.<name>.addprefix.prefix=/v1
user.traefik.http.middlewares.<name>.chain.middlewares=<mw1>,<mw2>
user.traefik.http.middlewares.<name>.ipallowlist.sourcerange=10.0.0.0/8
user.traefik.http.middlewares.<name>.ratelimit.average=100
user.traefik.http.middlewares.<name>.ratelimit.burst=200
user.traefik.http.middlewares.<name>.redirectscheme.scheme=https
user.traefik.http.middlewares.<name>.redirectscheme.permanent=true
user.traefik.http.middlewares.<name>.basicauth.users=user:password
user.traefik.http.middlewares.<name>.retry.attempts=3
user.traefik.http.middlewares.<name>.circuitbreaker.expression=NetworkErrorRatio() > 0.5
user.traefik.http.middlewares.<name>.compress=true
```

## Endpoints

| Path          | Description                                                 |
| ------------- | ----------------------------------------------------------- |
| `GET /config` | Returns current Traefik dynamic config as JSON              |
| `GET /health` | Returns `200 OK` if last refresh succeeded, `503` otherwise |

## Security

- **Socket binding** gives full Incus API access — treat the container as privileged.
  Prefer TLS remote with a dedicated read-only client certificate for production.
- The `/config` endpoint has no auth. Bind to localhost or an internal network.
- **Never expose the provider to the public internet.**
