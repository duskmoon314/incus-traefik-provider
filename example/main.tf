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
