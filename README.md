# Traefik Hub

<div align="center" style="margin: 30px;">
<a href="https://hub.traefik.io/">
  <img src="https://doc.traefik.io/traefik-hub/img/traefik-hub-logo.svg" style="width:250px;" align="center" />
</a>
<br />
<br />

<div align="center">
    <a href="https://hub.traefik.io">Log In</a> |
    <a href="https://doc.traefik.io/traefik-hub">Documentation</a>
</div>
</div>

<br />

<div align="center"><strong>Traefik Hub</strong>

<br />
<br />
</div>

<div align="center">Welcome to this repository!</div>

## :information_source: About

This repository contains source code showing how to use:

1. Traefik Hub API Gateway
2. Traefik Hub API Management


## :alembic: APIs used in this repository

All APIs are implemented using a simple JSON server in Go; the source code is [here](./src/api-server).

This JSON server is used to deploy simple JSON APIs using a configmap.

The Kubernetes manifests (YAML) to deploy those apps are [here](./src/manifests).

## :construction_worker: Where to start ?

The journey can start [here](WALKTHROUGH.md) for a quickstart with a global overview

## 📒 Repository Structure

```shell
.
├── api-gateway                       # Traefik Hub API Gateway tutorials
│   ├── 1-getting-started
│   ├── 2-secure-applications
├── api-management                    # Traefik Hub API Management tutorials
│   ├── 1-getting-started
│   ├── 2-access-control
│   ├── 3-api-lifecycle-management
│   └── 4-protect-api-infrastructure (WIP)
└── src
    ├── api-server                    # API server source code
    └── manifests                     # Yaml to deploy all apps
```
