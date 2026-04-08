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

> The following example is under development and may be changed in the future.

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

## Configuration

