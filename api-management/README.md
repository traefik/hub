# Traefik Hub API Management

<div align="center" style="margin: 30px;">
<a href="https://hub.traefik.io/">
  <img src="https://doc.traefik.io/traefik-hub/img/traefik-hub-logo.svg" style="width:250px;" align="center" />
</a>
<br />
<br />

<div align="center">
    <a href="https://hub.traefik.io">Log In</a> |
    <a href="https://doc.traefik.io/traefik-hub/">Documentation</a>
</div>
</div>

<br />

<div align="center"><strong>Traefik Hub</strong>

<br />
<br />
</div>

<div align="center">API Management</div>

## ⬇️ Install

### Prerequisites

1. [kubectl](https://kubernetes.io/docs/tasks/tools/) command-line tool installed and configured to access the cluster
2. [gh](https://cli.github.com/) command-line tool installed and configured with your account
3. [Flux CD](https://fluxcd.io/flux/cmd/) command-line tool installed.
4. A Kubernetes cluster running.

All tutorials are tested using [k3d](https://k3d.io/).

To test Traefik Hub with a local Kubernetes, see [this tutorial](./tutorials/0-prerequisites/README.md).

## 💫 APIs used in this repository

APIs are implemented using a simple JSON server in go, source code is [here](./src/api-server).

This JSON server is used to deploy those APIs on http://api.docker.localhost/ :

- **Public** weather API for current weather:
   - `/weather/public?city={x}`
- **Paid** weather API with forecast:
   - `/weather/licensed/forecast?city={x}&dt={ts}`
- **Paid** weather API v2, multi lingual support:
   - `/weather/licensed/v2/forecast?city={x}&dt={ts}&lang={lang}`
- **Admin** API, reserved for internal usage:
   - `/admin`
- **External** API:
   - [https://openskynetwork.github.io](https://openskynetwork.github.io/opensky-api/rest.html)
      - Example: https://opensky-network.org/api/states/all?lamin=45.8389&lomin=5.9962&lamax=47.8229&lomax=10.5226
      - API Portal: https://openskynetwork.github.io/opensky-api/rest.html

The Kubernetes manifests (YAML) to deploy those apps are [here](./src/manifests).

## 📒 Directory Structure

```shell
.
├── demo
│   ├── gitops                        # GitOps demo, with FluxCD specifics
│   └── observability                 # Observability demo, used in GitOps demo
├── src
│   ├── api-server                    # API server source code
│   └── manifests                     # Yaml to deploy all apps
└── tutorials
    ├── 0-prerequisites
    ├── 1-getting-started
    ├── 2-advanced-use-cases
    ├── 3-access-control
    ├── 4-version-management
    ├── 5-users-documentation
    ├── 6-protect-infrastructure
    └── 7-management-at-scale
```
